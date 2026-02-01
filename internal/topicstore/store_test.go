package topicstore

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

type mockTopic struct {
	closed bool
}

func (m *mockTopic) Close() {
	m.closed = true
}

func TestSimpleStore(t *testing.T) {
	s := NewSimpleStore()

	// GetOrCreate
	created := false
	topic := s.GetOrCreate("test", func() Topic {
		return &mockTopic{}
	}, func(t Topic) {
		created = true
	})

	if !created {
		assert.Fail(t, "expected onCreated to be called")
	}

	// Get existing
	created = false
	topic2 := s.GetOrCreate("test", func() Topic {
		return &mockTopic{}
	}, func(t Topic) {
		created = true
	})

	assert.False(t, created, "expected onCreated NOT to be called for existing topic")
	assert.Same(t, topic, topic2, "expected same topic instance")

	// Range
	count := 0
	s.Range(func(name string, topic Topic) bool {
		count++
		return true
	})
	assert.Equal(t, 1, count, "expected 1 topic, got %d", count)

	// Shutdown
	s.Shutdown(context.Background())
	assert.True(t, topic.(*mockTopic).closed, "expected topic to be closed")

	// Clear
	s.GetOrCreate("test2", func() Topic { return &mockTopic{} }, nil)
	s.Clear()
	count = 0
	s.Range(func(name string, topic Topic) bool {
		count++
		return true
	})
	assert.Zero(t, count, "expected 0 topics after clear, got %d", count)
}

func TestShardedStore(t *testing.T) {
	s := NewShardedStore()

	// GetOrCreate
	for i := 0; i < 100; i++ {
		name := fmt.Sprintf("topic-%d", i)
		s.GetOrCreate(name, func() Topic {
			return &mockTopic{}
		}, nil)
	}

	// Double check GetOrCreate (fast path and slow path with double check)
	s.GetOrCreate("topic-0", func() Topic {
		return &mockTopic{}
	}, nil)

	// Range
	count := 0
	s.Range(func(name string, topic Topic) bool {
		count++
		if count == 50 {
			return false // Test early exit
		}
		return true
	})
	assert.Equal(t, 50, count, "expected 50 topics after early exit, got %d", count)

	count = 0
	s.Range(func(name string, topic Topic) bool {
		count++
		return true
	})
	assert.Equal(t, 100, count, "expected 100 topics, got %d", count)

	// Shutdown
	s.Shutdown(context.Background())
	s.Range(func(name string, topic Topic) bool {
		assert.Truef(t, topic.(*mockTopic).closed, "topic %s not closed", name)
		return true
	})

	// Clear
	s.GetOrCreate("test", func() Topic { return &mockTopic{} }, nil)
	s.Clear()
	count = 0
	s.Range(func(name string, topic Topic) bool {
		count++
		return true
	})
	assert.Zero(t, count, "expected 0 topics after clear, got %d", count)
}

func TestSimpleStore_RangeExit(t *testing.T) {
	s := NewSimpleStore()
	s.GetOrCreate("t1", func() Topic { return &mockTopic{} }, nil)
	s.GetOrCreate("t2", func() Topic { return &mockTopic{} }, nil)

	count := 0
	s.Range(func(name string, topic Topic) bool {
		count++
		return false
	})
	assert.Equal(t, 1, count, "expected 1 topic after early exit, got %d", count)
}

func TestStore_ShutdownTimeout(t *testing.T) {
	s := NewSimpleStore()
	s.GetOrCreate("test", func() Topic { return &mockTopic{} }, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Already cancelled

	s.Shutdown(ctx)

	s2 := NewShardedStore()
	s2.GetOrCreate("test", func() Topic { return &mockTopic{} }, nil)
	s2.Shutdown(ctx)
}
