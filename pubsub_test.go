package pubsub

import (
	"slices"
	"testing"
	"time"
)

func TestPub_SubscribeAll_WithNoHistory(t *testing.T) {
	ps := NewPubSub()

	ps.Publish("topic1", "msg1")
	ps.Publish("topic2", "msg2")

	ch1 := ps.SubscribeAll()
	ps.Publish("topic2", "msg3")

	receivedMessages := readAllFromChannel(ch1, 5*time.Millisecond)

	expectedMessages := []interface{}{"msg3"}

	if !orderedListsAreEqual(receivedMessages, expectedMessages) {
		t.Errorf("expected %v, got %v", expectedMessages, receivedMessages)
	}
}

func orderedListsAreEqual(list1 []interface{}, list2 []interface{}) bool {
	if len(list1) != len(list2) {
		return false
	}

	for i, item1 := range list1 {
		if item1 != list2[i] {
			return false
		}
	}

	return true
}

func unorderedListsAreEqual(list1 []interface{}, list2 []interface{}) bool {
	if len(list1) != len(list2) {
		return false
	}

	for _, item1 := range list1 {
		if !slices.Contains(list2, item1) {
			return false
		}
	}

	return true
}

func TestPubSub_SubscribeAll_WithHistory(t *testing.T) {
	ps := NewPubSub(WithHistorySize(10))

	ps.Publish("topic1", "msg1")
	ps.Publish("topic2", "msg2")

	ch1 := ps.SubscribeAll()
	ps.Publish("topic2", "msg3")

	receivedMessages := readAllFromChannel(ch1, 5*time.Millisecond)

	expectedMessages := []interface{}{"msg1", "msg2", "msg3"}

	if !orderedListsAreEqual(receivedMessages, expectedMessages) {
		t.Errorf("expected %v, got %v", expectedMessages, receivedMessages)
	}
}

func readAllFromChannel[T any](ch <-chan T, timePeriod time.Duration) []T {
	var messages []T
	for {
		select {
		case <-time.After(timePeriod):
			return messages
		case msg, open := <-ch:
			if !open {
				return messages
			}
			messages = append(messages, msg)

		}
	}
}

func TestPubSub(t *testing.T) {
	ps := NewPubSub(WithHistorySize(10))

	ch1 := ps.Subscribe("topic1")
	ch2 := ps.Subscribe("topic2")
	ch4 := ps.SubscribeAll()

	ps.Publish("topic1", "msg1")
	ps.Publish("topic2", "msg2")

	msg11 := <-ch1
	msg21 := <-ch2

	receivedMessages := readAllFromChannel(ch4, 5*time.Millisecond)

	expectedMessages := []interface{}{"msg1", "msg2"}

	if msg11 != "msg1" {
		t.Errorf("expected msg11, got %v", msg11)
	}

	if msg21 != "msg2" {
		t.Errorf("expected msg21, got %v", msg21)
	}

	if !unorderedListsAreEqual(receivedMessages, expectedMessages) {
		t.Errorf("expected %v, got %v", expectedMessages, receivedMessages)
	}
}

func TestPubSub_SubscribeFunc(t *testing.T) {
	ps := NewPubSub(WithHistorySize(10))

	ch1 := make(chan interface{})
	ch2 := make(chan interface{})

	ps.SubscribeFunc("topic1", func(msg interface{}) {
		ch1 <- msg
	})
	ps.SubscribeFunc("topic2", func(msg interface{}) {
		ch2 <- msg
	})

	ps.Publish("topic1", "msg1")
	ps.Publish("topic2", "msg2")

	msg11 := <-ch1
	msg21 := <-ch2

	if msg11 != "msg1" {
		t.Errorf("expected msg11, got %v", msg11)
	}

	if msg21 != "msg2" {
		t.Errorf("expected msg21, got %v", msg21)
	}
}

func TestPubSub_SubscribeAllFunc(t *testing.T) {
	ps := NewPubSub()
	ch := make(chan interface{}, 10)

	ps.SubscribeAllFunc(func(msg interface{}) {
		ch <- msg
	})

	ps.Publish("topic1", "msg1")
	// Small sleep to ensure messages are not arriving exactly at the same time if that's an issue
	time.Sleep(5 * time.Millisecond)
	ps.Publish("topic2", "msg2")

	received := readAllFromChannel(ch, 50*time.Millisecond)
	expected := []interface{}{"msg1", "msg2"}

	if !unorderedListsAreEqual(received, expected) {
		t.Errorf("expected %v, got %v", expected, received)
	}
}

func TestPubSub_Topics(t *testing.T) {
	ps := NewPubSub()
	ps.Publish("topic1", "msg1")
	ps.Publish("topic2", "msg2")
	ps.Subscribe("topic3")

	topics := ps.Topics()
	expected := []string{"topic1", "topic2", "topic3", "*"}

	if len(topics) != len(expected) {
		t.Fatalf("expected %d topics, got %d: %v", len(expected), len(topics), topics)
	}

	for _, e := range expected {
		found := false
		for _, tt := range topics {
			if e == tt {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected topic %s not found in %v", e, topics)
		}
	}
}

func TestPubSub_Unsubscribe(t *testing.T) {
	ps := NewPubSub()
	ch := ps.Subscribe("topic1")

	ps.Publish("topic1", "msg1")
	msg := <-ch
	if msg != "msg1" {
		t.Errorf("expected msg1, got %v", msg)
	}

	ps.Unsubscribe("topic1", ch)

	// Channel should be closed
	select {
	case _, ok := <-ch:
		if ok {
			t.Errorf("expected channel to be closed")
		}
	case <-time.After(100 * time.Millisecond):
		t.Errorf("timeout waiting for channel to close")
	}

	// Publishing after unsubscribe should not panic
	ps.Publish("topic1", "msg2")
}

func TestPubSub_UnsubscribeAll(t *testing.T) {
	ps := NewPubSub()
	ch := ps.SubscribeAll()

	ps.Publish("topic1", "msg1")
	msg := <-ch
	if msg != "msg1" {
		t.Errorf("expected msg1, got %v", msg)
	}

	ps.UnsubscribeAll(ch)

	// Channel should be closed
	select {
	case _, ok := <-ch:
		if ok {
			t.Errorf("expected channel to be closed")
		}
	case <-time.After(100 * time.Millisecond):
		t.Errorf("timeout waiting for channel to close")
	}
}

type mockFeeder struct {
	ch chan *EventTuple
}

func (m *mockFeeder) Feed() <-chan *EventTuple {
	return m.ch
}

func TestPubSub_AddFeeder(t *testing.T) {
	ps := NewPubSub()
	ch := ps.Subscribe("topic1")

	feeder := &mockFeeder{ch: make(chan *EventTuple, 1)}
	ps.AddFeeder(feeder)

	feeder.ch <- &EventTuple{Topic: "topic1", Event: "msg1"}

	select {
	case msg := <-ch:
		if msg != "msg1" {
			t.Errorf("expected msg1, got %v", msg)
		}
	case <-time.After(100 * time.Millisecond):
		t.Errorf("timeout waiting for message")
	}

	close(feeder.ch)
}

func TestPubSub_AddFeedingFunc(t *testing.T) {
	ps := NewPubSub()
	ch := ps.Subscribe("topic1")

	ps.AddFeedingFunc(nil)

	feedCh := make(chan *EventTuple, 1)
	ps.AddFeedingFunc(func() <-chan *EventTuple {
		return feedCh
	})

	feedCh <- &EventTuple{Topic: "topic1", Event: "msg1"}

	select {
	case msg := <-ch:
		if msg != "msg1" {
			t.Errorf("expected msg1, got %v", msg)
		}
	case <-time.After(100 * time.Millisecond):
		t.Errorf("timeout waiting for message")
	}

	// Test with nil channel from feeding func
	ps.AddFeedingFunc(func() <-chan *EventTuple {
		return nil
	})

	// Test with nil EventTuple
	feedCh <- nil

	close(feedCh)
}

func TestPubSub_AddFeedingFunc_Multiple(t *testing.T) {
	ps := NewPubSub()
	ch := ps.Subscribe("topic1")

	feedCh1 := make(chan *EventTuple, 1)
	feedCh2 := make(chan *EventTuple, 1)

	ps.AddFeedingFunc(func() <-chan *EventTuple { return feedCh1 })
	ps.AddFeedingFunc(func() <-chan *EventTuple { return feedCh2 })

	feedCh1 <- &EventTuple{Topic: "topic1", Event: "msg1"}
	feedCh2 <- &EventTuple{Topic: "topic1", Event: "msg2"}

	received := readAllFromChannel(ch, 50*time.Millisecond)
	expected := []interface{}{"msg1", "msg2"}

	if !unorderedListsAreEqual(received, expected) {
		t.Errorf("expected %v, got %v", expected, received)
	}
}
