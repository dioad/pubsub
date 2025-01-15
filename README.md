# In-Memory Pub/Sub System in Go

This repository contains an implementation of an in-memory publish/subscribe (pub/sub) system in Go. The system supports
subscribing to specific topics, subscribing to all topics, and maintaining a history of messages for each topic.

## Features

- **Subscribe to specific topics**: Allows clients to subscribe to specific topics and receive messages published to 
  those topics.
- **Subscribe to all topics**: Allows clients to subscribe to all topics and receive messages published to any topic.
- **Message history**: Optionally maintains a history buffer of messages for each topic and feeds the history to new 
  subscribers if requested.
- **Asynchronous message delivery**: Messages are delivered to subscribers asynchronously using Go channels.

## Usage

Here is an example of how to use the pub/sub system:

```go
package main

import (
	"fmt"
	"time"

	"github.com/dioad/pubsub"
)

func main() {
	ps := pubsub.NewPubSub(pubsub.WithHistorySize(10))

	ch1 := ps.Subscribe("topic1", true)
	ch2 := ps.SubscribeAll(true)

	ps.Publish("topic1", "Hello, Topic 1!")
	ps.Publish("topic2", "Hello, Topic 2!")

	go func() {
		for msg := range ch1 {
			fmt.Println("Received on topic1:", msg)
		}
	}()

	go func() {
		for msg := range ch2 {
			fmt.Println("Received on all topics:", msg)
		}
	}()

	time.Sleep(1 * time.Second)
}
```

## Testing

The repository includes tests for the pub/sub system. To run the tests, use the following command:

```sh
go test ./...
```

## License

This project is licensed under the BSD 3-Clause License. See the `LICENSE` file for details.
