# OpenTelemetry Observer Example

This example demonstrates how to use the OpenTelemetry and Prometheus observer with the PubSub library.

## Overview

The `otelpubsub` package provides an observer implementation that:
- Exports metrics to OpenTelemetry
- Exposes Prometheus metrics via an HTTP endpoint
- Supports multiple independent observer instances in the same process

## Running the Example

```bash
go run github.com/dioad/pubsub/examples/otel-observer
```

The example will:
1. Start a Prometheus metrics endpoint at `http://localhost:8080/metrics`
2. Publish several messages to a topic
3. Export OpenTelemetry metrics to stdout
4. Display subscriber messages

## Multiple Observer Instances

If you need to run multiple PubSub instances with separate metrics in the same process, provide a custom Prometheus registry:

```go
import (
    "github.com/dioad/pubsub"
    "github.com/dioad/pubsub/otelpubsub"
    "github.com/prometheus/client_golang/prometheus"
)

// Create separate registries for each observer
registry1 := prometheus.NewRegistry()
registry2 := prometheus.NewRegistry()

// Create observers with separate registries
observer1 := otelpubsub.NewObserver(
    otelpubsub.WithServiceName("service-1"),
    otelpubsub.WithPrometheusRegistry(registry1),
)
observer2 := otelpubsub.NewObserver(
    otelpubsub.WithServiceName("service-2"),
    otelpubsub.WithPrometheusRegistry(registry2),
)

// Create PubSub instances with different observers
ps1 := pubsub.NewPubSub(pubsub.WithObserver(observer1))
ps2 := pubsub.NewPubSub(pubsub.WithObserver(observer2))
```

## Metrics

The observer exports the following Prometheus metrics:

- `pubsub_messages_published_total{topic}` - Total number of messages published
- `pubsub_subscriptions_total{topic}` - Total number of subscriptions
- `pubsub_unsubscriptions_total{topic}` - Total number of unsubscriptions

And the following OpenTelemetry metrics:

- `pubsub.messages.published` - Total number of messages published
- `pubsub.messages.dropped` - Total number of messages dropped
- `pubsub.subscriptions.active` - Number of active subscriptions (up/down counter)
