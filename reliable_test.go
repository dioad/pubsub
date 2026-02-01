package pubsub

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestReliableDelivery(t *testing.T) {
	ps := NewPubSub()
	topic := ps.Topic("reliable")

	// Subscriber with unbuffered channel
	ch := topic.SubscribeUnbuffered()

	received := make(chan any, 1)
	go func() {
		msg := <-ch
		received <- msg
	}()

	// Give the goroutine a moment to start and block on channel read
	time.Sleep(10 * time.Millisecond)

	n := ps.PublishReliable("reliable", "test-msg")
	assert.GreaterOrEqual(t, n, 1)

	select {
	case msg := <-received:
		assert.Equal(t, "test-msg", msg)
	case <-time.After(500 * time.Millisecond):
		t.Error("timeout waiting for reliable delivery")
	}
}

func TestReliableDeliveryTimeout(t *testing.T) {
	ps := NewPubSub()
	topic := ps.Topic("reliable-timeout")

	// Subscriber with unbuffered channel that is NOT reading
	_ = topic.SubscribeUnbuffered()

	start := time.Now()
	n := ps.PublishReliable("reliable-timeout", "test-msg")
	duration := time.Since(start)

	assert.Equal(t, 0, n)
	assert.GreaterOrEqual(t, duration, 100*time.Millisecond)
}

func TestAddFeederReliable(t *testing.T) {
	ps := NewPubSub()
	ch := ps.SubscribeUnbuffered("test")

	feederCh := make(chan *EventTuple)
	feeder := &mockFeeder{ch: feederCh}
	ps.AddFeederReliable(context.Background(), feeder)

	received := make(chan any, 1)
	go func() {
		msg := <-ch
		received <- msg
	}()

	time.Sleep(10 * time.Millisecond)
	feederCh <- &EventTuple{Topic: "test", Event: "feeder-msg"}

	select {
	case msg := <-received:
		assert.Equal(t, "feeder-msg", msg)
	case <-time.After(500 * time.Millisecond):
		t.Error("timeout waiting for feeder reliable delivery")
	}

	close(feederCh)
}

func TestTopicWithHistoryReliable(t *testing.T) {
	ps := NewPubSub(WithHistorySize(5))
	topic := ps.Topic("history-reliable")

	n := ps.PublishReliable("history-reliable", "msg1", "msg2")
	assert.Equal(t, 0, n)

	// New subscriber should receive history
	ch := topic.Subscribe()
	received := readAllFromChannel(ch, 50*time.Millisecond)
	assert.Len(t, received, 2)

	// Test SubscribeUnbuffered on topic with history
	ch2 := topic.SubscribeUnbuffered()
	assert.Equal(t, 0, cap(ch2))

	// Test PublishReliable on topicWithHistory with a subscriber
	ch3 := topic.SubscribeUnbuffered()
	received3 := make(chan any, 1)
	go func() {
		msg := <-ch3
		received3 <- msg
	}()
	time.Sleep(10 * time.Millisecond)
	n2 := topic.PublishReliable("msg3")
	assert.GreaterOrEqual(t, n2, 1)
}

func TestSubscribeAllUnbuffered(t *testing.T) {
	ps := NewPubSub()
	ch := ps.SubscribeAllUnbuffered()

	received := make(chan any, 1)
	go func() {
		msg := <-ch
		received <- msg
	}()

	time.Sleep(10 * time.Millisecond)
	ps.PublishReliable("any-topic", "all-msg")

	select {
	case msg := <-received:
		assert.Equal(t, "all-msg", msg)
	case <-time.After(500 * time.Millisecond):
		t.Error("timeout waiting for SubscribeAllUnbuffered")
	}
}

func TestAddFeedingFuncReliable_Nil(t *testing.T) {
	ps := NewPubSub()
	ps.AddFeedingFuncReliable(context.Background(), nil)

	// Should return nil channel
	ps.AddFeedingFuncReliable(context.Background(), func() <-chan *EventTuple {
		return nil
	})
}
