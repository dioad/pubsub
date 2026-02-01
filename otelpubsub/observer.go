package otelpubsub

import (
	"context"

	"github.com/dioad/pubsub"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

var (
	publishCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "pubsub_messages_published_total",
		Help: "Total number of messages published.",
	}, []string{"topic"})

	subscribeCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "pubsub_subscriptions_total",
		Help: "Total number of active subscriptions.",
	}, []string{"topic"})

	unsubscribeCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "pubsub_unsubscriptions_total",
		Help: "Total number of unsubscriptions.",
	}, []string{"topic"})
)

type otelObserver struct {
	tracer trace.Tracer
	meter  metric.Meter

	msgPublished metric.Int64Counter
	subsActive   metric.Int64UpDownCounter
}

// ObserverOpt is a functional option for configuring an Observer.
type ObserverOpt func(*observerConfig)

type observerConfig struct {
	serviceName string
}

// WithServiceName sets the service name for the Observer.
func WithServiceName(name string) ObserverOpt {
	return func(cfg *observerConfig) {
		cfg.serviceName = name
	}
}

// NewObserver creates a new pubsub.Observer that exports metrics to OpenTelemetry and Prometheus,
// and starts traces for message publications. It accepts variadic options for configuration.
func NewObserver(opts ...ObserverOpt) pubsub.Observer {
	cfg := &observerConfig{
		serviceName: "pubsub",
	}

	for _, opt := range opts {
		opt(cfg)
	}

	tracer := otel.Tracer(cfg.serviceName)
	meter := otel.Meter(cfg.serviceName)

	msgPublished, _ := meter.Int64Counter("pubsub.messages.published",
		metric.WithDescription("Total number of messages published"),
	)
	subsActive, _ := meter.Int64UpDownCounter("pubsub.subscriptions.active",
		metric.WithDescription("Number of active subscriptions"),
	)

	return &otelObserver{
		tracer: tracer,
		meter:  meter,

		msgPublished: msgPublished,
		subsActive:   subsActive,
	}
}

func (o *otelObserver) OnPublish(topic string, msg interface{}) {
	// Prometheus
	publishCounter.WithLabelValues(topic).Inc()

	// OpenTelemetry Metrics
	ctx := context.Background()
	o.msgPublished.Add(ctx, 1, metric.WithAttributes(attribute.String("topic", topic)))

	// OpenTelemetry Tracing
	_, span := o.tracer.Start(ctx, "pubsub.publish",
		trace.WithAttributes(attribute.String("topic", topic)),
	)
	defer span.End()
}

func (o *otelObserver) OnSubscribe(topic string) {
	// Prometheus
	subscribeCounter.WithLabelValues(topic).Inc()

	// OpenTelemetry Metrics
	o.subsActive.Add(context.Background(), 1, metric.WithAttributes(attribute.String("topic", topic)))
}

func (o *otelObserver) OnUnsubscribe(topic string) {
	// Prometheus
	unsubscribeCounter.WithLabelValues(topic).Inc()

	// OpenTelemetry Metrics
	o.subsActive.Add(context.Background(), -1, metric.WithAttributes(attribute.String("topic", topic)))
}
