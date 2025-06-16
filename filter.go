// Package pubsub implements a publish/subscribe pattern with type-safe channels.
// It provides filtering capabilities and channel merging functionality for building
// event-driven applications with strong type checking.
package pubsub

import (
	"reflect"
)

// FilterChan returns a channel that will receive messages from the input channel that pass a filter function.
// It performs both type assertion and custom filtering in a single operation.
// Messages that don't match the type T or don't pass the filter function are silently dropped.
//
// Example:
//
//	// Filter for only high-priority messages
//	rawCh := topic.Subscribe()
//	highPriorityMsgs := FilterChan(rawCh, func(msg AlertMessage) bool {
//	    return msg.Priority > 7
//	})
//	for alert := range highPriorityMsgs {
//	    handleHighPriorityAlert(alert)
//	}
func FilterChan[T any](s <-chan interface{}, f func(T) bool) <-chan T {
	ts := make(chan T, cap(s))
	go func() {
		defer close(ts)
		for msg := range s {
			if typedMsg, ok := msg.(T); ok && f(typedMsg) {
				select {
				case ts <- typedMsg:
					// Message sent successfully
				default:
					// Channel is full, skip this message
				}
			}
		}
	}()

	return ts
}

// CastChan creates a new channel that converts messages from an interface{} channel to type T.
// It internally uses FilterChan with an always-true filter, effectively performing only type conversion.
// Messages that cannot be type-asserted to T are silently dropped.
//
// Example:
//
//	// Convert a generic channel to a type-specific channel
//	rawCh := topic.Subscribe()
//	userCh := CastChan[User](rawCh)
//
//	// Process only User messages
//	for user := range userCh {
//	    fmt.Printf("User: %s (ID: %d)\n", user.Name, user.ID)
//	}
func CastChan[T any](s <-chan interface{}) <-chan T {
	return FilterChan(s, func(T) bool { return true })
}

// SubscribeWithFilter returns a channel that will receive messages published to the topic
// that have a particular type and pass a filter function. This combines type filtering with
// custom filtering logic in a single operation.
//
// Example:
//
//	// Subscribe to only receive successful login events
//	loginEvents := pubsub.SubscribeWithFilter(topic, func(event LoginEvent) bool {
//	    return event.Success == true
//	})
//	for event := range loginEvents {
//	    fmt.Printf("Successful login: %s at %v\n", event.Username, event.Timestamp)
//	}
func SubscribeWithFilter[T any](t Topic, f func(T) bool) <-chan T {
	s := t.Subscribe()
	return FilterChan(s, f)
}

// Subscribe returns a channel that will receive messages published to the topic that have a particular type.
// It creates a new buffered channel with the same capacity as the source channel and
// automatically filters messages based on type T. Messages that cannot be type-asserted
// to T are silently dropped. Uses a non-blocking send to prevent deadlocks if the output channel's buffer is full.
//
// Example:
//
//	// Subscribe to only receive UserEvent messages
//	userEvents := pubsub.Subscribe[UserEvent](topic)
//	for event := range userEvents {
//	    fmt.Printf("User %s performed action: %s\n", event.Username, event.Action)
//	}
func Subscribe[T any](t Topic) <-chan T {
	s := t.Subscribe()
	return CastChan[T](s)
}

// Merge combines multiple input channels into a single output channel of type B.
// It uses reflection to handle channels of any type, allowing for flexible merging.
// The output channel's buffer capacity is the sum of all input channel capacities, or the number of channels if they are unbuffered.
// When any of the input channels is closed, it is removed from the merge set, but the merged channel remains open until all input channels are closed.
//
// Example:
//
//	// Merge channels from different topics
//	ch1 := topic1.Subscribe()
//	ch2 := topic2.Subscribe()
//	mergedCh := Merge[UserEvent](ch1, ch2)
//
//	// Process events from both channels
//	for event := range mergedCh {
//	    fmt.Printf("User %s performed action: %s\n", event.Username, event.Action)
//	}
func Merge[B any](channels ...<-chan interface{}) <-chan B {
	capacity := 0
	cases := make([]reflect.SelectCase, len(channels))
	for i, ch := range channels {
		cases[i] = reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(ch),
		}
		if cap(ch) > 0 {
			capacity += cap(ch)
		} else {
			capacity += 1 // Unbuffered channels count as 1 for capacity
		}
	}

	out := make(chan B, capacity)
	go func() {
		defer close(out)

		for len(cases) > 0 {
			chosen, recv, ok := reflect.Select(cases)
			if !ok {
				// Remove closed channel
				cases = append(cases[:chosen], cases[chosen+1:]...)
				continue
			}
			if val, ok := recv.Interface().(B); ok {
				select {
				case out <- val:
					// Message sent successfully
				default:
					// Channel is full, skip this message
				}
			}
		}
	}()
	return out
}
