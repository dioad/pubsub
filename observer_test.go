package pubsub

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

type mockObserver struct {
	mu              sync.Mutex
	publishes       map[string]int
	subscriptions   map[string]int
	unsubscriptions map[string]int
}

func newMockObserver() *mockObserver {
	return &mockObserver{
		publishes:       make(map[string]int),
		subscriptions:   make(map[string]int),
		unsubscriptions: make(map[string]int),
	}
}

func (o *mockObserver) OnPublish(topic string, msg interface{}) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.publishes[topic]++
}

func (o *mockObserver) OnSubscribe(topic string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.subscriptions[topic]++
}

func (o *mockObserver) OnUnsubscribe(topic string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.unsubscriptions[topic]++
}

func TestObserver_PubSub(t *testing.T) {
	obs := newMockObserver()
	ps := NewPubSub(WithObserver(obs))

	topic := "test-topic"
	ch := ps.Subscribe(topic)
	ps.Publish(topic, "msg1")
	ps.Unsubscribe(topic, ch)

	obs.mu.Lock()
	defer obs.mu.Unlock()

	assert.Equal(t, 1, obs.subscriptions[topic])
	assert.Equal(t, 1, obs.publishes[topic])
	// Also "*" topic for Publish
	assert.Equal(t, 1, obs.publishes["*"])
	assert.Equal(t, 1, obs.unsubscriptions[topic])
}

func TestObserver_ShardedPubSub(t *testing.T) {
	obs := newMockObserver()
	ps := NewShardedPubSub(WithObserver(obs))

	topic := "test-topic"
	ch := ps.Subscribe(topic)
	ps.Publish(topic, "msg1")
	ps.Unsubscribe(topic, ch)

	obs.mu.Lock()
	defer obs.mu.Unlock()

	assert.Equal(t, 1, obs.subscriptions[topic])
	assert.Equal(t, 1, obs.publishes[topic])
	assert.Equal(t, 1, obs.unsubscriptions[topic])
}

func TestObserver_Topic(t *testing.T) {
	obs := newMockObserver()
	topic := NewTopic(WithTopicObserver(obs))

	ch := topic.Subscribe()
	topic.Publish("msg1")
	topic.Unsubscribe(ch)

	obs.mu.Lock()
	defer obs.mu.Unlock()

	assert.Equal(t, 1, obs.subscriptions[""]) // Topic name is empty when created via NewTopic(obs)
	assert.Equal(t, 1, obs.publishes[""])
	assert.Equal(t, 1, obs.unsubscriptions[""])
}
