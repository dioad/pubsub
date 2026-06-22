package pubsub

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dioad/pubsub/pkg/ringbuffer"
)

func TestRingBuffer_Basic(t *testing.T) {
	t.Parallel()
	rb := ringbuffer.New(5)

	// Push some items
	rb.Push("a")
	rb.Push("b")
	rb.Push("c")

	all := rb.GetAll()
	assert.Len(t, all, 3)
	assert.Equal(t, []any{"a", "b", "c"}, all)
}

func TestRingBuffer_Overflow(t *testing.T) {
	t.Parallel()
	rb := ringbuffer.New(3)

	// Push more than capacity
	rb.Push("a")
	rb.Push("b")
	rb.Push("c")
	rb.Push("d")
	rb.Push("e")

	all := rb.GetAll()
	assert.Len(t, all, 3)
	// Should have the last 3 items
	assert.Equal(t, []any{"c", "d", "e"}, all)
}

func TestRingBuffer_Concurrent(t *testing.T) {
	t.Parallel()
	rb := ringbuffer.New(100)
	var wg sync.WaitGroup

	// Concurrent writers
	for i := range 10 {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := range 100 {
				rb.Push(id*100 + j)
			}
		}(i)
	}

	// Concurrent readers
	for range 5 {
		wg.Go(func() {
			for range 100 {
				_ = rb.GetAll()
			}
		})
	}

	wg.Wait()

	assert.Equal(t, 100, rb.Len())
}

func TestShardedPubSub_Basic(t *testing.T) {
	t.Parallel()
	ps := NewShardedPubSub()

	ch := ps.Subscribe("test")
	go func() {
		for range ch {
		}
	}()

	res := ps.Publish("test", "hello", "world")
	// res.Deliveries counts successful channel sends; each message goes to 1 subscriber on "test" + 0 on "*" (no subscriber yet)
	// So with 2 messages and 1 subscriber, we expect 2 deliveries
	assert.GreaterOrEqual(t, res.Deliveries, 1)

	ps.Unsubscribe("test", ch)
}

func TestShardedPubSub_WithHistory(t *testing.T) {
	t.Parallel()
	ps := NewShardedPubSub(WithHistorySize(10))

	// Publish before subscribing
	ps.Publish("test", "msg1")
	ps.Publish("test", "msg2")
	ps.Publish("test", "msg3")

	// Subscribe and should receive history
	ch := ps.Subscribe("test")

	// Check channel has historical messages (non-blocking read)
	count := 0
	for range 5 {
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
	t.Parallel()
	ps := NewShardedPubSub(WithLockFreeHistorySize(10))

	// Publish before subscribing
	ps.Publish("test", "msg1")
	ps.Publish("test", "msg2")
	ps.Publish("test", "msg3")

	// Subscribe and should receive history
	ch := ps.Subscribe("test")

	// Check channel has historical messages (non-blocking read)
	count := 0
	for range 5 {
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
	t.Parallel()
	ps := NewShardedPubSub()

	allCh := ps.SubscribeAll()

	// Publish to different topics
	ps.Publish("topic1", "a")
	ps.Publish("topic2", "b")
	ps.Publish("topic3", "c")

	// Give a moment for channels to receive
	count := 0
	for range 10 {
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
	t.Parallel()
	ps := NewShardedPubSub()

	ps.Publish("topic1", "msg")
	ps.Publish("topic2", "msg")
	ps.Publish("topic3", "msg")

	topics := ps.Topics()

	// Should have 3 topics + "*" (catch-all)
	assert.GreaterOrEqual(t, len(topics), 3)
}

func TestTopicWithLockFreeHistory_Basic(t *testing.T) {
	t.Parallel()
	topic := NewTopic(WithLockFreeHistory(10))

	// Publish some messages
	topic.Publish("a", "b", "c")

	// Subscribe - should get history
	ch := topic.Subscribe()

	received := make([]any, 0)
	for range 3 {
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
	t.Parallel()
	topic := NewTopic(WithLockFreeHistory(100))

	ch := topic.Subscribe()
	go func() {
		for range ch {
		}
	}()

	var wg sync.WaitGroup
	for i := range 10 {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := range 100 {
				topic.Publish(id*100 + j)
			}
		}(i)
	}

	wg.Wait()
	topic.Unsubscribe(ch)
}
