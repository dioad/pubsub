package otelpubsub

import (
	"testing"

	"github.com/dioad/pubsub"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
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
		observer1.OnPublish("shared-topic", "message1")
		observer1.OnSubscribe("shared-topic")

		observer2.OnPublish("shared-topic", "message2")
		observer2.OnSubscribe("shared-topic")
	})

	// Verify that both observers contribute to the same metric series
	// by checking the default registry has the metrics and their values reflect both observers' activity
	metrics, err := prometheus.DefaultGatherer.Gather()
	require.NoError(t, err)

	// Find the publish counter metric
	var publishMetric *dto.MetricFamily
	for _, m := range metrics {
		if m.GetName() == "pubsub_messages_published_total" {
			publishMetric = m
			break
		}
	}
	require.NotNil(t, publishMetric, "pubsub_messages_published_total metric should exist")

	// Find the counter for "shared-topic" label
	var sharedTopicValue float64
	for _, metric := range publishMetric.GetMetric() {
		for _, label := range metric.GetLabel() {
			if label.GetName() == "topic" && label.GetValue() == "shared-topic" {
				sharedTopicValue = metric.GetCounter().GetValue()
				break
			}
		}
	}

	// Both observers published to "shared-topic", so the counter should be at least 2
	// This verifies that the AlreadyRegisteredError path correctly reused the existing collector
	assert.GreaterOrEqual(t, sharedTopicValue, float64(2), "shared-topic counter should reflect contributions from both observers")
}
