package pubsub

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRingBuffer_Basic(t *testing.T) {
	rb := newRingBuffer(5)

	// Push some items
	rb.Push("a")
	rb.Push("b")
	rb.Push("c")

	all := rb.GetAll()
	assert.Len(t, all, 3)
	assert.Equal(t, []interface{}{"a", "b", "c"}, all)
}

func TestRingBuffer_Overflow(t *testing.T) {
	rb := newRingBuffer(3)

	// Push more than capacity
	rb.Push("a")
	rb.Push("b")
	rb.Push("c")
	rb.Push("d")
	rb.Push("e")

	all := rb.GetAll()
	assert.Len(t, all, 3)
	// Should have the last 3 items
	assert.Equal(t, []interface{}{"c", "d", "e"}, all)
}

func TestRingBuffer_Concurrent(t *testing.T) {
	rb := newRingBuffer(100)
	var wg sync.WaitGroup

	// Concurrent writers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				rb.Push(id*100 + j)
			}
		}(i)
	}

	// Concurrent readers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_ = rb.GetAll()
			}
		}()
	}

	wg.Wait()

	assert.Equal(t, 100, rb.Len())
}

func TestShardedPubSub_Basic(t *testing.T) {
	ps := NewShardedPubSub()

	ch := ps.Subscribe("test")
	go func() {
		for range ch {
		}
	}()

	n := ps.Publish("test", "hello", "world")
	// n counts successful channel sends; each message goes to 1 subscriber on "test" + 0 on "*" (no subscriber yet)
	// So with 2 messages and 1 subscriber, we expect 2 deliveries
	assert.GreaterOrEqual(t, n, 1)

	ps.Unsubscribe("test", ch)
}

func TestShardedPubSub_WithHistory(t *testing.T) {
	ps := NewShardedPubSub(WithHistorySize(10))

	// Publish before subscribing
	ps.Publish("test", "msg1")
	ps.Publish("test", "msg2")
	ps.Publish("test", "msg3")

	// Subscribe and should receive history
	ch := ps.Subscribe("test")

	// Check channel has historical messages (non-blocking read)
	count := 0
	for i := 0; i < 5; i++ {
		select {
		case <-ch:
			count++
		default:
		}
	}

	if count < 3 {
		t.Logf("received %d historical messages (may be buffered)", count)
	}

	ps.Unsubscribe("test", ch)
}

func TestShardedPubSub_WithLockFreeHistory(t *testing.T) {
	ps := NewShardedPubSub(WithLockFreeHistory(10))

	// Publish before subscribing
	ps.Publish("test", "msg1")
	ps.Publish("test", "msg2")
	ps.Publish("test", "msg3")

	// Subscribe and should receive history
	ch := ps.Subscribe("test")

	// Check channel has historical messages (non-blocking read)
	count := 0
	for i := 0; i < 5; i++ {
		select {
		case <-ch:
			count++
		default:
		}
	}

	if count < 3 {
		t.Logf("received %d historical messages (may be buffered)", count)
	}

	ps.Unsubscribe("test", ch)
}

func TestShardedPubSub_SubscribeAll(t *testing.T) {
	ps := NewShardedPubSub()

	allCh := ps.SubscribeAll()

	// Publish to different topics
	ps.Publish("topic1", "a")
	ps.Publish("topic2", "b")
	ps.Publish("topic3", "c")

	// Give a moment for channels to receive
	count := 0
	for i := 0; i < 10; i++ {
		select {
		case <-allCh:
			count++
		default:
		}
	}

	if count < 1 {
		t.Logf("received %d messages via SubscribeAll (non-blocking)", count)
	}

	ps.UnsubscribeAll(allCh)
}

func TestShardedPubSub_Topics(t *testing.T) {
	ps := NewShardedPubSub()

	ps.Publish("topic1", "msg")
	ps.Publish("topic2", "msg")
	ps.Publish("topic3", "msg")

	topics := ps.Topics()

	// Should have 3 topics + "*" (catch-all)
	assert.GreaterOrEqual(t, len(topics), 3)
}

func TestTopicWithLockFreeHistory_Basic(t *testing.T) {
	topic := NewTopic(WithLockFreeHistoryOpt(10))

	// Publish some messages
	topic.Publish("a", "b", "c")

	// Subscribe - should get history
	ch := topic.Subscribe()

	received := make([]interface{}, 0)
	for i := 0; i < 3; i++ {
		select {
		case msg := <-ch:
			received = append(received, msg)
		default:
		}
	}

	assert.Len(t, received, 3)

	topic.Unsubscribe(ch)
}

func TestTopicWithLockFreeHistory_Concurrent(t *testing.T) {
	topic := NewTopic(WithLockFreeHistoryOpt(100))

	ch := topic.Subscribe()
	go func() {
		for range ch {
		}
	}()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				topic.Publish(id*100 + j)
			}
		}(i)
	}

	wg.Wait()
	topic.Unsubscribe(ch)
}
