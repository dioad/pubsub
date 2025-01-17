package pubsub

import (
	"slices"
	"testing"
	"time"
)

func TestPubSubAllWithNoHistory(t *testing.T) {
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

func TestPubSubAllWithHistory(t *testing.T) {
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
		case msg := <-ch:
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

func TestPubSubSubscribeFunc(t *testing.T) {
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
