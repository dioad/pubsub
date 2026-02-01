package pubsub

import (
	"context"
	"sync"
	"time"
)

// subscribeWithFunc is a helper that subscribes and calls a function for each message.
// Used by both pubsubTopic and topicWithHistory to avoid duplication.
func subscribeWithFunc(subscribeFn func() <-chan any, f func(msg any)) <-chan any {
	ch := subscribeFn()
	go func() {
		for msg := range ch {
			f(msg)
		}
	}()
	return ch
}

// Topic is a simple, single topic, publish/subscribe interface.
// It provides methods to publish messages, subscribe to messages,
// and unsubscribe from the topic. Messages can be of any type.
//
// There are several implementations of the Topic interface:
//   - Basic Topic (NewTopic): A simple topic with no history.
//   - Topic with History (newTopicWithHistory): Maintains a history of messages
//     using a mutex-protected slice. Good for most use cases where history is needed.
//   - Topic with Lock-Free History (newTopicWithLockFreeHistory): Maintains history
//     using a lock-free ring buffer. Recommended for high-concurrency environments
//     to minimize contention on the publish path.
//
// Basic usage:
//
//	// Create a new Topic
//	topic := pubsub.NewTopic()
//
//	// Subscribe to the topic
//	ch := topic.Subscribe()
//
//	// Process messages in a goroutine
//	go func() {
//	    for msg := range ch {
//	        fmt.Printf("Received: %v\n", msg)
//	    }
//	}()
//
//	// Publish messages to the topic
//	topic.Publish("Hello, World!")
//
//	// With history:
//	topicWithHistory := pubsub.NewTopic(pubsub.WithHistory(10))
type Topic interface {
	// Publish sends one or more messages to all subscribers of the topic.
	// Uses a non-blocking send to prevent deadlocks if a subscriber is not reading.
	// Returns a PublishResult containing the number of successful deliveries and drops.
	Publish(msg ...any) PublishResult

	// Subscribe returns a channel that will receive messages published to the topic.
	// If the topic has history enabled, the channel will receive historical messages.
	Subscribe() <-chan any

	// SubscribeFunc subscribes to the topic and calls the provided function for each message.
	// Returns the subscription channel that can be used to unsubscribe later.
	// If the topic has history enabled, the function will be called for historical messages.
	SubscribeFunc(f func(msg any)) <-chan any

	// SubscribeWithBuffer returns a channel with a custom buffer size that will receive messages.
	// Larger buffer sizes can help prevent message loss when subscribers can't keep up.
	SubscribeWithBuffer(size int) <-chan any

	// Unsubscribe removes a channel from the list of subscribers and closes the channel.
	// After unsubscribing, the channel will be closed.
	Unsubscribe(ch <-chan any)

	// PublishReliable publishes a message to the topic using blocking sends with timeout.
	// This ensures messages are delivered even to unbuffered channels, but may block briefly.
	// Returns the number of subscribers that successfully received the message.
	PublishReliable(msg ...any) int

	// SubscribeUnbuffered returns an unbuffered channel that will receive messages.
	// Should be used with PublishReliable for guaranteed message delivery.
	SubscribeUnbuffered() <-chan any

	// Shutdown closes all subscriptions with context for timeout support.
	Shutdown(ctx context.Context)

	// Close shuts down the topic and closes all subscription channels.
	Close()
}

// PublishResult contains information about the result of a Publish operation.
type PublishResult struct {
	// Deliveries is the number of successful deliveries.
	Deliveries int
	// Drops is the number of messages dropped due to full buffers.
	Drops int
}

type pubsubTopic struct {
	mu            sync.RWMutex
	subscriptions []chan any
	observer      Observer
	name          string
}

// sendFunc is a function type for sending a message to a channel.
// Returns true if the message was sent successfully.
type sendFunc func(ch chan any, m any) bool

// publishWithSendFunc publishes messages using the provided send function.
func (t *pubsubTopic) publishWithSendFunc(msg []any, send sendFunc) (int, int) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	for _, m := range msg {
		t.observer.OnPublish(t.name, m)
	}

	successCount := 0
	dropCount := 0
	for _, m := range msg {
		for _, ch := range t.subscriptions {
			if send(ch, m) {
				successCount++
			} else {
				t.observer.OnDrop(t.name, m)
				dropCount++
			}
		}
	}
	return successCount, dropCount
}

// Publish publishes a message to the topic.
// Uses a non-blocking send to prevent deadlocks if a subscriber is not reading.
func (t *pubsubTopic) Publish(msg ...any) PublishResult {
	deliveries, drops := t.publishWithSendFunc(msg, func(ch chan any, m any) bool {
		select {
		case ch <- m:
			return true
		default:
			return false
		}
	})
	return PublishResult{Deliveries: deliveries, Drops: drops}
}

// PublishReliable publishes a message to the topic using blocking sends with timeout.
// This ensures messages are delivered even to unbuffered channels, but may block briefly.
// Returns the number of subscribers that successfully received the message.
func (t *pubsubTopic) PublishReliable(msg ...any) int {
	deliveries, _ := t.publishWithSendFunc(msg, func(ch chan any, m any) bool {
		select {
		case ch <- m:
			return true
		case <-time.After(100 * time.Millisecond):
			return false
		}
	})
	return deliveries
}

// SubscribeWithBuffer returns a channel that will receive messages published to the topic.
//
// The channel will have a buffer of `size` messages.
func (t *pubsubTopic) SubscribeWithBuffer(size int) <-chan any {
	return t.subscribeWithBuffer(size)
}

// subscribeWithBuffer returns a channel that will receive messages published to the topic.
// The channel has a buffer of the specified size to prevent blocking on message delivery.
// This is an internal method used by Subscribe and SubscribeWithBuffer.
func (t *pubsubTopic) subscribeWithBuffer(size int) chan any {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.observer.OnSubscribe(t.name)

	ch := make(chan any, size)
	t.subscriptions = append(t.subscriptions, ch)
	return ch
}

// SubscribeFunc subscribes to the topic and calls the provided function for each message.
// Returns the subscription channel that can be used to unsubscribe later.
// If the topic has history enabled, the function will be called for historical messages.
func (t *pubsubTopic) SubscribeFunc(f func(msg any)) <-chan any {
	return subscribeWithFunc(t.Subscribe, f)
}

// Subscribe returns a channel that will receive messages published to the topic.
// Uses a small buffer to prevent message loss with non-blocking sends.
func (t *pubsubTopic) Subscribe() <-chan any {
	return t.subscribeWithBuffer(1)
}

// SubscribeUnbuffered returns an unbuffered channel that will receive messages.
// Should be used with PublishReliable for guaranteed message delivery.
func (t *pubsubTopic) SubscribeUnbuffered() <-chan any {
	return t.subscribeWithBuffer(0)
}

// Unsubscribe removes a channel from the list of subscribers and closes the channel.
// After unsubscribing, the channel will be closed.
func (t *pubsubTopic) Unsubscribe(ch <-chan any) {
	t.mu.Lock()
	defer t.mu.Unlock()

	subs := t.subscriptions
	for i, sub := range subs {
		if ch == sub {
			t.observer.OnUnsubscribe(t.name)
			t.subscriptions = append(subs[:i], subs[i+1:]...)
			close(sub)
			break
		}
	}
}

// Shutdown closes all subscriptions with context for timeout support.
func (t *pubsubTopic) Shutdown(ctx context.Context) {
	t.mu.Lock()
	defer t.mu.Unlock()

	for _, sub := range t.subscriptions {
		select {
		case <-ctx.Done():
			return
		default:
			close(sub)
			t.observer.OnUnsubscribe(t.name)
		}
	}
	t.subscriptions = nil
}

// Close shuts down the topic and closes all subscription channels.
func (t *pubsubTopic) Close() {
	t.Shutdown(context.Background())
}

func newTopic(o Observer) *pubsubTopic {
	if o == nil {
		o = NoopObserver{}
	}
	return &pubsubTopic{
		subscriptions: make([]chan any, 0),
		observer:      o,
	}
}

// TopicOpt is a functional option for configuring a Topic instance.
type TopicOpt func(*topicConfig)

type topicConfig struct {
	historySize  int
	lockFreeSize int
	observer     Observer
}

// WithTopicObserver sets the observer for the Topic instance.
func WithTopicObserver(o Observer) TopicOpt {
	return func(cfg *topicConfig) {
		cfg.observer = o
	}
}

// WithHistory enables message history for the Topic.
func WithHistory(size int) TopicOpt {
	return func(cfg *topicConfig) {
		cfg.historySize = size
	}
}

// WithLockFreeHistoryOpt enables lock-free message history for the Topic.
func WithLockFreeHistoryOpt(size int) TopicOpt {
	return func(cfg *topicConfig) {
		cfg.lockFreeSize = size
	}
}

// NewTopic creates a new Topic instance that implements a simple
// publish/subscribe messaging system. It accepts variadic options
// to configure features like history and observers.
//
// Example:
//
//	topic := pubsub.NewTopic(pubsub.WithHistory(10))
func NewTopic(opts ...TopicOpt) Topic {
	cfg := &topicConfig{
		observer: NoopObserver{},
	}

	for _, opt := range opts {
		opt(cfg)
	}

	if cfg.lockFreeSize > 0 {
		return newTopicWithLockFreeHistory(cfg.observer, cfg.lockFreeSize)
	}

	if cfg.historySize > 0 {
		return newTopicWithHistory(cfg.observer, cfg.historySize)
	}

	return newTopic(cfg.observer)
}
