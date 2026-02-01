# PubSub

A lightweight, type-safe publish/subscribe library for Go applications.

## Features

- Simple, intuitive API for topic-based messaging
- Type-safe message filtering with generics
- Support for message history
- Channel merging capabilities
- Non-blocking and Reliable message delivery options
- Thread-safe operations
- Callback-based message handling
- Wildcard topic subscription

## Installation

```bash
go get github.com/dioad/pubsub
```

## Quick Start

```go
package main

import (
    "fmt"
    "time"
    
    "github.com/dioad/pubsub"
)

func main() {
    // Create a new PubSub instance
    ps := pubsub.NewPubSub()
    
    // Subscribe to a topic
    ch := ps.Subscribe("notifications")
    
    // Process messages in a goroutine
    go func() {
        for msg := range ch {
            fmt.Printf("Received: %v\n", msg)
        }
    }()
    
    // Publish messages to the topic
    ps.Publish("notifications", "Hello, World!")
    ps.Publish("notifications", "Another message")
    
    // Wait for messages to be processed
    time.Sleep(100 * time.Millisecond)
}
```

## Core Concepts

### Topics

A Topic is the basic unit for message distribution. Publishers send messages to a topic, and subscribers receive messages from that topic.

There are several implementations of the Topic interface:

- **Basic Topic**: A simple topic with no history.
- **Topic with History**: Maintains a history of messages using a mutex-protected slice. Good for most use cases where history is needed.
- **Topic with Lock-Free History**: Maintains history using a lock-free ring buffer. Recommended for high-concurrency environments to minimize contention on the publish path.

```go
// Create a basic topic
topic := pubsub.NewTopic()

// Create a topic with history
topicWithHistory := pubsub.NewTopic(pubsub.WithHistory(10))

// Create a topic with lock-free history for high performance
topicLockFree := pubsub.NewTopic(pubsub.WithLockFreeHistoryOpt(100))

// Subscribe to the topic
ch := topic.Subscribe()

// Publish to the topic
topic.Publish("Hello from topic!")
```

### PubSub

The PubSub interface manages multiple topics and provides a higher-level API.

There are two main implementations of the PubSub interface:

- **Default PubSub** (`NewPubSub`): Suitable for most applications with a moderate number of topics and message rates. It uses a single map and mutex for topic management.
- **Sharded PubSub** (`NewShardedPubSub`): Optimized for high-concurrency scenarios with many topics. It reduces lock contention by sharding topics across multiple internal maps.

```go
// Create a default PubSub instance
ps := pubsub.NewPubSub()

// Create a sharded PubSub instance for high concurrency
psHighConf := pubsub.NewShardedPubSub()

// Subscribe to a specific topic
ch1 := ps.Subscribe("topic1")

// Subscribe to all topics
ch2 := ps.SubscribeAll()

// Publish to a topic
ps.Publish("topic1", "Message for topic1")
```

### Message History

Enable message history to allow new subscribers to receive previously published messages.

```go
// Create a PubSub with history
ps := pubsub.NewPubSub(pubsub.WithHistorySize(10))

// Publish some messages
ps.Publish("news", "Breaking news 1")
ps.Publish("news", "Breaking news 2")

// New subscribers will receive historical messages
ch := ps.Subscribe("news")
```

### Type Filtering

Filter messages by type using generics.

```go
type UserEvent struct {
    Username string
    Action   string
}

// Subscribe to only receive UserEvent messages
userEvents := pubsub.Subscribe[UserEvent](topic)
for event := range userEvents {
    fmt.Printf("User %s performed action: %s\n", event.Username, event.Action)
}
```

### Custom Filtering

Apply custom filters to messages.

```go
// Filter for high-priority messages
highPriorityMsgs := pubsub.SubscribeWithFilter(topic, func(msg AlertMessage) bool {
    return msg.Priority > 7
})
```

### Channel Merging

Merge multiple channels into a single channel.

```go
// Merge channels from different topics
ch1 := topic1.Subscribe()
ch2 := topic2.Subscribe()
mergedCh := pubsub.Merge[UserEvent](ch1, ch2)
```

### Reliable Delivery

For scenarios where message delivery is more important than non-blocking speed, use the `PublishReliable` and
`SubscribeUnbuffered` methods.

```go
// Create unbuffered subscription
ch := ps.SubscribeUnbuffered("critical-events")

// Publish with 100ms timeout for blocking delivery
ps.PublishReliable("critical-events", "Important Message")
```

## Examples

See the `examples` directory for more detailed examples:

- [Basic usage](examples/basic/main.go)
- [Working with history](examples/history/main.go)
- [Type filtering](examples/filtering/main.go)
- [Channel merging](examples/merging/main.go)

## Performance

Run the benchmarks to evaluate performance:

```bash
go test -bench=. github.com/dioad/pubsub
```

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.