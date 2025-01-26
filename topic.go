package pubsub

import "sync"

// Topic is a simple, single topic, publish/subscribe interface.
type Topic interface {
	Publish(msg ...any)
	Subscribe() <-chan any
	SubscribeFunc(f func(msg any))
	SubscribeWithBuffer(size int) <-chan any
	Unsubscribe(ch <-chan any)
}

type topic struct {
	mu            sync.RWMutex
	subscriptions []chan any
}

// Publish publishes a message to the topic.
func (t *topic) Publish(msg ...any) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	for _, m := range msg {
		for _, ch := range t.subscriptions {
			ch <- m
		}
	}
}

// SubscribeWithBuffer returns a channel that will receive messages published to the topic.
//
// The channel will have a buffer of `size` messages.
func (t *topic) SubscribeWithBuffer(size int) <-chan any {
	return t.subscribeWithBuffer(size)
}

// subscribeWithBuffer returns a channel that will receive messages published to the topic.
func (t *topic) subscribeWithBuffer(size int) chan any {
	t.mu.Lock()
	defer t.mu.Unlock()

	ch := make(chan any, size)
	t.subscriptions = append(t.subscriptions, ch)
	return ch
}

// SubscribeFunc subscribes to a topic and calls the provided function for each received message.
func (t *topic) SubscribeFunc(f func(msg any)) {
	ch := t.Subscribe()
	go func() {
		for msg := range ch {
			f(msg)
		}
	}()
}

// Subscribe returns a channel that will receive messages published to the topic.
func (t *topic) Subscribe() <-chan any {
	return t.subscribeWithBuffer(1)
}

// Unsubscribe removes a channel from the list of subscribers.
func (t *topic) Unsubscribe(ch <-chan any) {
	t.mu.Lock()
	defer t.mu.Unlock()

	subs := t.subscriptions
	for i, sub := range subs {
		if ch == sub {
			t.subscriptions = append(subs[:i], subs[i+1:]...)
			close(sub)
			break
		}
	}
}

func newTopic() *topic {
	return &topic{
		subscriptions: make([]chan any, 0),
	}
}

// NewTopic
func NewTopic() Topic {
	return newTopic()
}

type topicWithHistory struct {
	topic       *topic
	mu          sync.RWMutex
	history     []any
	historySize int
}

// Publish publishes a message to the topic.
func (t *topicWithHistory) Publish(msg ...any) {
	t.topic.Publish(msg...)

	t.mu.Lock()
	defer t.mu.Unlock()

	msgLen := len(msg)
	t.history = append(t.history, msg...)
	if len(t.history) > t.historySize {
		t.history = t.history[msgLen:]
	}
}

// SubscribeWithBuffer returns a channel that will receive messages published to the topic.
//
// The channel will have a buffer of `size` messages.
func (t *topicWithHistory) SubscribeWithBuffer(size int) <-chan any {
	return t.subscribeWithBuffer(size)
}

// subscribeWithBuffer returns a channel that will receive messages published to the topic.
func (t *topicWithHistory) subscribeWithBuffer(size int) chan any {
	t.mu.Lock()
	defer t.mu.Unlock()

	ch := t.topic.subscribeWithBuffer(size)

	for _, msg := range t.history {
		ch <- msg
	}

	return ch
}

// SubscriberFunc subscribes to a topic and calls the provided function for each received message.
func (t *topicWithHistory) SubscribeFunc(f func(msg any)) {
	ch := t.Subscribe()
	go func() {
		for msg := range ch {
			f(msg)
		}
	}()
}

// Subscribe returns a channel that will receive messages published to the topic.
func (t *topicWithHistory) Subscribe() <-chan any {
	return t.subscribeWithBuffer(t.historySize * 2)
}

// Unsubscribe removes a channel from the list of subscribers.
func (t *topicWithHistory) Unsubscribe(ch <-chan any) {
	t.topic.Unsubscribe(ch)
}

// NewTopicWithHistory creates a new Topic instance with history.
// The history buffer will store the last `size` messages published to the topic.
func NewTopicWithHistory(size int) Topic {
	return &topicWithHistory{
		topic:       newTopic(),
		history:     make([]any, 0, size),
		historySize: size,
	}
}
