package pubsub

import (
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
}

// NewShardedPubSub creates a new sharded PubSub instance optimized for
// high-concurrency scenarios. It distributes topics across 16 shards
// to minimize lock contention.
func NewShardedPubSub(opt ...Opt) PubSub {
	ps := &shardedPubSub{
		topicFunc: NewTopic,
	}

	// Apply options
	wrapper := &pubSub{topicFunc: ps.topicFunc}
	for _, o := range opt {
		o(wrapper)
	}
	ps.topicFunc = wrapper.topicFunc

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
func (ps *shardedPubSub) AddFeeder(f Feeder) {
	ps.AddFeedingFunc(f.Feed)
}

// AddFeedingFunc registers a FeedingFunc for message feeding.
func (ps *shardedPubSub) AddFeedingFunc(f FeedingFunc) {
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

// AddFeederReliable registers a Feeder using reliable delivery.
func (ps *shardedPubSub) AddFeederReliable(f Feeder) {
	ps.AddFeedingFuncReliable(f.Feed)
}

// AddFeedingFuncReliable registers a FeedingFunc using reliable delivery.
func (ps *shardedPubSub) AddFeedingFuncReliable(f FeedingFunc) {
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
