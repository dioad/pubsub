package main

import (
	"fmt"
	"time"

	"github.com/dioad/pubsub"
)

func main() {
	fmt.Println("Basic PubSub Example")
	fmt.Println("====================")

	// Create a new PubSub instance
	ps := pubsub.NewPubSub()

	// Subscribe to a topic
	ch := ps.Subscribe("greetings")

	// Subscribe with a callback function
	ps.SubscribeFunc("greetings", func(msg interface{}) {
		fmt.Printf("Callback received: %v\n", msg)
	})

	// Start a goroutine to read from the channel
	go func() {
		for msg := range ch {
			fmt.Printf("Channel received: %v\n", msg)
		}
	}()

	// Publish messages to the topic
	fmt.Println("Publishing messages...")
	ps.Publish("greetings", "Hello, World!")
	ps.Publish("greetings", "How are you?")

	// Wait for messages to be processed
	time.Sleep(100 * time.Millisecond)

	fmt.Println("\nTopic Example")
	fmt.Println("=============")

	// Create a new Topic directly
	topic := pubsub.NewTopic()

	// Subscribe to the topic
	topicCh := topic.Subscribe()

	// Start a goroutine to read from the channel
	go func() {
		for msg := range topicCh {
			fmt.Printf("Topic received: %v\n", msg)
		}
	}()

	// Publish messages to the topic
	fmt.Println("Publishing to topic...")
	topic.Publish("Direct topic message")

	// Wait for messages to be processed
	time.Sleep(100 * time.Millisecond)

	fmt.Println("\nUnsubscribe Example")
	fmt.Println("==================")

	// Unsubscribe from the topic
	ps.Unsubscribe("greetings", ch)
	topic.Unsubscribe(topicCh)

	// These messages won't be received
	ps.Publish("greetings", "This won't be received")
	topic.Publish("This won't be received either")

	fmt.Println("Unsubscribed successfully")
}
