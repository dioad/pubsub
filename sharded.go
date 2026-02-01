package pubsub

import (
	"context"
	"hash/fnv"
	"sync"
)

const numShards = 16

// shard represents a single shard of the topic map.
type shard struct {
	mu     sync.RWMutex
	topics sync.Map // map[string]Topic - lock-free for reads
}

// shardedPubSub is a high-performance publish/subscribe implementation
// that uses sharding to reduce lock contention across multiple CPUs.
type shardedPubSub struct {
	shards    [numShards]shard
	topicFunc func() Topic
	observer  Observer
}

// NewShardedPubSub creates a new sharded PubSub instance optimized for
// high-concurrency scenarios. It distributes topics across 16 shards
// to minimize lock contention by reducing the frequency of global lock acquisition.
// This is particularly beneficial when many topics are being created or accessed
// simultaneously by different goroutines.
func NewShardedPubSub(opt ...Opt) PubSub {
	ps := &shardedPubSub{
		topicFunc: nil,
		observer:  NoopObserver{},
	}

	// Apply options
	wrapper := &pubSub{observer: ps.observer}
	for _, o := range opt {
		o(wrapper)
	}
	ps.observer = wrapper.observer
	ps.topicFunc = wrapper.topicFunc

	if ps.topicFunc == nil {
		ps.topicFunc = func() Topic { return NewTopic(WithTopicObserver(ps.observer)) }
	}

	return ps
}

// getShard returns the shard for a given topic name using FNV-1a hash.
func (ps *shardedPubSub) getShard(topic string) *shard {
	h := fnv.New32a()
	h.Write([]byte(topic))
	return &ps.shards[h.Sum32()%numShards]
}

// Topic returns a Topic instance for the given topic name.
// Uses sync.Map for lock-free reads; only locks for creation.
func (ps *shardedPubSub) Topic(topic string) Topic {
	s := ps.getShard(topic)

	// Fast path: lock-free read from sync.Map
	if t, ok := s.topics.Load(topic); ok {
		return t.(Topic)
	}

	// Slow path: lock to create topic atomically
	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-check after acquiring lock
	if t, ok := s.topics.Load(topic); ok {
		return t.(Topic)
	}

	t := ps.topicFunc()
	if ot, ok := t.(*pubsubTopic); ok {
		ot.name = topic
	} else if ht, ok := t.(*topicWithHistory); ok {
		ht.topic.name = topic
	}
	s.topics.Store(topic, t)
	return t
}

// Topics returns a list of all currently registered topics.
func (ps *shardedPubSub) Topics() []string {
	var topics []string
	for i := 0; i < numShards; i++ {
		ps.shards[i].topics.Range(func(key, value any) bool {
			topics = append(topics, key.(string))
			return true
		})
	}
	return topics
}

// Publish sends messages to a specific topic and the "*" catch-all topic.
func (ps *shardedPubSub) Publish(topic string, msg ...interface{}) int {
	total := ps.Topic(topic).Publish(msg...)
	total += ps.Topic("*").Publish(msg...)
	return total
}

// PublishReliable sends messages using blocking sends with timeout.
func (ps *shardedPubSub) PublishReliable(topic string, msg ...interface{}) int {
	total := ps.Topic(topic).PublishReliable(msg...)
	total += ps.Topic("*").PublishReliable(msg...)
	return total
}

// Subscribe creates a subscription to a specific topic.
func (ps *shardedPubSub) Subscribe(topic string) <-chan interface{} {
	return ps.Topic(topic).Subscribe()
}

// SubscribeFunc registers a callback function for a topic.
func (ps *shardedPubSub) SubscribeFunc(topic string, f func(msg interface{})) {
	ps.Topic(topic).SubscribeFunc(f)
}

// Unsubscribe removes a subscription from a topic.
func (ps *shardedPubSub) Unsubscribe(topic string, sub <-chan interface{}) {
	ps.Topic(topic).Unsubscribe(sub)
}

// SubscribeAll subscribes to all topics via the "*" topic.
func (ps *shardedPubSub) SubscribeAll() <-chan interface{} {
	return ps.Subscribe("*")
}

// SubscribeAllFunc registers a callback for all topics.
func (ps *shardedPubSub) SubscribeAllFunc(f func(msg interface{})) {
	ps.SubscribeFunc("*", f)
}

// UnsubscribeAll removes a subscription from the "*" topic.
func (ps *shardedPubSub) UnsubscribeAll(sub <-chan interface{}) {
	ps.Unsubscribe("*", sub)
}

// SubscribeUnbuffered returns an unbuffered channel for a topic.
func (ps *shardedPubSub) SubscribeUnbuffered(topic string) <-chan interface{} {
	return ps.Topic(topic).SubscribeUnbuffered()
}

// SubscribeAllUnbuffered returns an unbuffered channel for all topics.
func (ps *shardedPubSub) SubscribeAllUnbuffered() <-chan interface{} {
	return ps.SubscribeUnbuffered("*")
}

// AddFeeder registers a Feeder for message feeding.
// The context can be used to cancel the feeding process.
func (ps *shardedPubSub) AddFeeder(ctx context.Context, f Feeder) {
	ps.AddFeedingFunc(ctx, f.Feed)
}

// AddFeedingFunc registers a FeedingFunc for message feeding.
// The context can be used to cancel the feeding process.
func (ps *shardedPubSub) AddFeedingFunc(ctx context.Context, f FeedingFunc) {
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

// AddFeederReliable registers a Feeder using reliable delivery.
// The context can be used to cancel the feeding process.
func (ps *shardedPubSub) AddFeederReliable(ctx context.Context, f Feeder) {
	ps.AddFeedingFuncReliable(ctx, f.Feed)
}

// AddFeedingFuncReliable registers a FeedingFunc using reliable delivery.
// The context can be used to cancel the feeding process.
func (ps *shardedPubSub) AddFeedingFuncReliable(ctx context.Context, f FeedingFunc) {
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
func (ps *shardedPubSub) Shutdown(ctx context.Context) {
	for i := 0; i < numShards; i++ {
		ps.shards[i].mu.Lock()
		ps.shards[i].topics.Range(func(key, value any) bool {
			select {
			case <-ctx.Done():
				return false
			default:
				value.(Topic).Close()
				return true
			}
		})
		ps.shards[i].topics = sync.Map{}
		ps.shards[i].mu.Unlock()

		select {
		case <-ctx.Done():
			return
		default:
		}
	}
}

// Close is an alias for Shutdown with a background context.
func (ps *shardedPubSub) Close() {
	ps.Shutdown(context.Background())
}
