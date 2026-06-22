package pubsub

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTopic_Close(t *testing.T) {
	t.Parallel()
	topic := NewTopic()
	ch := topic.Subscribe()

	topic.Close()

	select {
	case _, ok := <-ch:
		assert.False(t, ok, "expected channel to be closed after topic.Close()")
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for channel to close after topic.Close()")
	}
}

func TestPubSub_Shutdown(t *testing.T) {
	t.Parallel()
	ps := NewPubSub()
	ch1 := ps.Subscribe("topic1")
	ch2 := ps.Subscribe("topic2")
	chAll := ps.SubscribeAll()

	ps.Shutdown(context.Background())

	channels := []<-chan any{ch1, ch2, chAll}
	for i, ch := range channels {
		select {
		case _, ok := <-ch:
			assert.Falsef(t, ok, "channel %d expected to be closed after ps.Shutdown()", i)
		case <-time.After(100 * time.Millisecond):
			t.Errorf("timeout waiting for channel %d to close after ps.Shutdown()", i)
		}
	}

	assert.Empty(t, ps.Topics())
}

func TestShardedPubSub_Shutdown(t *testing.T) {
	t.Parallel()
	ps := NewShardedPubSub()
	ch1 := ps.Subscribe("topic1")
	ch2 := ps.Subscribe("topic2")
	chAll := ps.SubscribeAll()

	ps.Shutdown(context.Background())

	channels := []<-chan any{ch1, ch2, chAll}
	for i, ch := range channels {
		select {
		case _, ok := <-ch:
			assert.Falsef(t, ok, "channel %d expected to be closed after sharded ps.Shutdown()", i)
		case <-time.After(100 * time.Millisecond):
			t.Errorf("timeout waiting for channel %d to close after sharded ps.Shutdown()", i)
		}
	}

	assert.Empty(t, ps.Topics())
}

func TestTopic_Shutdown_DoubleClose_NoPanic(t *testing.T) {
	t.Parallel()
	topic := NewTopic()
	topic.Subscribe()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled — Shutdown will see ctx.Done() immediately

	topic.Shutdown(ctx) // partial shutdown (context already done)
	topic.Close()       // must not panic with "close of closed channel"
}

func TestPubSub_Shutdown_DoubleClose_NoPanic(t *testing.T) {
	t.Parallel()
	ps := NewPubSub()
	ps.Subscribe("topic1")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	ps.Shutdown(ctx)
	ps.Close() // must not panic
}

func TestAddFeeder_ContextCancel(t *testing.T) {
	t.Parallel()
	ps := NewPubSub()
	ch := ps.Subscribe("test")

	ctx, cancel := context.WithCancel(context.Background())

	feederCh := make(chan *EventTuple, 1)
	ps.AddFeeder(ctx, &mockFeeder{ch: feederCh})

	feederCh <- &EventTuple{Topic: "test", Event: "msg1"}

	select {
	case msg := <-ch:
		assert.Equal(t, "msg1", msg)
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for message from feeder")
	}

	cancel()
	time.Sleep(10 * time.Millisecond) // Give goroutine time to exit

	feederCh <- &EventTuple{Topic: "test", Event: "msg2"}

	select {
	case msg := <-ch:
		t.Errorf("received message %v after feeder context was cancelled", msg)
	case <-time.After(100 * time.Millisecond):
		// Success: no message received
	}
}
