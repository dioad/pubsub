// Package pubsub implements a publish/subscribe pattern with type-safe channels.
// It provides filtering capabilities and channel merging functionality for building
// event-driven applications with strong type checking.
package pubsub

import (
	"reflect"
	"sync"
)

// FilterChan returns a channel that will receive messages from the input channel that pass a filter function.
func FilterChan[T any](s <-chan interface{}, f func(T) bool) <-chan T {
	ts := make(chan T, cap(s))
	go func() {
		defer close(ts)
		for msg := range s {
			if typedMsg, ok := msg.(T); ok && f(typedMsg) {
				select {
				case ts <- typedMsg:
				default:
				}
			}
		}
	}()

	return ts
}

// CastChan creates a new channel that converts messages from an interface{} channel to type T.
// It internally uses FilterChan with an always-true filter, effectively performing only type conversion.
func CastChan[T any](s <-chan interface{}) <-chan T {
	return FilterChan(s, func(T) bool { return true })
}

// SubscribeWithFilter returns a channel that will receive messages published to the topic that have a particular type and pass a filter function.
func SubscribeWithFilter[T any](t Topic, f func(T) bool) <-chan T {
	s := t.Subscribe()

	return FilterChan(s, f)
}

// Subscribe returns a channel that will receive messages published to the topic that have a particular type.
// It creates a new buffered channel with the same capacity as the source channel and
// automatically filters messages based on type T. Messages that cannot be type-asserted
// to T are silently dropped. Uses a non-blocking send to prevent deadlocks if the output channel's buffer is full.
func Subscribe[T any](t Topic) <-chan T {
	s := t.Subscribe()

	ts := make(chan T, cap(s))
	go func() {
		defer close(ts)
		for msg := range s {
			if typedMsg, ok := msg.(T); ok {
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

// Merge combines multiple input channels into a single output channel of type B.
// The output channel's buffer capacity is the sum of all input channel capacities.
// It spawns a goroutine for each input channel to handle message forwarding and
// uses a WaitGroup to properly close the output channel when all input channels are closed.
func MergeOriginal[B any](channels ...<-chan interface{}) <-chan B {
	var wg sync.WaitGroup

	neededCapacity := 0
	for _, c := range channels {
		neededCapacity += cap(c)
	}

	out := make(chan B, neededCapacity)

	output := func(c <-chan interface{}) {
		for n := range c {
			if o, ok := n.(B); ok {
				select {
				case out <- o:
					// Message sent successfully
				default:
					// Channel is full, skip this message
				}
			}
		}
		wg.Done()
	}

	wg.Add(len(channels))
	for _, c := range channels {
		go output(c)
	}

	go func() {
		wg.Wait()
		close(out)
	}()

	return out
}

// Merge combines multiple input channels into a single output channel of type B.
// It uses reflection to handle channels of any type, allowing for flexible merging.
// The output channel's buffer capacity is the sum of all input channel capacities, or the number of channels if they are unbuffered.
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
				out <- val
			}
		}
	}()
	return out
}
