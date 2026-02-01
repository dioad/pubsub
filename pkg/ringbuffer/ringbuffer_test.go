package ringbuffer

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRingBuffer_Basic(t *testing.T) {
	size := 3
	rb := New(size)

	assert.Equal(t, size, rb.Cap(), "expected cap %d, got %d", size, rb.Cap())
	assert.Zero(t, rb.Len(), "expected len 0, got %d", rb.Len())

	rb.Push("msg1")
	rb.Push("msg2")

	assert.Equal(t, 2, rb.Len(), "expected len 2, got %d", rb.Len())

	msgs := rb.GetAll()
	require.Len(t, msgs, 2, "expected 2 messages, got %d", len(msgs))
	assert.Equal(t, []any{"msg1", "msg2"}, msgs, "unexpected messages: %v", msgs)

	rb.Push("msg3")
	rb.Push("msg4") // Should wrap around, msg1 should be gone

	assert.Equal(t, 3, rb.Len(), "expected len 3, got %d", rb.Len())

	msgs = rb.GetAll()
	require.Len(t, msgs, 3, "expected 3 messages, got %d", len(msgs))
	assert.Equal(t, []any{"msg2", "msg3", "msg4"}, msgs, "unexpected messages after wrap: %v", msgs)
}

func TestRingBuffer_InvalidSize(t *testing.T) {
	rb := New(0)
	assert.Equal(t, 1, rb.Cap(), "expected cap 1 for size 0, got %d", rb.Cap())

	rb = New(-5)
	assert.Equal(t, 1, rb.Cap(), "expected cap 1 for size -5, got %d", rb.Cap())
}

func TestRingBuffer_Concurrent(t *testing.T) {
	size := 100
	rb := New(size)
	numGoroutines := 10
	numIterations := 1000

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numIterations; j++ {
				rb.Push(fmt.Sprintf("g%d-i%d", id, j))
				_ = rb.GetAll()
			}
		}(i)
	}

	wg.Wait()

	assert.Equal(t, size, rb.Len(), "expected len %d, got %d", size, rb.Len())
}
