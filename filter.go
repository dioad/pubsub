package pubsub

import "sync"

// FilterChan returns a channel that will receive messages from the input channel that pass a filter function.
func FilterChan[T any](s <-chan interface{}, f func(T) bool) <-chan T {
	ts := make(chan T, cap(s))
	go func() {
		defer close(ts)
		for msg := range s {
			if typedMsg, ok := msg.(T); ok && f(typedMsg) {
				ts <- typedMsg
			}
		}
	}()

	return ts
}

func CastChan[T any](s <-chan interface{}) <-chan T {
	return FilterChan(s, func(T) bool { return true })
}

// SubscribeWithFilter returns a channel that will receive messages published to the topic that have a particular type and pass a filter function.
func SubscribeWithFilter[T any](t Topic, f func(T) bool) <-chan T {
	s := t.Subscribe()

	return FilterChan(s, f)
}

// Subscribe returns a channel that will receive messages published to the topic that have a particular type.
func Subscribe[T any](t Topic) <-chan T {
	s := t.Subscribe()

	ts := make(chan T, cap(s))
	go func() {
		defer close(ts)
		for msg := range s {
			if typedMsg, ok := msg.(T); ok {
				ts <- typedMsg
			}
		}
	}()

	return ts
}

// Merge returns a channel that will receive messages from all input channels.
func Merge[B any](channels ...<-chan interface{}) <-chan B {
	var wg sync.WaitGroup

	neededCapacity := 0
	for _, c := range channels {
		neededCapacity += cap(c)
	}

	out := make(chan B, neededCapacity)

	output := func(c <-chan interface{}) {
		for n := range c {
			if o, ok := n.(B); ok {
				out <- o
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
