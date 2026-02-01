package otelpubsub

import (
	"testing"

	"github.com/dioad/pubsub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewObserver(t *testing.T) {
	observer := NewObserver(WithServiceName("test-service"))
	require.NotNil(t, observer, "expected observer to be non-nil")

	// Test calls don't panic
	assert.NotPanics(t, func() {
		observer.OnPublish("test-topic", "message")
		observer.OnDrop("test-topic", "message")
		observer.OnSubscribe("test-topic")
		observer.OnUnsubscribe("test-topic")
	})
}

func TestObserverImplementation(t *testing.T) {
	var _ pubsub.Observer = (*otelObserver)(nil)
}
