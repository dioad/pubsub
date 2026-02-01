package otelpubsub

import (
	"testing"

	"github.com/dioad/pubsub"
	"github.com/prometheus/client_golang/prometheus"
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

func TestMultipleObserversWithSeparateRegistries(t *testing.T) {
	// Create two separate registries
	registry1 := prometheus.NewRegistry()
	registry2 := prometheus.NewRegistry()

	// Create two observers with different registries
	observer1 := NewObserver(
		WithServiceName("test-service-1"),
		WithPrometheusRegistry(registry1),
	)
	observer2 := NewObserver(
		WithServiceName("test-service-2"),
		WithPrometheusRegistry(registry2),
	)

	require.NotNil(t, observer1)
	require.NotNil(t, observer2)

	// Both observers should work independently without conflicts
	assert.NotPanics(t, func() {
		observer1.OnPublish("topic1", "message1")
		observer1.OnSubscribe("topic1")

		observer2.OnPublish("topic2", "message2")
		observer2.OnSubscribe("topic2")
	})

	// Verify metrics are registered in their respective registries
	metrics1, err := registry1.Gather()
	require.NoError(t, err)
	assert.NotEmpty(t, metrics1, "registry1 should have metrics")

	metrics2, err := registry2.Gather()
	require.NoError(t, err)
	assert.NotEmpty(t, metrics2, "registry2 should have metrics")
}

func TestMultipleObserversWithDefaultRegistry(t *testing.T) {
	// Create two observers using the default registry
	// They should share the same Prometheus metrics without panicking
	observer1 := NewObserver(WithServiceName("test-service-1"))
	observer2 := NewObserver(WithServiceName("test-service-2"))

	require.NotNil(t, observer1)
	require.NotNil(t, observer2)

	// Both observers should work without panicking, sharing the same metrics
	assert.NotPanics(t, func() {
		observer1.OnPublish("topic1", "message1")
		observer1.OnSubscribe("topic1")

		observer2.OnPublish("topic2", "message2")
		observer2.OnSubscribe("topic2")
	})
}
