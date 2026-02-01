package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/dioad/pubsub"
	"github.com/dioad/pubsub/otelpubsub"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

func initConn() func() {
	// Tracer provider
	traceExporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	if err != nil {
		log.Fatal(err)
	}
	tp := trace.NewTracerProvider(
		trace.WithSampler(trace.AlwaysSample()),
		trace.WithBatcher(traceExporter),
		trace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String("pubsub-example"),
		)),
	)
	otel.SetTracerProvider(tp)

	// Meter provider
	metricExporter, err := stdoutmetric.New(stdoutmetric.WithPrettyPrint())
	if err != nil {
		log.Fatal(err)
	}
	mp := metric.NewMeterProvider(
		metric.WithReader(metric.NewPeriodicReader(metricExporter)),
		metric.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String("pubsub-example"),
		)),
	)
	otel.SetMeterProvider(mp)

	return func() {
		tp.Shutdown(context.Background())
		mp.Shutdown(context.Background())
	}
}

func main() {
	cleanup := initConn()
	defer cleanup()

	// Create the OpenTelemetry observer
	observer := otelpubsub.NewObserver(otelpubsub.WithServiceName("pubsub-service"))

	// Create PubSub with the observer
	ps := pubsub.NewPubSub(pubsub.WithObserver(observer))

	// Expose Prometheus metrics
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		fmt.Println("Prometheus metrics available at http://localhost:8080/metrics")
		if err := http.ListenAndServe(":8080", nil); err != nil {
			log.Fatal(err)
		}
	}()

	// Demonstrate PubSub usage
	topic := "events"
	ch := ps.Subscribe(topic)

	go func() {
		for msg := range ch {
			fmt.Printf("Subscriber received: %v\n", msg)
		}
	}()

	fmt.Println("Publishing messages...")
	for i := 1; i <= 5; i++ {
		ps.Publish(topic, fmt.Sprintf("message %d", i))
		time.Sleep(500 * time.Millisecond)
	}

	fmt.Println("Unsubscribing...")
	ps.Unsubscribe(topic, ch)

	// Give time for metrics to be exported to stdout
	time.Sleep(2 * time.Second)
	fmt.Println("Example finished.")
}
