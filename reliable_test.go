package pubsub

import (
	"testing"
	"time"
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
	if n < 1 {
		t.Errorf("expected at least 1 delivery, got %d", n)
	}

	select {
	case msg := <-received:
		if msg != "test-msg" {
			t.Errorf("expected test-msg, got %v", msg)
		}
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

	if n != 0 {
		t.Errorf("expected 0 deliveries to non-reading subscriber, got %d", n)
	}

	if duration < 100*time.Millisecond {
		t.Errorf("expected at least 100ms timeout, got %v", duration)
	}
}

func TestAddFeederReliable(t *testing.T) {
	ps := NewPubSub()
	ch := ps.SubscribeUnbuffered("test")

	feederCh := make(chan *EventTuple)
	feeder := &mockFeeder{ch: feederCh}
	ps.AddFeederReliable(feeder)

	received := make(chan any, 1)
	go func() {
		msg := <-ch
		received <- msg
	}()

	time.Sleep(10 * time.Millisecond)
	feederCh <- &EventTuple{Topic: "test", Event: "feeder-msg"}

	select {
	case msg := <-received:
		if msg != "feeder-msg" {
			t.Errorf("expected feeder-msg, got %v", msg)
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("timeout waiting for feeder reliable delivery")
	}

	close(feederCh)
}

func TestTopicWithHistoryReliable(t *testing.T) {
	ps := NewPubSub(WithHistorySize(5))
	topic := ps.Topic("history-reliable")

	n := ps.PublishReliable("history-reliable", "msg1", "msg2")
	if n != 0 {
		t.Errorf("expected 0 deliveries, got %d", n)
	}

	// New subscriber should receive history
	ch := topic.Subscribe()
	received := readAllFromChannel(ch, 50*time.Millisecond)
	if len(received) != 2 {
		t.Errorf("expected 2 historical messages, got %d", len(received))
	}

	// Test SubscribeUnbuffered on topic with history
	ch2 := topic.SubscribeUnbuffered()
	if cap(ch2) != 0 {
		t.Errorf("expected unbuffered channel, got capacity %d", cap(ch2))
	}

	// Test PublishReliable on topicWithHistory with a subscriber
	ch3 := topic.SubscribeUnbuffered()
	received3 := make(chan any, 1)
	go func() {
		msg := <-ch3
		received3 <- msg
	}()
	time.Sleep(10 * time.Millisecond)
	n2 := topic.PublishReliable("msg3")
	if n2 < 1 {
		t.Errorf("expected at least 1 delivery, got %d", n2)
	}
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
		if msg != "all-msg" {
			t.Errorf("expected all-msg, got %v", msg)
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("timeout waiting for SubscribeAllUnbuffered")
	}
}

func TestAddFeedingFuncReliable_Nil(t *testing.T) {
	ps := NewPubSub()
	ps.AddFeedingFuncReliable(nil)

	// Should return nil channel
	ps.AddFeedingFuncReliable(func() <-chan *EventTuple {
		return nil
	})
}
