package pubsub

import (
	"context"
	"sync"

	"github.com/dioad/pubsub/pkg/ringbuffer"
)

// historyStore defines the interface for message history storage.
type historyStore interface {
	// Push adds a message to the history.
	Push(msg any)
	// GetAll returns all messages in the history.
	GetAll() []any
	// Cap returns the capacity of the history store.
	Cap() int
}

// sliceHistory is a mutex-protected slice-based history store.
type sliceHistory struct {
	mu      sync.RWMutex
	history []any
	size    int
}

// newSliceHistory creates a new slice-based history store.
func newSliceHistory(size int) *sliceHistory {
	return &sliceHistory{
		history: make([]any, 0, size),
		size:    size,
	}
}

// Push adds messages to the history.
func (h *sliceHistory) Push(msg any) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.history = append(h.history, msg)
	if len(h.history) > h.size {
		h.history = h.history[1:]
	}
}

// GetAll returns all messages in the history.
func (h *sliceHistory) GetAll() []any {
	h.mu.RLock()
	defer h.mu.RUnlock()

	result := make([]any, len(h.history))
	copy(result, h.history)
	return result
}

// Cap returns the capacity of the history store.
func (h *sliceHistory) Cap() int {
	return h.size
}

// ringBufferHistory wraps a ringBuffer to implement historyStore.
type ringBufferHistory struct {
	rb *ringbuffer.RingBuffer
}

// newRingBufferHistory creates a new ring buffer-based history store.
func newRingBufferHistory(size int) *ringBufferHistory {
	return &ringBufferHistory{
		rb: ringbuffer.New(size),
	}
}

// Push adds a message to the history.
func (h *ringBufferHistory) Push(msg any) {
	h.rb.Push(msg)
}

// GetAll returns all messages in the history.
func (h *ringBufferHistory) GetAll() []any {
	return h.rb.GetAll()
}

// Cap returns the capacity of the history store.
func (h *ringBufferHistory) Cap() int {
	return h.rb.Cap()
}

// topicWithHistory implements the Topic interface and maintains a history
// of published messages using the provided history store.
type topicWithHistory struct {
	topic   *pubsubTopic
	history historyStore
}

// newTopicWithHistory creates a new Topic instance with mutex-protected history.
func newTopicWithHistory(o Observer, size int) Topic {
	return &topicWithHistory{
		topic:   newTopic(o),
		history: newSliceHistory(size),
	}
}

// newTopicWithLockFreeHistory creates a new Topic with lock-free history.
func newTopicWithLockFreeHistory(o Observer, size int) Topic {
	return &topicWithHistory{
		topic:   newTopic(o),
		history: newRingBufferHistory(size),
	}
}

// publishAndRecord publishes messages and records them in history.
func (t *topicWithHistory) publishAndRecord(publishFn func(...any) any, msg ...any) any {
	res := publishFn(msg...)
	for _, m := range msg {
		t.history.Push(m)
	}
	return res
}

// Publish publishes a message to the topic and stores it in history.
func (t *topicWithHistory) Publish(msg ...any) PublishResult {
	return t.publishAndRecord(func(m ...any) any { return t.topic.Publish(m...) }, msg...).(PublishResult)
}

// PublishReliable publishes a message using blocking sends with timeout.
func (t *topicWithHistory) PublishReliable(msg ...any) int {
	return t.publishAndRecord(func(m ...any) any { return t.topic.PublishReliable(m...) }, msg...).(int)
}

// Subscribe returns a channel that will receive messages.
func (t *topicWithHistory) Subscribe() <-chan any {
	return t.subscribeWithBuffer(t.history.Cap() * 2)
}

// SubscribeWithBuffer returns a channel with a custom buffer size.
func (t *topicWithHistory) SubscribeWithBuffer(size int) <-chan any {
	return t.subscribeWithBuffer(size)
}

// subscribeWithBuffer creates a subscription and sends historical messages.
func (t *topicWithHistory) subscribeWithBuffer(size int) chan any {
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
func (t *topicWithHistory) SubscribeFunc(f func(msg any)) <-chan any {
	return subscribeWithFunc(t.Subscribe, f)
}

// SubscribeUnbuffered returns an unbuffered channel.
func (t *topicWithHistory) SubscribeUnbuffered() <-chan any {
	return t.subscribeWithBuffer(0)
}

// Unsubscribe removes a subscription.
func (t *topicWithHistory) Unsubscribe(ch <-chan any) {
	t.topic.Unsubscribe(ch)
}

// Close shuts down the topic and closes all subscription channels.
func (t *topicWithHistory) Close() {
	t.topic.Close()
}

// Shutdown closes all subscriptions with context for timeout support.
func (t *topicWithHistory) Shutdown(ctx context.Context) {
	t.topic.Shutdown(ctx)
}

// setName sets the name of the topic for observer callbacks.
// This is an internal method used by PubSub to configure topic names.
func (t *topicWithHistory) setName(name string) {
	t.topic.setName(name)
}
