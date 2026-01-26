package pubsub

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"sync"
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
		t.Fatalf("expected 2 messages, got %v", len(messages))
	}

	if messages[0].Field != "value1" {
		t.Errorf("expected %v, got %v", "value1", messages[0].Field)
	}

	if messages[1].Field != "value2" {
		t.Errorf("expected %v, got %v", "value2", messages[1].Field)
	}

	close(chOne)
	close(chTwo)

	select {
	case _, ok := <-mergedCh:
		if ok {
			t.Errorf("expected merged channel to be closed")
		}
	case <-time.After(100 * time.Millisecond):
		t.Errorf("timeout waiting for merged channel to close")
	}
}

func TestMerge_SkipMismatchedType(t *testing.T) {
	ch1 := make(chan interface{}, 2)
	merged := Merge[string](ch1)

	ch1 <- "msg1"
	ch1 <- 123 // Should be skipped

	received := readAllFromChannel(merged, 10*time.Millisecond)
	if len(received) != 1 || received[0] != "msg1" {
		t.Errorf("expected [msg1], got %v", received)
	}
}

func TestCastChan(t *testing.T) {
	ch := make(chan interface{}, 2)
	castCh := CastChan[testStructOne](ch)

	ch <- testStructOne{Field: "value1"}
	ch <- testStructTwo{Field: "value2"}
	close(ch)

	messages := readAllFromChannel[testStructOne](castCh, 50*time.Millisecond)

	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %v", len(messages))
	}

	if messages[0].Field != "value1" {
		t.Errorf("expected %v, got %v", "value1", messages[0].Field)
	}
}

// TestApplyChan_BasicFunctionality tests basic transformation functionality
func TestApplyChan_BasicFunctionality(t *testing.T) {
	input := make(chan int, 3)

	// Transform int to string
	output := ApplyChan(input, func(i int) (string, error) {
		return strconv.Itoa(i), nil
	})

	// Send test data
	input <- 1
	input <- 2
	input <- 3
	close(input)

	// Collect results
	results := readAllFromChannel(output, 10*time.Millisecond)

	expected := []string{"1", "2", "3"}
	if len(results) != len(expected) {
		t.Fatalf("expected %d results, got %d", len(expected), len(results))
	}

	for i, result := range results {
		if result != expected[i] {
			t.Errorf("expected %v, got %v", expected[i], result)
		}
	}
}

// TestApplyChan_ErrorHandling tests that errors are handled properly
func TestApplyChan_ErrorHandling(t *testing.T) {
	input := make(chan int, 5)

	// Transform function that errors on even numbers
	output := ApplyChan(input, func(i int) (string, error) {
		if i%2 == 0 {
			return "", errors.New("even number error")
		}
		return fmt.Sprintf("odd-%d", i), nil
	})

	// Send test data (mix of odd and even)
	input <- 1
	input <- 2 // should be skipped due to error
	input <- 3
	input <- 4 // should be skipped due to error
	input <- 5
	close(input)

	// Collect results
	results := readAllFromChannel(output, 10*time.Millisecond)

	expected := []string{"odd-1", "odd-3", "odd-5"}
	if len(results) != len(expected) {
		t.Fatalf("expected %d results, got %d", len(expected), len(results))
	}

	for i, result := range results {
		if result != expected[i] {
			t.Errorf("expected %v, got %v", expected[i], result)
		}
	}
}

// TestApplyChan_EmptyChannel tests behavior with empty input channel
func TestApplyChan_EmptyChannel(t *testing.T) {
	input := make(chan int)

	output := ApplyChan(input, func(i int) (string, error) {
		return strconv.Itoa(i), nil
	})

	// Close input immediately
	close(input)

	// Should get no results
	results := readAllFromChannel(output, 10*time.Millisecond)

	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

// TestApplyChan_ChannelCapacity tests that output channel has same capacity as input
func TestApplyChan_ChannelCapacity(t *testing.T) {
	input := make(chan int, 5)

	output := ApplyChan(input, func(i int) (string, error) {
		return strconv.Itoa(i), nil
	})

	// Check capacity
	if cap(output) != cap(input) {
		t.Errorf("expected output capacity %d, got %d", cap(input), cap(output))
	}

	close(input)
}

// TestApplyChan_UnbufferedChannel tests with unbuffered channels
func TestApplyChan_UnbufferedChannel(t *testing.T) {
	input := make(chan int)

	output := ApplyChan(input, func(i int) (string, error) {
		return strconv.Itoa(i), nil
	})

	// Send data in goroutine to avoid blocking
	go func() {
		input <- 42
		close(input)
	}()

	// Collect results
	results := readAllFromChannel(output, 10*time.Millisecond)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if results[0] != "42" {
		t.Errorf("expected '42', got %v", results[0])
	}
}

// TestApplyChan_RaceConditions tests for race conditions with concurrent access
func TestApplyChan_RaceConditions(t *testing.T) {
	const numGoroutines = 10
	const messagesPerGoroutine = 100

	input := make(chan int, numGoroutines*messagesPerGoroutine)

	output := ApplyChan(input, func(i int) (string, error) {
		// Simulate some work
		time.Sleep(time.Microsecond)
		return fmt.Sprintf("processed-%d", i), nil
	})

	var wg sync.WaitGroup

	// Start multiple goroutines sending data
	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for i := 0; i < messagesPerGoroutine; i++ {
				input <- goroutineID*messagesPerGoroutine + i
			}
		}(g)
	}

	// Close input after all goroutines finish
	go func() {
		wg.Wait()
		close(input)
	}()

	// Collect all results
	results := readAllFromChannel(output, 100*time.Millisecond)

	// Should receive all messages (no errors in this test)
	expectedCount := numGoroutines * messagesPerGoroutine
	if len(results) != expectedCount {
		t.Errorf("expected %d results, got %d", expectedCount, len(results))
	}

	// Verify all results have correct format
	for _, result := range results {
		resultStr := fmt.Sprintf("%s", result)
		if len(resultStr) < 10 || resultStr[:10] != "processed-" {
			t.Errorf("unexpected result format: %v", result)
			break
		}
	}
}

// TestApplyChan_FullOutputChannel tests behavior when output channel buffer is full
func TestApplyChan_FullOutputChannel(t *testing.T) {
	input := make(chan int, 10)

	// Use small buffer for output to test full channel behavior
	output := ApplyChan(input, func(i int) (string, error) {
		return strconv.Itoa(i), nil
	})

	// Fill input with more data than output buffer can hold
	for i := 0; i < 20; i++ {
		input <- i
	}
	close(input)

	// Don't read from output immediately to let it fill up
	time.Sleep(5 * time.Millisecond)

	// Now read results - some messages might be dropped due to full buffer
	results := readAllFromChannel(output, 20*time.Millisecond)

	// Should get some results, but possibly not all due to non-blocking send
	if len(results) == 0 {
		t.Error("expected some results, got none")
	}

	// All received results should be valid
	for _, result := range results {
		if _, err := strconv.Atoi(result); err != nil {
			t.Errorf("invalid result format: %v", result)
		}
	}
}

// TestApplyChan_TypeTransformation tests transformation between different types
func TestApplyChan_TypeTransformation(t *testing.T) {
	input := make(chan testStructOne, 3)

	// Transform struct to string
	output := ApplyChan(input, func(s testStructOne) (string, error) {
		return fmt.Sprintf("field:%s", s.Field), nil
	})

	// Send test data
	input <- testStructOne{Field: "test1"}
	input <- testStructOne{Field: "test2"}
	input <- testStructOne{Field: "test3"}
	close(input)

	// Collect results
	results := readAllFromChannel(output, 10*time.Millisecond)

	expected := []string{"field:test1", "field:test2", "field:test3"}
	if len(results) != len(expected) {
		t.Fatalf("expected %d results, got %d", len(expected), len(results))
	}

	for i, result := range results {
		if result != expected[i] {
			t.Errorf("expected %v, got %v", expected[i], result)
		}
	}
}
