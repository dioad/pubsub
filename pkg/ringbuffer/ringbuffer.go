// Package ringbuffer provides a lock-free ring buffer implementation
// for storing message history with concurrent read/write support.
package ringbuffer

import (
	"sync/atomic"
)

// RingBuffer is a lock-free ring buffer for storing message history.
// It uses atomic operations to allow concurrent reads and writes
// without mutex contention.
type RingBuffer struct {
	buffer   []atomic.Value
	size     int
	writePos atomic.Uint64
	count    atomic.Uint64
}

// New creates a new ring buffer with the specified capacity.
func New(size int) *RingBuffer {
	if size <= 0 {
		size = 1
	}
	rb := &RingBuffer{
		buffer: make([]atomic.Value, size),
		size:   size,
	}
	return rb
}

// Push adds a message to the ring buffer.
// This is safe to call concurrently with other Push and GetAll calls.
func (rb *RingBuffer) Push(msg any) {
	// Get the next write position atomically
	pos := rb.writePos.Add(1) - 1
	idx := int(pos % uint64(rb.size))

	// Store the message
	rb.buffer[idx].Store(msg)

	// Update count (capped at size)
	for {
		current := rb.count.Load()
		newCount := current + 1
		if newCount > uint64(rb.size) {
			newCount = uint64(rb.size)
		}
		if rb.count.CompareAndSwap(current, newCount) {
			break
		}
	}
}

// GetAll returns all messages currently in the buffer in order.
// This is safe to call concurrently with Push.
func (rb *RingBuffer) GetAll() []any {
	count := rb.count.Load()
	if count == 0 {
		return nil
	}

	writePos := rb.writePos.Load()
	result := make([]any, 0, count)

	// Calculate start position
	var startPos uint64
	if count < uint64(rb.size) {
		startPos = 0
	} else {
		startPos = writePos - uint64(rb.size)
	}

	// Read messages in order
	for i := uint64(0); i < count; i++ {
		idx := int((startPos + i) % uint64(rb.size))
		if val := rb.buffer[idx].Load(); val != nil {
			result = append(result, val)
		}
	}

	return result
}

// Len returns the current number of messages in the buffer.
func (rb *RingBuffer) Len() int {
	count := rb.count.Load()
	if count > uint64(rb.size) {
		return rb.size
	}
	return int(count)
}

// Cap returns the capacity of the ring buffer.
func (rb *RingBuffer) Cap() int {
	return rb.size
}
