package pubsub

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
