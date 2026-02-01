package pubsub

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPub_SubscribeAll_WithNoHistory(t *testing.T) {
	ps := NewPubSub()

	ps.Publish("topic1", "msg1")
	ps.Publish("topic2", "msg2")

	ch1 := ps.SubscribeAll()
	ps.Publish("topic2", "msg3")

	receivedMessages := readAllFromChannel(ch1, 5*time.Millisecond)

	expectedMessages := []any{"msg3"}

	assert.Equal(t, expectedMessages, receivedMessages)
}

func TestPubSub_DeleteTopic(t *testing.T) {
	ps := NewPubSub()
	ch := ps.Subscribe("test")
	ps.Publish("test", "msg1")

	// Read msg1 to clear the channel
	<-ch

	ps.DeleteTopic("test")

	select {
	case _, ok := <-ch:
		assert.False(t, ok, "expected channel to be closed after DeleteTopic")
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for channel to close after DeleteTopic")
	}

	assert.NotContains(t, ps.Topics(), "test")
}

func TestPubSub_Drops(t *testing.T) {
	ps := NewPubSub()
	_ = ps.SubscribeWithBuffer("test", 0) // Unbuffered

	// We don't read from the channel, so Publish should drop
	res := ps.Publish("test", "msg1")
	assert.Equal(t, 1, res.Drops)
	assert.Equal(t, 0, res.Deliveries)
}

func TestPubSub_SubscribeAll_WithHistory(t *testing.T) {
	ps := NewPubSub(WithHistorySize(10))

	ps.Publish("topic1", "msg1")
	ps.Publish("topic2", "msg2")

	ch1 := ps.SubscribeAll()
	ps.Publish("topic2", "msg3")

	receivedMessages := readAllFromChannel(ch1, 5*time.Millisecond)

	expectedMessages := []any{"msg1", "msg2", "msg3"}

	assert.Equal(t, expectedMessages, receivedMessages)
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

	expectedMessages := []any{"msg1", "msg2"}

	assert.Equal(t, "msg1", msg11)
	assert.Equal(t, "msg2", msg21)
	assert.ElementsMatch(t, expectedMessages, receivedMessages)
}

func TestPubSub_SubscribeFunc(t *testing.T) {
	ps := NewPubSub(WithHistorySize(10))

	ch1 := make(chan any)
	ch2 := make(chan any)

	ps.SubscribeFunc("topic1", func(msg any) {
		ch1 <- msg
	})
	ps.SubscribeFunc("topic2", func(msg any) {
		ch2 <- msg
	})

	ps.Publish("topic1", "msg1")
	ps.Publish("topic2", "msg2")

	msg11 := <-ch1
	msg21 := <-ch2

	assert.Equal(t, "msg1", msg11)
	assert.Equal(t, "msg2", msg21)
}

func TestPubSub_SubscribeAllFunc(t *testing.T) {
	ps := NewPubSub()
	ch := make(chan any, 10)

	ps.SubscribeAllFunc(func(msg any) {
		ch <- msg
	})

	ps.Publish("topic1", "msg1")
	// Small sleep to ensure messages are not arriving exactly at the same time if that's an issue
	time.Sleep(5 * time.Millisecond)
	ps.Publish("topic2", "msg2")

	received := readAllFromChannel(ch, 50*time.Millisecond)
	expected := []any{"msg1", "msg2"}

	assert.ElementsMatch(t, expected, received)
}

func TestPubSub_Topics(t *testing.T) {
	ps := NewPubSub()
	ps.Publish("topic1", "msg1")
	ps.Publish("topic2", "msg2")
	ps.Subscribe("topic3")

	topics := ps.Topics()
	expected := []string{"topic1", "topic2", "topic3", "*"}

	assert.ElementsMatch(t, expected, topics)
}

func TestPubSub_Unsubscribe(t *testing.T) {
	ps := NewPubSub()
	ch := ps.Subscribe("topic1")

	ps.Publish("topic1", "msg1")
	msg := <-ch
	assert.Equal(t, "msg1", msg)

	ps.Unsubscribe("topic1", ch)

	// Channel should be closed
	select {
	case _, ok := <-ch:
		assert.False(t, ok, "expected channel to be closed")
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
	assert.Equal(t, "msg1", msg)

	ps.UnsubscribeAll(ch)

	// Channel should be closed
	select {
	case _, ok := <-ch:
		assert.False(t, ok, "expected channel to be closed")
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
	ps.AddFeeder(context.Background(), feeder)

	feeder.ch <- &EventTuple{Topic: "topic1", Event: "msg1"}

	select {
	case msg := <-ch:
		assert.Equal(t, "msg1", msg)
	case <-time.After(100 * time.Millisecond):
		t.Errorf("timeout waiting for message")
	}

	close(feeder.ch)
}

func TestPubSub_AddFeedingFunc(t *testing.T) {
	ps := NewPubSub()
	ch := ps.Subscribe("topic1")

	ps.AddFeedingFunc(context.Background(), nil)

	feedCh := make(chan *EventTuple, 1)
	ps.AddFeedingFunc(context.Background(), func() <-chan *EventTuple {
		return feedCh
	})

	feedCh <- &EventTuple{Topic: "topic1", Event: "msg1"}

	select {
	case msg := <-ch:
		assert.Equal(t, "msg1", msg)
	case <-time.After(100 * time.Millisecond):
		t.Errorf("timeout waiting for message")
	}

	// Test with nil channel from feeding func
	ps.AddFeedingFunc(context.Background(), func() <-chan *EventTuple {
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

	ps.AddFeedingFunc(context.Background(), func() <-chan *EventTuple { return feedCh1 })
	ps.AddFeedingFunc(context.Background(), func() <-chan *EventTuple { return feedCh2 })

	feedCh1 <- &EventTuple{Topic: "topic1", Event: "msg1"}
	feedCh2 <- &EventTuple{Topic: "topic1", Event: "msg2"}

	received := readAllFromChannel(ch, 50*time.Millisecond)
	expected := []any{"msg1", "msg2"}

	assert.ElementsMatch(t, expected, received)
}
