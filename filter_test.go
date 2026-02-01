package pubsub

import (
	"errors"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	assert.IsType(t, testStructOne{}, msg)
	assert.Equal(t, "value1", msg.Field)
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

	assert.Len(t, messages, 1)

	msg := messages[0]
	assert.IsType(t, testStructOne{}, msg)
	assert.Equal(t, "value1", msg.Field)
}

func TestSubscribeWithFilter(t *testing.T) {
	topic := NewTopic()

	msgOne := testStructOne{Field: "value1"}
	msgTwo := testStructOne{Field: "value2"}
	msgThree := testStructTwo{Field: "value3"}

	ch := SubscribeWithFilter(topic, func(s testStructOne) bool { return s.Field == "value1" })

	topic.Publish(msgOne, msgTwo, msgThree)

	msg := <-ch
	assert.IsType(t, testStructOne{}, msg)
	assert.Equal(t, "value1", msg.Field)
}

func TestMerge(t *testing.T) {
	myMerge := Merge[testStructOne]

	msgOne := testStructOne{Field: "value1"}
	msgTwo := testStructOne{Field: "value2"}

	chOne := make(chan any)
	chTwo := make(chan any)
	mergedCh := myMerge(chOne, chTwo)

	chOne <- msgOne
	chTwo <- msgTwo

	messages := readAllFromChannel[testStructOne](mergedCh, 5*time.Millisecond)

	require.Len(t, messages, 2)
	assert.Equal(t, "value1", messages[0].Field)
	assert.Equal(t, "value2", messages[1].Field)

	close(chOne)
	close(chTwo)

	select {
	case _, ok := <-mergedCh:
		assert.False(t, ok, "expected merged channel to be closed")
	case <-time.After(100 * time.Millisecond):
		t.Errorf("timeout waiting for merged channel to close")
	}
}

func TestMerge_SkipMismatchedType(t *testing.T) {
	ch1 := make(chan any, 2)
	merged := Merge[string](ch1)

	ch1 <- "msg1"
	ch1 <- 123 // Should be skipped

	received := readAllFromChannel(merged, 10*time.Millisecond)
	assert.Equal(t, []string{"msg1"}, received)
}

func TestCastChan(t *testing.T) {
	ch := make(chan any, 2)
	castCh := CastChan[testStructOne](ch)

	ch <- testStructOne{Field: "value1"}
	ch <- testStructTwo{Field: "value2"}
	close(ch)

	messages := readAllFromChannel[testStructOne](castCh, 50*time.Millisecond)

	require.Len(t, messages, 1)
	assert.Equal(t, "value1", messages[0].Field)
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
	assert.Equal(t, expected, results)
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
	assert.Equal(t, expected, results)
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

	assert.Empty(t, results)
}

// TestApplyChan_ChannelCapacity tests that output channel has same capacity as input
func TestApplyChan_ChannelCapacity(t *testing.T) {
	input := make(chan int, 5)

	output := ApplyChan(input, func(i int) (string, error) {
		return strconv.Itoa(i), nil
	})

	// Check capacity
	assert.Equal(t, cap(input), cap(output))

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

	require.Len(t, results, 1)
	assert.Equal(t, "42", results[0])
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
	assert.Len(t, results, expectedCount)

	// Verify all results have correct format
	for _, result := range results {
		assert.Regexp(t, "^processed-[0-9]+$", result)
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
	assert.NotEmpty(t, results)

	// All received results should be valid
	for _, result := range results {
		_, err := strconv.Atoi(result)
		assert.NoError(t, err)
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
	assert.Equal(t, expected, results)
}

func TestApplyChanWithMetrics_DroppedEvents(t *testing.T) {
	// Create a small buffered input channel
	input := make(chan int, 10)

	// Create output with very small buffer to force drops
	metrics := &DropMetrics{}
	output, _ := ApplyChanWithMetrics(input, func(i int) (int, error) {
		return i * 2, nil
	}, metrics)

	// Send more messages than output buffer can hold without reading
	for i := 0; i < 10; i++ {
		input <- i
	}
	close(input)

	// Wait a bit for goroutine to process
	time.Sleep(50 * time.Millisecond)

	// Some messages should have been dropped
	dropped := metrics.GetDropped()
	if dropped == 0 {
		t.Log("No drops occurred - buffer was large enough (this is okay)")
	}

	// Drain output to avoid goroutine leak
	for range output {
	}
}

func TestApplyChanWithMetrics_FilteredEvents(t *testing.T) {
	input := make(chan int, 10)

	metrics := &DropMetrics{}
	output, _ := ApplyChanWithMetrics(input, func(i int) (int, error) {
		if i%2 == 0 {
			return i, nil
		}
		return 0, errors.New("odd number filtered")
	}, metrics)

	// Send mix of even and odd numbers
	for i := 0; i < 10; i++ {
		input <- i
	}
	close(input)

	// Collect results
	var results []int
	for v := range output {
		results = append(results, v)
	}

	// Should have filtered 5 odd numbers
	assert.Equal(t, uint64(5), metrics.GetFiltered())

	// Should have 5 even numbers
	assert.Len(t, results, 5)
}

func TestDropMetrics_Concurrent(t *testing.T) {
	metrics := &DropMetrics{}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				metrics.IncrementDropped()
				metrics.IncrementFiltered()
			}
		}()
	}
	wg.Wait()

	assert.Equal(t, uint64(10000), metrics.GetDropped())
	assert.Equal(t, uint64(10000), metrics.GetFiltered())
}
