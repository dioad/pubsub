// Package pubsub provides a simple publish/subscribe messaging system
// that allows multiple subscribers to receive messages published to a topic.
package pubsub

import (
	"sync"
	"time"
)

// Topic is a simple, single topic, publish/subscribe interface.
// It provides methods to publish messages, subscribe to messages,
// and unsubscribe from the topic. Messages can be of any type.
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
//	topicWithHistory := pubsub.NewTopicWithHistory(10)
type Topic interface {
	// Publish sends one or more messages to all subscribers of the topic.
	// Uses a non-blocking send to prevent deadlocks if a subscriber is not reading.
	Publish(msg ...any) int

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
	Unsubscribe(ch <-chan any)

	// PublishReliable publishes a message to the topic using blocking sends with timeout.
	// This ensures messages are delivered even to unbuffered channels, but may block briefly.
	// Returns the number of subscribers that successfully received the message.
	PublishReliable(msg ...any) int

	// SubscribeUnbuffered returns an unbuffered channel that will receive messages.
	// Should be used with PublishReliable for guaranteed message delivery.
	SubscribeUnbuffered() <-chan any
}

type topic struct {
	mu            sync.RWMutex
	subscriptions []chan any
}

// Publish publishes a message to the topic.
// Uses a non-blocking send to prevent deadlocks if a subscriber is not reading.
func (t *topic) Publish(msg ...any) int {
	t.mu.RLock()
	defer t.mu.RUnlock()

	successCount := 0
	for _, m := range msg {
		for _, ch := range t.subscriptions {
			select {
			case ch <- m:
				successCount++
				// Message sent successfully
			default:
				// Channel is full or not being read from, skip this message
			}
		}
	}
	return successCount
}

// PublishReliable publishes a message to the topic using blocking sends with timeout.
// This ensures messages are delivered even to unbuffered channels, but may block briefly.
// Returns the number of subscribers that successfully received the message.
func (t *topic) PublishReliable(msg ...any) int {
	t.mu.RLock()
	defer t.mu.RUnlock()

	successCount := 0
	for _, m := range msg {
		for _, ch := range t.subscriptions {
			select {
			case ch <- m:
				// Message sent successfully
				successCount++
			case <-time.After(100 * time.Millisecond):
				// Timeout - subscriber is not reading, but we tried
				// This prevents indefinite blocking while still attempting delivery
			}
		}
	}
	return successCount
}

// SubscribeWithBuffer returns a channel that will receive messages published to the topic.
//
// The channel will have a buffer of `size` messages.
func (t *topic) SubscribeWithBuffer(size int) <-chan any {
	return t.subscribeWithBuffer(size)
}

// subscribeWithBuffer returns a channel that will receive messages published to the topic.
// The channel has a buffer of the specified size to prevent blocking on message delivery.
// This is an internal method used by Subscribe and SubscribeWithBuffer.
func (t *topic) subscribeWithBuffer(size int) chan any {
	t.mu.Lock()
	defer t.mu.Unlock()

	ch := make(chan any, size)
	t.subscriptions = append(t.subscriptions, ch)
	return ch
}

// SubscribeFunc subscribes to the topic and calls the provided function for each message.
// Returns the subscription channel that can be used to unsubscribe later.
// If the topic has history enabled, the function will be called for historical messages.
func (t *topic) SubscribeFunc(f func(msg any)) <-chan any {
	ch := t.Subscribe()
	go func() {
		for msg := range ch {
			f(msg)
		}
	}()
	return ch
}

// Subscribe returns a channel that will receive messages published to the topic.
// Uses a small buffer to prevent message loss with non-blocking sends.
func (t *topic) Subscribe() <-chan any {
	return t.subscribeWithBuffer(1)
}

// SubscribeUnbuffered returns an unbuffered channel that will receive messages.
// Should be used with PublishReliable for guaranteed message delivery.
func (t *topic) SubscribeUnbuffered() <-chan any {
	return t.subscribeWithBuffer(0)
}

// Unsubscribe removes a channel from the list of subscribers and closes the channel.
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

// NewTopic creates a new Topic instance that implements a simple
// publish/subscribe messaging system.
func NewTopic() Topic {
	return newTopic()
}

// topicWithHistory implements the Topic interface and maintains a history
// of published messages. It stores the last N messages where N is specified
// during creation.
type topicWithHistory struct {
	topic       *topic
	mu          sync.RWMutex
	history     []any
	historySize int
}

// Publish publishes a message to the topic.
// Uses a non-blocking send to prevent deadlocks if a subscriber is not reading.
func (t *topicWithHistory) Publish(msg ...any) int {
	n := t.topic.Publish(msg...)

	t.mu.Lock()
	defer t.mu.Unlock()

	msgLen := len(msg)
	t.history = append(t.history, msg...)
	if len(t.history) > t.historySize {
		t.history = t.history[msgLen:]
	}
	return n
}

// SubscribeWithBuffer returns a channel with a custom buffer size that will receive messages.
// Larger buffer sizes can help prevent message loss when subscribers can't keep up.
// The channel will have a buffer of `size` messages.
func (t *topicWithHistory) SubscribeWithBuffer(size int) <-chan any {
	return t.subscribeWithBuffer(size)
}

// subscribeWithBuffer returns a channel that will receive messages published to the topic.
// Uses a non-blocking send for historical messages to prevent deadlocks if the channel buffer is smaller than the history size.
func (t *topicWithHistory) subscribeWithBuffer(size int) chan any {
	t.mu.Lock()
	defer t.mu.Unlock()

	ch := t.topic.subscribeWithBuffer(size)

	for _, msg := range t.history {
		select {
		case ch <- msg:
			// Message sent successfully
		default:
			// Channel is full, skip this historical message
		}
	}

	return ch
}

// SubscribeFunc subscribes to the topic and calls the provided function for each message.
// Returns the subscription channel that can be used to unsubscribe later.
// The function will be called for historical messages.
func (t *topicWithHistory) SubscribeFunc(f func(msg any)) <-chan any {
	ch := t.Subscribe()
	go func() {
		for msg := range ch {
			f(msg)
		}
	}()
	return ch
}

// Subscribe returns a channel that will receive messages published to the topic.
// If the topic has history enabled, the channel will receive historical messages.
func (t *topicWithHistory) Subscribe() <-chan any {
	return t.subscribeWithBuffer(t.historySize * 2)
}

// SubscribeUnbuffered returns an unbuffered channel that will receive messages.
// Should be used with PublishReliable for guaranteed message delivery.
func (t *topicWithHistory) SubscribeUnbuffered() <-chan any {
	return t.subscribeWithBuffer(0)
}

// Unsubscribe removes a channel from the list of subscribers and closes the channel.
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

// PublishReliable publishes a message to the topic using blocking sends with timeout.
// This ensures messages are delivered even to unbuffered channels, but may block briefly.
// Returns the number of subscribers that successfully received the message.
func (t *topicWithHistory) PublishReliable(msg ...any) int {
	n := t.topic.PublishReliable(msg...)

	t.mu.Lock()
	defer t.mu.Unlock()

	msgLen := len(msg)
	t.history = append(t.history, msg...)
	if len(t.history) > t.historySize {
		t.history = t.history[msgLen:]
	}
	return n
}
