package pubsub

import (
	"sync"
	"time"
)

// Opt is a functional option for pubSub
type Opt func(*pubSub)

// WithHistorySize enables history and sets the size of the history buffer
func WithHistorySize(size int) Opt {
	return func(ps *pubSub) {
		ps.topicFunc = func() Topic { return NewTopicWithHistory(size) }
	}
}

type EventTuple struct {
	Topic string
	Event interface{}
}

type Feeder interface {
	Feed() <-chan *EventTuple
}

type FeedingFunc func() <-chan *EventTuple

// PubSub is a thread-safe publish/subscribe message broker interface that enables
// topic-based message distribution. It supports both direct channel subscriptions
// and callback-based message handling. The interface provides message history
// capabilities when enabled, and supports a special "*" topic that receives
// all messages from all topics.
//
// Basic usage:
//
//	// Create a new PubSub instance
//	ps := pubsub.NewPubSub()
//
//	// Subscribe to a topic
//	ch := ps.Subscribe("notifications")
//
//	// Process messages in a goroutine
//	go func() {
//	    for msg := range ch {
//	        fmt.Printf("Received: %v\n", msg)
//	    }
//	}()
//
//	// Publish messages to the topic
//	ps.Publish("notifications", "Hello, World!")
//
//	// With history:
//	ps := pubsub.NewPubSub(pubsub.WithHistorySize(10))
type PubSub interface {
	// Publish sends messages to a specific topic. All subscribers to the topic
	// and subscribers to the "*" topic will receive these messages.
	Publish(topic string, msg ...interface{}) int

	// AddFeeder registers a Feeder that will provide messages to the PubSub system.
	// The Feeder should implement the Feed method, which returns a channel that emits
	// EventTuple instances. Each EventTuple contains the topic and the event message.
	// This allows for dynamic message feeding into the PubSub system.
	AddFeeder(f Feeder)

	// AddFeedingFunc registers a FeedingFunc that will provide messages to the PubSub system.
	// The FeedingFunc should return a channel that emits EventTuple instances.
	// Each EventTuple contains the topic and the event message.
	// This allows for dynamic message feeding into the PubSub system.
	AddFeedingFunc(f FeedingFunc)

	// Topic returns a Topic instance for the given topic name.
	// If the topic doesn't exist, it creates a new one.
	Topic(topic string) Topic

	// Topics returns a list of all currently registered topics.
	Topics() []string

	// Subscribe creates a subscription to a specific topic and returns a channel
	// that will receive all messages published to that topic.
	// If history is enabled, new subscribers will receive historical messages.
	Subscribe(topic string) <-chan interface{}

	// SubscribeFunc registers a callback function that will be invoked for
	// each message published to the specified topic.
	// If history is enabled, the callback will be invoked for historical messages.
	SubscribeFunc(topic string, f func(msg interface{}))

	// Unsubscribe removes a subscription channel from a specific topic.
	// After unsubscribing, the channel will be closed.
	Unsubscribe(topic string, sub <-chan interface{})

	// SubscribeAll creates a subscription to all topics by subscribing to
	// the special "*" topic and returns a channel for receiving messages.
	// If history is enabled, new subscribers will receive historical messages from all topics.
	SubscribeAll() <-chan interface{}

	// SubscribeAllFunc registers a callback function that will be invoked
	// for every message published to any topic.
	// If history is enabled, the callback will be invoked for historical messages from all topics.
	SubscribeAllFunc(f func(msg interface{}))

	// UnsubscribeAll removes a subscription channel from the special "*" topic,
	// effectively unsubscribing from all messages.
	// After unsubscribing, the channel will be closed.
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
	ps.Topic(topic).SubscribeFunc(f)
}

// Subscribe subscribes to a topic and returns a channel that will receive messages published to the topic
// If withHistory is true, the channel will be populated with all messages in the history
// "*" is a special topic that will receive all messages to all topics
func (ps *pubSub) Subscribe(topic string) <-chan interface{} {
	return ps.Topic(topic).Subscribe()
}

// SubscribeAllFunc subscribes to all topics and calls the provided function for each received message
func (ps *pubSub) SubscribeAllFunc(f func(msg interface{})) {
	ps.SubscribeFunc("*", f)
}

// SubscribeAll subscribes to all topics and returns a channel that will receive all messages published to all topics
func (ps *pubSub) SubscribeAll() <-chan interface{} {
	return ps.Topic("*").Subscribe()
}

func (ps *pubSub) Topics() []string {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	topicNames := make([]string, 0, len(ps.subscribers))
	for topicName := range ps.subscribers {
		topicNames = append(topicNames, topicName)
	}
	return topicNames
}

func (ps *pubSub) Topic(topic string) Topic {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if _, ok := ps.subscribers[topic]; !ok {
		ps.subscribers[topic] = ps.topicFunc()
	}
	return ps.subscribers[topic]
}

func (ps *pubSub) publishToTopic(topic string, msg ...interface{}) int {
	return ps.Topic(topic).Publish(msg...)
}

// Publish publishes a message to a topic
func (ps *pubSub) Publish(topic string, msg ...interface{}) int {
	total := ps.publishToTopic(topic, msg...)
	total += ps.publishToTopic("*", msg...)
	return total
}

// UnsubscribeAll unsubscribes a channel from the "*" special topic
func (ps *pubSub) UnsubscribeAll(sub <-chan interface{}) {
	ps.Unsubscribe("*", sub)
}

// Unsubscribe unsubscribes a channel from a topic
func (ps *pubSub) Unsubscribe(topic string, sub <-chan interface{}) {
	ps.Topic(topic).Unsubscribe(sub)
}

// AddFeeder Complete the implementation of AddFeeder
func (ps *pubSub) AddFeeder(f Feeder) {
	ps.AddFeedingFunc(f.Feed)
}

// AddFeedingFunc registers a FeedingFunc that will provide messages to the PubSub system.
func (ps *pubSub) AddFeedingFunc(f FeedingFunc) {
	if f == nil {
		return // Exit if the feeding function is nil
	}
	go func() {
		feedChan := f()
		for {
			select {
			case eventTuple, ok := <-feedChan:
				if !ok {
					return // Exit if the channel is closed
				}
				ps.Publish(eventTuple.Topic, eventTuple.Event)
				// If the feed channel is closed, exit the loop
				// return
			case <-time.After(10 * time.Millisecond):
				// default:
				// Continue processing messages from the feed channel
			}

			// Wait for the feeding function to return a channel
			// eventTuple, ok := <-feedChan
			// if !ok {
			// 	return // Exit if the channel is closed
			// }

		}

		// for eventTuple := range f() {
		// 	ps.Publish(eventTuple.Topic, eventTuple.Event)
		// }
	}()
}
