package pubsub

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTopic_Publish(t *testing.T) {
	topic := NewTopic()

	ch := topic.Subscribe()
	topic.Publish("msg1")

	msg := <-ch
	assert.Equal(t, "msg1", msg)
}

func TestTopic_SubscribeFunc(t *testing.T) {
	topic := NewTopic()

	ch := make(chan any)
	topic.SubscribeFunc(func(msg any) {
		ch <- msg
	})
	topic.Publish("msg1")

	msg := <-ch
	assert.Equal(t, "msg1", msg)
}

func TestTopic_SubscribeWithBuffer(t *testing.T) {
	topic := NewTopic()

	ch := topic.SubscribeWithBuffer(1)
	topic.Publish("msg1")

	msg := <-ch
	assert.Equal(t, "msg1", msg)
}

func TestTopic_Unsubscribe(t *testing.T) {
	topic := NewTopic()

	ch := topic.Subscribe()
	topic.Unsubscribe(ch)
	topic.Publish("msg1")

	select {
	case m, ok := <-ch:
		assert.False(t, ok, "expected channel to be closed, got %v", m)
	default:
	}
}

func TestTopicWithHistory_Publish(t *testing.T) {
	topic := NewTopic(WithHistory(10))

	ch := topic.Subscribe()
	topic.Publish("msg1", "msg2")

	msg1 := <-ch
	msg2 := <-ch
	assert.Equal(t, "msg1", msg1)
	assert.Equal(t, "msg2", msg2)
}

func TestTopicWithHistory_Subscribe(t *testing.T) {
	topic := NewTopic(WithHistory(10))

	ch := topic.Subscribe()
	topic.Publish("msg1", "msg2")

	msg1 := <-ch
	msg2 := <-ch
	assert.Equal(t, "msg1", msg1)
	assert.Equal(t, "msg2", msg2)
}

func TestTopicWithHistory_SubscribeFunc(t *testing.T) {
	topic := NewTopic(WithHistory(10))

	ch := make(chan any)
	topic.SubscribeFunc(func(msg any) {
		ch <- msg
	})
	topic.Publish("msg1", "msg2")

	msg1 := <-ch
	msg2 := <-ch
	assert.Equal(t, "msg1", msg1)
	assert.Equal(t, "msg2", msg2)
}

func TestTopicWithHistory_Unsubscribe(t *testing.T) {
	topic := NewTopic(WithHistory(10))

	ch := topic.Subscribe()
	topic.Unsubscribe(ch)
	topic.Publish("msg1", "msg2")

	select {
	case m, ok := <-ch:
		assert.False(t, ok, "expected channel to be closed, got %v", m)
	case <-time.After(100 * time.Millisecond):
		t.Errorf("timeout waiting for channel to close")
	}
}

func TestTopicWithHistory_SubscribeWithBuffer(t *testing.T) {
	topic := NewTopic(WithHistory(2))
	topic.Publish("msg1")
	topic.Publish("msg2")
	topic.Publish("msg3")

	// Wait a bit to ensure history is updated if there's any async (there isn't but for safety)
	time.Sleep(10 * time.Millisecond)

	// History only has "msg2", "msg3"
	ch := topic.SubscribeWithBuffer(10)

	received := readAllFromChannel(ch, 20*time.Millisecond)
	expected := []any{"msg2", "msg3"}

	assert.Equal(t, expected, received)
	assert.Equal(t, 10, cap(ch))
}
