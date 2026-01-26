package pubsub

import (
	"sync"
)

// Opt is a functional option for configuring a PubSub instance.
type Opt func(*pubSub)

// WithHistorySize enables message history for all topics created by the PubSub instance.
// It sets the maximum number of historical messages to store per topic.
func WithHistorySize(size int) Opt {
	return func(ps *pubSub) {
		ps.topicFunc = func() Topic { return NewTopicWithHistory(size) }
	}
}

// EventTuple represents a message and its associated topic, used by Feeders.
type EventTuple struct {
	// Topic is the name of the topic the message belongs to.
	Topic string
	// Event is the actual message content.
	Event interface{}
}

// Feeder is an interface for components that provide a stream of messages to PubSub.
type Feeder interface {
	// Feed returns a channel that emits EventTuple instances to be published.
	Feed() <-chan *EventTuple
}

// FeedingFunc is a function type that acts as a Feeder.
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

	// PublishReliable sends messages to a specific topic using blocking sends with a timeout.
	// This ensures messages are delivered even to unbuffered channels, but may block briefly.
	// Returns the total number of deliveries across the topic and the "*" topic.
	PublishReliable(topic string, msg ...interface{}) int

	// AddFeederReliable registers a Feeder that will provide messages to the PubSub system using reliable delivery.
	AddFeederReliable(f Feeder)

	// AddFeedingFuncReliable registers a FeedingFunc that will provide messages to the PubSub system using reliable delivery.
	AddFeedingFuncReliable(f FeedingFunc)

	// SubscribeUnbuffered creates a subscription to a specific topic and returns an unbuffered channel.
	// This should be used with PublishReliable for guaranteed message delivery.
	SubscribeUnbuffered(topic string) <-chan interface{}

	// SubscribeAllUnbuffered creates a subscription to all topics and returns an unbuffered channel.
	SubscribeAllUnbuffered() <-chan interface{}
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

// SubscribeFunc registers a callback function that will be invoked for
// each message published to the specified topic.
// If history is enabled, the callback will be invoked for historical messages.
func (ps *pubSub) SubscribeFunc(topic string, f func(msg interface{})) {
	ps.Topic(topic).SubscribeFunc(f)
}

// Subscribe creates a subscription to a specific topic and returns a channel
// that will receive all messages published to that topic.
// If history is enabled, new subscribers will receive historical messages.
// "*" is a special topic that will receive all messages to all topics.
func (ps *pubSub) Subscribe(topic string) <-chan interface{} {
	return ps.Topic(topic).Subscribe()
}

// SubscribeAllFunc registers a callback function that will be invoked
// for every message published to any topic.
// If history is enabled, the callback will be invoked for historical messages from all topics.
func (ps *pubSub) SubscribeAllFunc(f func(msg interface{})) {
	ps.SubscribeFunc("*", f)
}

// SubscribeAll creates a subscription to all topics by subscribing to
// the special "*" topic and returns a channel for receiving messages.
// If history is enabled, new subscribers will receive historical messages from all topics.
func (ps *pubSub) SubscribeAll() <-chan interface{} {
	return ps.Topic("*").Subscribe()
}

// Topics returns a list of all currently registered topics.
func (ps *pubSub) Topics() []string {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	topicNames := make([]string, 0, len(ps.subscribers))
	for topicName := range ps.subscribers {
		topicNames = append(topicNames, topicName)
	}
	return topicNames
}

// Topic returns a Topic instance for the given topic name.
// If the topic doesn't exist, it creates a new one.
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

// Publish sends messages to a specific topic. All subscribers to the topic
// and subscribers to the "*" topic will receive these messages.
func (ps *pubSub) Publish(topic string, msg ...interface{}) int {
	total := ps.publishToTopic(topic, msg...)
	total += ps.publishToTopic("*", msg...)
	return total
}

func (ps *pubSub) publishReliableToTopic(topic string, msg ...interface{}) int {
	return ps.Topic(topic).PublishReliable(msg...)
}

// PublishReliable publishes a message to a topic using reliable delivery.
func (ps *pubSub) PublishReliable(topic string, msg ...interface{}) int {
	total := ps.publishReliableToTopic(topic, msg...)
	total += ps.publishReliableToTopic("*", msg...)
	return total
}

// UnsubscribeAll unsubscribes a channel from the "*" special topic
func (ps *pubSub) UnsubscribeAll(sub <-chan interface{}) {
	ps.Unsubscribe("*", sub)
}

// Unsubscribe removes a subscription channel from a specific topic.
// After unsubscribing, the channel will be closed.
func (ps *pubSub) Unsubscribe(topic string, sub <-chan interface{}) {
	ps.Topic(topic).Unsubscribe(sub)
}

// SubscribeUnbuffered returns an unbuffered channel for a topic.
func (ps *pubSub) SubscribeUnbuffered(topic string) <-chan interface{} {
	return ps.Topic(topic).SubscribeUnbuffered()
}

// SubscribeAllUnbuffered returns an unbuffered channel for all topics.
func (ps *pubSub) SubscribeAllUnbuffered() <-chan interface{} {
	return ps.SubscribeUnbuffered("*")
}

// AddFeeder registers a Feeder that will provide messages to the PubSub system.
// The Feeder should implement the Feed method, which returns a channel that emits
// EventTuple instances. Each EventTuple contains the topic and the event message.
// This allows for dynamic message feeding into the PubSub system.
func (ps *pubSub) AddFeeder(f Feeder) {
	ps.AddFeedingFunc(f.Feed)
}

// AddFeedingFunc registers a FeedingFunc that will provide messages to the PubSub system.
func (ps *pubSub) AddFeedingFunc(f FeedingFunc) {
	if f == nil {
		return
	}
	go func() {
		feedChan := f()
		if feedChan == nil {
			return
		}
		for eventTuple := range feedChan {
			if eventTuple != nil {
				ps.Publish(eventTuple.Topic, eventTuple.Event)
			}
		}
	}()
}

// AddFeederReliable registers a Feeder that will provide messages using reliable delivery.
func (ps *pubSub) AddFeederReliable(f Feeder) {
	ps.AddFeedingFuncReliable(f.Feed)
}

// AddFeedingFuncReliable registers a FeedingFunc that will provide messages using reliable delivery.
func (ps *pubSub) AddFeedingFuncReliable(f FeedingFunc) {
	if f == nil {
		return
	}
	go func() {
		feedChan := f()
		if feedChan == nil {
			return
		}
		for eventTuple := range feedChan {
			if eventTuple != nil {
				ps.PublishReliable(eventTuple.Topic, eventTuple.Event)
			}
		}
	}()
}
