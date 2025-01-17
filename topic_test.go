package pubsub

import (
	"testing"
)

func TestTopic_Publish(t *testing.T) {
	topic := NewTopic()

	ch := topic.Subscribe()
	topic.Publish("msg1")

	msg := <-ch
	if msg != "msg1" {
		t.Errorf("expected %v, got %v", "msg1", msg)
	}
}

func TestTopic_SubscribeFunc(t *testing.T) {
	topic := NewTopic()

	ch := make(chan interface{})
	topic.SubscribeFunc(func(msg interface{}) {
		ch <- msg
	})
	topic.Publish("msg1")

	msg := <-ch
	if msg != "msg1" {
		t.Errorf("expected %v, got %v", "msg1", msg)
	}
}

func TestTopic_SubscribeWithBuffer(t *testing.T) {
	topic := NewTopic()

	ch := topic.SubscribeWithBuffer(1)
	topic.Publish("msg1")

	msg := <-ch
	if msg != "msg1" {
		t.Errorf("expected %v, got %v", "msg1", msg)
	}
}

func TestTopic_Unsubscribe(t *testing.T) {
	topic := NewTopic()

	ch := topic.Subscribe()
	topic.Unsubscribe(ch)
	topic.Publish("msg1")

	select {
	case m, ok := <-ch:
		if ok {
			t.Errorf("expected channel to be closed, got %v", m)
		}
	default:
	}
}

func TestTopicWithHistory_Publish(t *testing.T) {
	topic := NewTopicWithHistory(10)

	ch := topic.Subscribe()
	topic.Publish("msg1")
	topic.Publish("msg2")

	msg1 := <-ch
	msg2 := <-ch
	if msg1 != "msg1" {
		t.Errorf("expected %v, got %v", "msg1", msg1)
	}
	if msg2 != "msg2" {
		t.Errorf("expected %v, got %v", "msg2", msg2)
	}
}

func TestTopicWithHistory_Subscribe(t *testing.T) {
	topic := NewTopicWithHistory(10)

	ch := topic.Subscribe()
	topic.Publish("msg1")
	topic.Publish("msg2")

	msg1 := <-ch
	msg2 := <-ch
	if msg1 != "msg1" {
		t.Errorf("expected %v, got %v", "msg1", msg1)
	}
	if msg2 != "msg2" {
		t.Errorf("expected %v, got %v", "msg2", msg2)
	}
}

func TestTopicWithHistory_SubscribeFunc(t *testing.T) {
	topic := NewTopicWithHistory(10)

	ch := make(chan interface{})
	topic.SubscribeFunc(func(msg interface{}) {
		ch <- msg
	})
	topic.Publish("msg1")
	topic.Publish("msg2")

	msg1 := <-ch
	msg2 := <-ch
	if msg1 != "msg1" {
		t.Errorf("expected %v, got %v", "msg1", msg1)
	}
	if msg2 != "msg2" {
		t.Errorf("expected %v, got %v", "msg2", msg2)
	}
}

func TestTopicWithHistory_Unsubscribe(t *testing.T) {
	topic := NewTopicWithHistory(10)

	ch := topic.Subscribe()
	topic.Unsubscribe(ch)
	topic.Publish("msg1")

	select {
	case m, ok := <-ch:
		if ok {
			t.Errorf("expected channel to be closed, got %v", m)
		}
	default:
	}
}
