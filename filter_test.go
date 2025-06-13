package pubsub

import (
	"reflect"
	"testing"
	"time"
)

type testStructOne struct {
	Field string
}

type testStructTwo struct {
	Field string
}

func TestSubscribe(t *testing.T) {
	topic := NewTopic()

	msgOne := testStructOne{Field: "value1"}
	msgTwo := testStructTwo{Field: "value2"}

	ch := Subscribe[testStructOne](topic)

	topic.Publish(msgOne, msgTwo)

	msg := <-ch
	if reflect.TypeOf(msg).Name() != "testStructOne" {
		t.Errorf("expected msg of type testStructOne, got %v", reflect.TypeOf(msg).Name())
	}

	if msg.Field != "value1" {
		t.Errorf("expected %v, got %v", "value1", msg.Field)
	}
}

func TestFilterChan(t *testing.T) {
	topic := NewTopic()

	msgOne := testStructOne{Field: "value1"}
	msgTwo := testStructOne{Field: "value2"}
	msgThree := testStructTwo{Field: "value3"}

	ch := topic.Subscribe()
	filteredCh := FilterChan(ch, func(s testStructOne) bool { return s.Field == "value1" })

	topic.Publish(msgOne, msgTwo, msgThree)

	messages := readAllFromChannel[testStructOne](filteredCh, 5*time.Millisecond)

	if len(messages) != 1 {
		t.Errorf("expected 1 message, got %v", len(messages))
	}

	msg := messages[0]
	if reflect.TypeOf(msg).Name() != "testStructOne" {
		t.Errorf("expected msg of type testStructOne, got %v", reflect.TypeOf(msg).Name())
	}

	if msg.Field != "value1" {
		t.Errorf("expected %v, got %v", "value1", msg.Field)
	}
}

func TestSubscribeWithFilter(t *testing.T) {
	topic := NewTopic()

	msgOne := testStructOne{Field: "value1"}
	msgTwo := testStructOne{Field: "value2"}
	msgThree := testStructTwo{Field: "value3"}

	ch := SubscribeWithFilter(topic, func(s testStructOne) bool { return s.Field == "value1" })

	topic.Publish(msgOne, msgTwo, msgThree)

	msg := <-ch
	if reflect.TypeOf(msg).Name() != "testStructOne" {
		t.Errorf("expected msg of type testStructOne, got %v", reflect.TypeOf(msg).Name())
	}

	if msg.Field != "value1" {
		t.Errorf("expected %v, got %v", "value1", msg.Field)
	}
}

func TestMerge(t *testing.T) {
	myMerge := Merge[testStructOne]

	msgOne := testStructOne{Field: "value1"}
	msgTwo := testStructOne{Field: "value2"}

	chOne := make(chan interface{})
	chTwo := make(chan interface{})
	mergedCh := myMerge(chOne, chTwo)

	chOne <- msgOne
	chTwo <- msgTwo

	messages := readAllFromChannel[testStructOne](mergedCh, 5*time.Millisecond)

	if len(messages) != 2 {
		t.Fatalf("expected 3 messages, got %v", len(messages))
	}

	if messages[0].Field != "value1" {
		t.Errorf("expected %v, got %v", "value1", messages[0].Field)
	}

	if messages[1].Field != "value2" {
		t.Errorf("expected %v, got %v", "value2", messages[1].Field)
	}
}

func TestCastChan(t *testing.T) {
	ch := make(chan interface{})
	castCh := CastChan[testStructOne](ch)

	go func() {
		ch <- testStructOne{Field: "value1"}
		ch <- testStructTwo{Field: "value2"}
		close(ch)
	}()

	messages := readAllFromChannel[testStructOne](castCh, 5*time.Millisecond)

	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %v", len(messages))
	}

	if messages[0].Field != "value1" {
		t.Errorf("expected %v, got %v", "value1", messages[0].Field)
	}
}
