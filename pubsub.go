package pubsub

import (
	"sync"
)

// Opt is a functional option for PubSub
type Opt func(*PubSub)

// WithHistorySize enables history and sets the size of the history buffer
func WithHistorySize(size int) Opt {
	return func(ps *PubSub) {
		ps.historySize = size
		ps.enableHistory = true
	}
}

// PubSub is a simple publish/subscribe implementation
// It supports subscribing to topics and publishing messages to topics
// Optionally, it can keep a history of messages for each topic
// and deliver them to new subscribers
// It is safe for concurrent use
type PubSub struct {
	mu            sync.RWMutex
	subscribers   map[string][]chan interface{}
	history       map[string][]interface{}
	historySize   int
	enableHistory bool
}

// NewPubSub creates a new PubSub instance
// Optionally pass WithHistorySize to enable history
// and set the size of the history buffer
func NewPubSub(opt ...Opt) *PubSub {
	ps := &PubSub{
		subscribers: make(map[string][]chan interface{}),
	}

	for _, o := range opt {
		o(ps)
	}

	if ps.enableHistory {
		ps.history = make(map[string][]interface{})
	}

	return ps
}

// SubscribeFunc subscribes to a topic and calls the provided function for each received message
// If withHistory is true, the function will be called with all messages in the history
func (ps *PubSub) SubscribeFunc(topic string, withHistory bool, f func(msg interface{})) {
	ch := ps.Subscribe(topic, withHistory)
	go func() {
		for msg := range ch {
			f(msg)
		}
	}()
}

// Subscribe subscribes to a topic and returns a channel that will receive messages published to the topic
// If withHistory is true, the channel will be populated with all messages in the history
// "*" is a special topic that will receive all messages to all topics
func (ps *PubSub) Subscribe(topic string, withHistory bool) <-chan interface{} {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	ch := make(chan interface{})

	if withHistory {
		// Is it confusing having a buffer withHistory and no buffer without
		ch = make(chan interface{}, ps.historySize*2)
		if history, ok := ps.history[topic]; ok {
			for _, msg := range history {
				ch <- msg
			}
		}
	}
	ps.subscribers[topic] = append(ps.subscribers[topic], ch)

	return ch
}

// SubscribeAllFunc subscribes to all topics and calls the provided function for each received message
func (ps *PubSub) SubscribeAllFunc(withHistory bool, f func(msg interface{})) {
	ps.SubscribeFunc("*", withHistory, f)
}

// SubscribeAll subscribes to all topics and returns a channel that will receive all messages published to all topics
func (ps *PubSub) SubscribeAll(withHistory bool) <-chan interface{} {
	return ps.Subscribe("*", withHistory)
}

func (ps *PubSub) publishToTopicWithHistory(topic string, msg interface{}) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	// Add message to history
	if ps.enableHistory {
		ps.history[topic] = append(ps.history[topic], msg)
		if len(ps.history[topic]) > ps.historySize {
			ps.history[topic] = ps.history[topic][1:]
		}
	}

	for _, ch := range ps.subscribers[topic] {
		go func(c chan interface{}) {
			c <- msg
		}(ch)
	}
}

// Publish publishes a message to a topic
func (ps *PubSub) Publish(topic string, msg interface{}) {
	ps.publishToTopicWithHistory(topic, msg)
	ps.publishToTopicWithHistory("*", msg)
}

// UnsubscribeAll unsubscribes a channel from the "*" special topic
func (ps *PubSub) UnsubscribeAll(sub <-chan interface{}) {
	ps.Unsubscribe("*", sub)
}

// Unsubscribe unsubscribes a channel from a topic
func (ps *PubSub) Unsubscribe(topic string, sub <-chan interface{}) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	subs := ps.subscribers[topic]
	for i, ch := range subs {
		if ch == sub {
			ps.subscribers[topic] = append(subs[:i], subs[i+1:]...)
			close(ch)
			break
		}
	}
}
