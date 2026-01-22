# PubSub

A lightweight, type-safe publish/subscribe library for Go applications.

## Features

- Simple, intuitive API for topic-based messaging
- Type-safe message filtering with generics
- Support for message history
- Channel merging capabilities
- Non-blocking message delivery
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

```go
// Create a topic directly
topic := pubsub.NewTopic()

// Subscribe to the topic
ch := topic.Subscribe()

// Publish to the topic
topic.Publish("Hello from topic!")
```

### PubSub

The PubSub interface manages multiple topics and provides a higher-level API.

```go
// Create a PubSub instance
ps := pubsub.NewPubSub()

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

[Apache License 2.0](LICENSE)