// Package otelpubsub provides an OpenTelemetry and Prometheus observer for the pubsub package.
package otelpubsub

import (
	"context"

	"github.com/dioad/pubsub"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

type otelObserver struct {
	tracer trace.Tracer
	meter  metric.Meter

	msgPublished metric.Int64Counter
	msgDropped   metric.Int64Counter
	subsActive   metric.Int64UpDownCounter

	// Prometheus counters (instance-specific)
	publishCounter     *prometheus.CounterVec
	subscribeCounter   *prometheus.CounterVec
	unsubscribeCounter *prometheus.CounterVec
}

// ObserverOpt is a functional option for configuring an Observer.
type ObserverOpt func(*observerConfig)

type observerConfig struct {
	serviceName        string
	prometheusRegistry prometheus.Registerer
}

// WithServiceName sets the service name for the Observer.
func WithServiceName(name string) ObserverOpt {
	return func(cfg *observerConfig) {
		cfg.serviceName = name
	}
}

// WithPrometheusRegistry sets a custom Prometheus registry for the Observer.
// If not provided, the default global registry (prometheus.DefaultRegisterer) will be used.
// Use this option when you need multiple independent observer instances with separate metrics.
func WithPrometheusRegistry(registry prometheus.Registerer) ObserverOpt {
	return func(cfg *observerConfig) {
		cfg.prometheusRegistry = registry
	}
}

// NewObserver creates a new pubsub.Observer that exports metrics to OpenTelemetry and Prometheus,
// and starts traces for message publications. It accepts variadic options for configuration.
//
// By default, Prometheus metrics are registered with the global registry (prometheus.DefaultRegisterer).
// To use multiple independent observers in the same process, provide separate registries using
// WithPrometheusRegistry option.
func NewObserver(opts ...ObserverOpt) pubsub.Observer {
	cfg := &observerConfig{
		serviceName:        "pubsub",
		prometheusRegistry: prometheus.DefaultRegisterer,
	}

	for _, opt := range opts {
		opt(cfg)
	}

	tracer := otel.Tracer(cfg.serviceName)
	meter := otel.Meter(cfg.serviceName)

	msgPublished, _ := meter.Int64Counter("pubsub.messages.published",
		metric.WithDescription("Total number of messages published"),
	)
	msgDropped, _ := meter.Int64Counter("pubsub.messages.dropped",
		metric.WithDescription("Total number of messages dropped"),
	)
	subsActive, _ := meter.Int64UpDownCounter("pubsub.subscriptions.active",
		metric.WithDescription("Number of active subscriptions"),
	)

	// Create instance-specific Prometheus counters
	publishCounter := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "pubsub_messages_published_total",
		Help: "Total number of messages published.",
	}, []string{"topic"})

	subscribeCounter := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "pubsub_subscriptions_total",
		Help: "Total number of active subscriptions.",
	}, []string{"topic"})

	unsubscribeCounter := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "pubsub_unsubscriptions_total",
		Help: "Total number of unsubscriptions.",
	}, []string{"topic"})

	// Register the counters with the provided registry
	cfg.prometheusRegistry.MustRegister(publishCounter)
	cfg.prometheusRegistry.MustRegister(subscribeCounter)
	cfg.prometheusRegistry.MustRegister(unsubscribeCounter)

	return &otelObserver{
		tracer: tracer,
		meter:  meter,

		msgPublished: msgPublished,
		msgDropped:   msgDropped,
		subsActive:   subsActive,

		publishCounter:     publishCounter,
		subscribeCounter:   subscribeCounter,
		unsubscribeCounter: unsubscribeCounter,
	}
}

func (o *otelObserver) OnPublish(topic string, msg any) {
	// Prometheus
	o.publishCounter.WithLabelValues(topic).Inc()

	// OpenTelemetry Metrics
	ctx := context.Background()
	o.msgPublished.Add(ctx, 1, metric.WithAttributes(attribute.String("topic", topic)))
}

func (o *otelObserver) OnDrop(topic string, msg any) {
	// OpenTelemetry Metrics
	o.msgDropped.Add(context.Background(), 1, metric.WithAttributes(attribute.String("topic", topic)))
}

func (o *otelObserver) OnSubscribe(topic string) {
	// Prometheus
	o.subscribeCounter.WithLabelValues(topic).Inc()

	// OpenTelemetry Metrics
	o.subsActive.Add(context.Background(), 1, metric.WithAttributes(attribute.String("topic", topic)))
}

func (o *otelObserver) OnUnsubscribe(topic string) {
	// Prometheus
	o.unsubscribeCounter.WithLabelValues(topic).Inc()

	// OpenTelemetry Metrics
	o.subsActive.Add(context.Background(), -1, metric.WithAttributes(attribute.String("topic", topic)))
}
