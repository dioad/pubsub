package pubsub

import (
	"context"
	"sync"
)

// Observer is an interface for components that want to observe events in the PubSub system.
type Observer interface {
	// OnPublish is called when a message is published to a topic.
	OnPublish(topic string, msg interface{})
	// OnSubscribe is called when a new subscription is created for a topic.
	OnSubscribe(topic string)
	// OnUnsubscribe is called when a subscription is removed from a topic.
	OnUnsubscribe(topic string)
}

// NoopObserver is an Observer that does nothing.
type NoopObserver struct{}

func (o NoopObserver) OnPublish(topic string, msg interface{}) {}
func (o NoopObserver) OnSubscribe(topic string)                {}
func (o NoopObserver) OnUnsubscribe(topic string)              {}

// Opt is a functional option for configuring a PubSub instance.
type Opt func(*pubSub)

// WithObserver sets the observer for the PubSub instance.
func WithObserver(o Observer) Opt {
	return func(ps *pubSub) {
		ps.observer = o
	}
}

// WithHistorySize enables message history for all topics created by the PubSub instance.
// It sets the maximum number of historical messages to store per topic.
func WithHistorySize(size int) Opt {
	return func(ps *pubSub) {
		ps.topicFunc = func() Topic { return NewTopic(WithHistory(size), WithTopicObserver(ps.observer)) }
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
// There are two main implementations of the PubSub interface:
//   - Default PubSub (NewPubSub): Appropriate for most use cases with a moderate
//     number of topics and message rates.
//   - Sharded PubSub (NewShardedPubSub): Optimized for high-concurrency scenarios
//     with many topics, reducing lock contention by sharding topics across
//     multiple internal maps.
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
	// The context can be used to cancel the feeding process.
	AddFeeder(ctx context.Context, f Feeder)

	// AddFeedingFunc registers a FeedingFunc that will provide messages to the PubSub system.
	// The FeedingFunc should return a channel that emits EventTuple instances.
	// Each EventTuple contains the topic and the event message.
	// This allows for dynamic message feeding into the PubSub system.
	// The context can be used to cancel the feeding process.
	AddFeedingFunc(ctx context.Context, f FeedingFunc)

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
	// The context can be used to cancel the feeding process.
	AddFeederReliable(ctx context.Context, f Feeder)

	// AddFeedingFuncReliable registers a FeedingFunc that will provide messages to the PubSub system using reliable delivery.
	// The context can be used to cancel the feeding process.
	AddFeedingFuncReliable(ctx context.Context, f FeedingFunc)

	// SubscribeUnbuffered creates a subscription to a specific topic and returns an unbuffered channel.
	// This should be used with PublishReliable for guaranteed message delivery.
	SubscribeUnbuffered(topic string) <-chan interface{}

	// SubscribeAllUnbuffered creates a subscription to all topics and returns an unbuffered channel.
	SubscribeAllUnbuffered() <-chan interface{}

	// Shutdown gracefully shuts down the PubSub instance, closing all topics and subscriptions.
	// The context can be used to set a timeout for the shutdown process.
	Shutdown(ctx context.Context)

	// Close is an alias for Shutdown with a background context.
	Close()
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
	observer    Observer
}

// NewPubSub creates a new default PubSub instance.
// This implementation uses a single map and mutex for topic management,
// which is efficient for most applications with a standard number of topics.
// Optionally pass WithHistorySize to enable history for all topics
// and set the size of the history buffer.
func NewPubSub(opt ...Opt) PubSub {
	ps := &pubSub{
		subscribers: make(map[string]Topic),
		topicFunc:   nil, // Will be initialized below
		observer:    NoopObserver{},
	}

	for _, o := range opt {
		o(ps)
	}

	if ps.topicFunc == nil {
		ps.topicFunc = func() Topic { return NewTopic(WithTopicObserver(ps.observer)) }
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
		t := ps.topicFunc()
		if ot, ok := t.(*pubsubTopic); ok {
			ot.name = topic
		} else if ht, ok := t.(*topicWithHistory); ok {
			ht.topic.name = topic
		}
		ps.subscribers[topic] = t
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
// The context can be used to cancel the feeding process.
func (ps *pubSub) AddFeeder(ctx context.Context, f Feeder) {
	ps.AddFeedingFunc(ctx, f.Feed)
}

// AddFeedingFunc registers a FeedingFunc that will provide messages to the PubSub system.
// The context can be used to cancel the feeding process.
func (ps *pubSub) AddFeedingFunc(ctx context.Context, f FeedingFunc) {
	if f == nil {
		return
	}
	go func() {
		feedChan := f()
		if feedChan == nil {
			return
		}
		for {
			select {
			case <-ctx.Done():
				return
			case eventTuple, ok := <-feedChan:
				if !ok {
					return
				}
				if eventTuple != nil {
					ps.Publish(eventTuple.Topic, eventTuple.Event)
				}
			}
		}
	}()
}

// AddFeederReliable registers a Feeder that will provide messages using reliable delivery.
// The context can be used to cancel the feeding process.
func (ps *pubSub) AddFeederReliable(ctx context.Context, f Feeder) {
	ps.AddFeedingFuncReliable(ctx, f.Feed)
}

// AddFeedingFuncReliable registers a FeedingFunc that will provide messages using reliable delivery.
// The context can be used to cancel the feeding process.
func (ps *pubSub) AddFeedingFuncReliable(ctx context.Context, f FeedingFunc) {
	if f == nil {
		return
	}
	go func() {
		feedChan := f()
		if feedChan == nil {
			return
		}
		for {
			select {
			case <-ctx.Done():
				return
			case eventTuple, ok := <-feedChan:
				if !ok {
					return
				}
				if eventTuple != nil {
					ps.PublishReliable(eventTuple.Topic, eventTuple.Event)
				}
			}
		}
	}()
}

// Shutdown gracefully shuts down the PubSub instance, closing all topics and subscriptions.
// The context can be used to set a timeout for the shutdown process.
func (ps *pubSub) Shutdown(ctx context.Context) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	for _, topic := range ps.subscribers {
		select {
		case <-ctx.Done():
			return
		default:
			topic.Close()
		}
	}
	ps.subscribers = make(map[string]Topic)
}

// Close is an alias for Shutdown with a background context.
func (ps *pubSub) Close() {
	ps.Shutdown(context.Background())
}
