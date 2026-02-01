package pubsub

import (
	"reflect"
	"sync/atomic"
)

// DropMetrics tracks statistics about dropped messages in channel operations.
type DropMetrics struct {
	// DroppedCount is the total number of messages dropped due to full buffers.
	DroppedCount uint64
	// FilteredCount is the total number of messages filtered out (not matching criteria).
	FilteredCount uint64
}

// IncrementDropped atomically increments the dropped count.
func (m *DropMetrics) IncrementDropped() {
	atomic.AddUint64(&m.DroppedCount, 1)
}

// IncrementFiltered atomically increments the filtered count.
func (m *DropMetrics) IncrementFiltered() {
	atomic.AddUint64(&m.FilteredCount, 1)
}

// GetDropped returns the current dropped count.
func (m *DropMetrics) GetDropped() uint64 {
	return atomic.LoadUint64(&m.DroppedCount)
}

// GetFiltered returns the current filtered count.
func (m *DropMetrics) GetFiltered() uint64 {
	return atomic.LoadUint64(&m.FilteredCount)
}

// ApplyChan creates a new channel by applying a transformation function to each message
// from the input channel. If the transformation function returns an error,
// the message is dropped. The output channel has the same capacity as the source channel.
//
// Example:
//
//	rawCh := topic.Subscribe()
//	stringCh := ApplyChan(rawCh, func(msg any) (string, error) {
//	    if s, ok := msg.(string); ok {
//	        return strings.ToUpper(s), nil
//	    }
//	    return "", errors.New("not a string")
//	})
func ApplyChan[A any, B any](s <-chan A, f func(A) (B, error)) <-chan B {
	out, _ := ApplyChanWithMetrics(s, f, nil)
	return out
}

// ApplyChanWithMetrics is like ApplyChan but also tracks dropped message statistics.
// If metrics is non-nil, it will be updated with counts of filtered and dropped messages.
// Returns both the output channel and the metrics object for observation.
//
// Example:
//
//	metrics := &pubsub.DropMetrics{}
//	stringCh, _ := ApplyChanWithMetrics(rawCh, transformFunc, metrics)
//	// Later: check metrics.GetDropped() for buffer overflow count
func ApplyChanWithMetrics[A any, B any](s <-chan A, f func(A) (B, error), metrics *DropMetrics) (<-chan B, *DropMetrics) {
	if metrics == nil {
		metrics = &DropMetrics{}
	}
	out := make(chan B, cap(s))
	go func() {
		defer close(out)
		for msg := range s {
			b, err := f(msg)
			if err != nil {
				metrics.IncrementFiltered()
				continue
			}
			select {
			case out <- b:
				// Message sent successfully
			default:
				// Channel is full, message dropped
				metrics.IncrementDropped()
			}
		}
	}()
	return out, metrics
}

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
func FilterChan[T any](s <-chan any, f func(T) bool) <-chan T {
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

// CastChan creates a new channel that converts messages from an any channel to type T.
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
func CastChan[T any](s <-chan any) <-chan T {
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
func Merge[B any](channels ...<-chan any) <-chan B {
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
