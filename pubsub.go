package pubsub

import (
	"sync"
)

// Opt is a functional option for pubSub
type Opt func(*pubSub)

// WithHistorySize enables history and sets the size of the history buffer
func WithHistorySize(size int) Opt {
	return func(ps *pubSub) {
		ps.topicFunc = func() Topic { return NewTopicWithHistory(size) }
	}
}

type PubSub interface {
	Publish(topic string, msg ...interface{})

	Subscribe(topic string) <-chan interface{}
	SubscribeFunc(topic string, f func(msg interface{}))
	Unsubscribe(topic string, sub <-chan interface{})

	SubscribeAll() <-chan interface{}
	SubscribeAllFunc(f func(msg interface{}))
	UnsubscribeAll(sub <-chan interface{})
}

// pubSub is a simple publish/subscribe implementation
// It supports subscribing to topics and publishing messages to topics
// Optionally, it can keep a history of messages for each topic
// and deliver them to new subscribers
// It is safe for concurrent use
type pubSub struct {
	mu          sync.RWMutex
	subscribers map[string]Topic
	topicFunc   func() Topic
}

// NewPubSub creates a new pubSub instance
// Optionally pass WithHistorySize to enable history
// and set the size of the history buffer
func NewPubSub(opt ...Opt) PubSub {
	ps := &pubSub{
		subscribers: make(map[string]Topic),
		topicFunc:   NewTopic,
	}

	for _, o := range opt {
		o(ps)
	}

	return ps
}

// SubscribeFunc subscribes to a topic and calls the provided function for each received message
// If withHistory is true, the function will be called with all messages in the history
func (ps *pubSub) SubscribeFunc(topic string, f func(msg interface{})) {
	ps.getTopic(topic).SubscribeFunc(f)
}

// Subscribe subscribes to a topic and returns a channel that will receive messages published to the topic
// If withHistory is true, the channel will be populated with all messages in the history
// "*" is a special topic that will receive all messages to all topics
func (ps *pubSub) Subscribe(topic string) <-chan interface{} {
	return ps.getTopic(topic).Subscribe()
}

// SubscribeAllFunc subscribes to all topics and calls the provided function for each received message
func (ps *pubSub) SubscribeAllFunc(f func(msg interface{})) {
	ps.SubscribeFunc("*", f)
}

// SubscribeAll subscribes to all topics and returns a channel that will receive all messages published to all topics
func (ps *pubSub) SubscribeAll() <-chan interface{} {
	return ps.getTopic("*").Subscribe()
}

func (ps *pubSub) getTopic(topic string) Topic {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if _, ok := ps.subscribers[topic]; !ok {
		ps.subscribers[topic] = ps.topicFunc()
	}
	return ps.subscribers[topic]
}

func (ps *pubSub) publishToTopic(topic string, msg ...interface{}) {
	ps.getTopic(topic).Publish(msg...)
}

// Publish publishes a message to a topic
func (ps *pubSub) Publish(topic string, msg ...interface{}) {
	ps.publishToTopic(topic, msg...)
	ps.publishToTopic("*", msg...)
}

// UnsubscribeAll unsubscribes a channel from the "*" special topic
func (ps *pubSub) UnsubscribeAll(sub <-chan interface{}) {
	ps.Unsubscribe("*", sub)
}

// Unsubscribe unsubscribes a channel from a topic
func (ps *pubSub) Unsubscribe(topic string, sub <-chan interface{}) {
	ps.getTopic(topic).Unsubscribe(sub)
}
