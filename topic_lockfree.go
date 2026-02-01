package pubsub

// topicWithLockFreeHistory implements the Topic interface with a lock-free
// ring buffer for history storage. This eliminates mutex contention in the
// Publish hot path.
type topicWithLockFreeHistory struct {
	topic   *pubsubTopic
	history *ringBuffer
}

// newTopicWithLockFreeHistory creates a new Topic with lock-free history.
// This provides better performance under high concurrency compared to
// newTopicWithHistory which uses mutex-protected slice history.
func newTopicWithLockFreeHistory(o Observer, size int) Topic {
	return &topicWithLockFreeHistory{
		topic:   newTopic(o),
		history: newRingBuffer(size),
	}
}

// Publish publishes messages to the topic and stores them in the lock-free history.
// This method has no mutex contention in the history update path.
func (t *topicWithLockFreeHistory) Publish(msg ...any) int {
	n := t.topic.Publish(msg...)

	// Store in lock-free ring buffer - no mutex needed
	for _, m := range msg {
		t.history.Push(m)
	}

	return n
}

// PublishReliable publishes messages using blocking sends with timeout.
func (t *topicWithLockFreeHistory) PublishReliable(msg ...any) int {
	n := t.topic.PublishReliable(msg...)

	// Store in lock-free ring buffer - no mutex needed
	for _, m := range msg {
		t.history.Push(m)
	}

	return n
}

// Subscribe returns a channel that will receive messages.
// New subscribers receive historical messages first.
func (t *topicWithLockFreeHistory) Subscribe() <-chan any {
	return t.subscribeWithBuffer(t.history.Cap() * 2)
}

// SubscribeWithBuffer returns a channel with a custom buffer size.
func (t *topicWithLockFreeHistory) SubscribeWithBuffer(size int) <-chan any {
	return t.subscribeWithBuffer(size)
}

// subscribeWithBuffer creates a subscription and sends historical messages.
func (t *topicWithLockFreeHistory) subscribeWithBuffer(size int) chan any {
	ch := t.topic.subscribeWithBuffer(size)

	// Send historical messages (non-blocking)
	for _, msg := range t.history.GetAll() {
		select {
		case ch <- msg:
		default:
			// Channel full, skip historical message
		}
	}

	return ch
}

// SubscribeFunc subscribes and calls the function for each message.
func (t *topicWithLockFreeHistory) SubscribeFunc(f func(msg any)) <-chan any {
	ch := t.Subscribe()
	go func() {
		for msg := range ch {
			f(msg)
		}
	}()
	return ch
}

// SubscribeUnbuffered returns an unbuffered channel.
func (t *topicWithLockFreeHistory) SubscribeUnbuffered() <-chan any {
	return t.subscribeWithBuffer(0)
}

// Unsubscribe removes a subscription.
func (t *topicWithLockFreeHistory) Unsubscribe(ch <-chan any) {
	t.topic.Unsubscribe(ch)
}

// Close shuts down the topic and closes all subscription channels.
func (t *topicWithLockFreeHistory) Close() {
	t.topic.Close()
}

// WithLockFreeHistory is an option for NewPubSub/NewShardedPubSub that
// enables lock-free message history for all topics.
func WithLockFreeHistory(size int) Opt {
	return func(ps *pubSub) {
		ps.topicFunc = func() Topic { return NewTopic(WithLockFreeHistoryOpt(size), WithTopicObserver(ps.observer)) }
	}
}
