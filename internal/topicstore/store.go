// Package topicstore provides topic storage implementations for the pubsub package.
package topicstore

import (
	"context"
	"hash/fnv"
	"sync"
)

// Topic represents the interface that stored topics must implement.
// This is a minimal interface to avoid circular dependencies.
type Topic interface {
	Close()
}

// Store defines the interface for topic storage backends.
// Implementations must be safe for concurrent use.
type Store interface {
	// GetOrCreate returns an existing topic or creates a new one using the factory.
	GetOrCreate(name string, factory func() Topic, onCreated func(Topic)) Topic

	// Range iterates over all topics. Return false from fn to stop iteration.
	Range(fn func(name string, topic Topic) bool)

	// DeleteTopic removes a topic and closes all its subscriptions.
	DeleteTopic(name string)

	// Shutdown closes all topics with context for timeout support.
	Shutdown(ctx context.Context)
}

// SimpleStore is a basic topic store using a single mutex-protected map.
// Suitable for most use cases with moderate topic counts.
type SimpleStore struct {
	mu     sync.RWMutex
	topics map[string]Topic
}

// NewSimpleStore creates a new SimpleStore.
func NewSimpleStore() *SimpleStore {
	return &SimpleStore{
		topics: make(map[string]Topic),
	}
}

// GetOrCreate returns an existing topic or creates a new one using the factory.
func (s *SimpleStore) GetOrCreate(name string, factory func() Topic, onCreated func(Topic)) Topic {
	s.mu.Lock()
	defer s.mu.Unlock()

	if t, ok := s.topics[name]; ok {
		return t
	}

	t := factory()
	if onCreated != nil {
		onCreated(t)
	}
	s.topics[name] = t
	return t
}

// Range iterates over all topics.
func (s *SimpleStore) Range(fn func(name string, topic Topic) bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for name, topic := range s.topics {
		if !fn(name, topic) {
			return
		}
	}
}

// Clear removes all topics from the store.
func (s *SimpleStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.topics = make(map[string]Topic)
}

// DeleteTopic removes a topic and closes all its subscriptions.
func (s *SimpleStore) DeleteTopic(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if t, ok := s.topics[name]; ok {
		t.Close()
		delete(s.topics, name)
	}
}

// Shutdown closes all topics with context support.
func (s *SimpleStore) Shutdown(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, topic := range s.topics {
		select {
		case <-ctx.Done():
			return
		default:
			topic.Close()
		}
	}
	s.topics = make(map[string]Topic)
}

const numShards = 16

// shard represents a single shard of the topic map.
type shard struct {
	mu     sync.RWMutex
	topics sync.Map
}

// ShardedStore is a high-performance topic store that distributes topics
// across multiple shards to reduce lock contention.
type ShardedStore struct {
	shards [numShards]shard
}

// NewShardedStore creates a new ShardedStore.
func NewShardedStore() *ShardedStore {
	return &ShardedStore{}
}

// getShard returns the shard for a given topic name using FNV-1a hash.
func (s *ShardedStore) getShard(name string) *shard {
	h := fnv.New32a()
	h.Write([]byte(name))
	return &s.shards[h.Sum32()%numShards]
}

// GetOrCreate returns an existing topic or creates a new one using the factory.
func (s *ShardedStore) GetOrCreate(name string, factory func() Topic, onCreated func(Topic)) Topic {
	sh := s.getShard(name)

	// Fast path: lock-free read from sync.Map
	if t, ok := sh.topics.Load(name); ok {
		return t.(Topic)
	}

	// Slow path: lock to create topic atomically
	sh.mu.Lock()
	defer sh.mu.Unlock()

	// Double-check after acquiring lock
	if t, ok := sh.topics.Load(name); ok {
		return t.(Topic)
	}

	t := factory()
	if onCreated != nil {
		onCreated(t)
	}
	sh.topics.Store(name, t)
	return t
}

// Range iterates over all topics across all shards.
func (s *ShardedStore) Range(fn func(name string, topic Topic) bool) {
	for i := 0; i < numShards; i++ {
		continueIteration := true
		s.shards[i].topics.Range(func(key, value any) bool {
			if !fn(key.(string), value.(Topic)) {
				continueIteration = false
				return false
			}
			return true
		})
		if !continueIteration {
			return
		}
	}
}

// Clear removes all topics from the store.
func (s *ShardedStore) Clear() {
	for i := 0; i < numShards; i++ {
		s.shards[i].mu.Lock()
		s.shards[i].topics = sync.Map{}
		s.shards[i].mu.Unlock()
	}
}

// DeleteTopic removes a topic and closes all its subscriptions.
func (s *ShardedStore) DeleteTopic(name string) {
	sh := s.getShard(name)
	sh.mu.Lock()
	defer sh.mu.Unlock()
	if t, ok := sh.topics.LoadAndDelete(name); ok {
		t.(Topic).Close()
	}
}

// Shutdown closes all topics with context support.
func (s *ShardedStore) Shutdown(ctx context.Context) {
	for i := 0; i < numShards; i++ {
		s.shards[i].mu.Lock()
		s.shards[i].topics.Range(func(key, value any) bool {
			select {
			case <-ctx.Done():
				return false
			default:
				value.(Topic).Close()
				return true
			}
		})
		s.shards[i].topics = sync.Map{}
		s.shards[i].mu.Unlock()

		select {
		case <-ctx.Done():
			return
		default:
		}
	}
}
