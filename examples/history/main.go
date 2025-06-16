package main

import (
	"fmt"

	"github.com/dioad/pubsub"
)

func main() {
	fmt.Println("PubSub with History Example")
	fmt.Println("==========================")

	// Create a new PubSub instance with history
	ps := pubsub.NewPubSub(pubsub.WithHistorySize(10))

	// Publish some messages before subscribing
	fmt.Println("Publishing messages before subscribing...")
	ps.Publish("news", "Breaking news 1")
	ps.Publish("news", "Breaking news 2")
	ps.Publish("news", "Breaking news 3")

	// Subscribe to the topic after messages have been published
	fmt.Println("\nSubscribing to topic with history...")
	ch := ps.Subscribe("news")

	// Read and display historical messages
	fmt.Println("Reading historical messages:")

	// Read the first 3 messages (which should be the historical ones)
	for i := 0; i < 3; i++ {
		msg := <-ch
		fmt.Printf("Historical message received: %v\n", msg)
	}

	// Publish more messages after subscribing
	fmt.Println("\nPublishing more messages after subscribing...")
	ps.Publish("news", "Breaking news 4")
	ps.Publish("news", "Breaking news 5")

	// Read the new messages
	for i := 0; i < 2; i++ {
		msg := <-ch
		fmt.Printf("New message received: %v\n", msg)
	}

	fmt.Println("\nTopic with History Example")
	fmt.Println("=========================")

	// Create a topic with history directly
	topic := pubsub.NewTopicWithHistory(5)

	// Publish some messages to the topic
	fmt.Println("Publishing messages to topic before subscribing...")
	topic.Publish("Topic message 1")
	topic.Publish("Topic message 2")

	// Subscribe to the topic
	fmt.Println("\nSubscribing to topic with history...")
	topicCh := topic.Subscribe()

	// Read historical messages
	for i := 0; i < 2; i++ {
		msg := <-topicCh
		fmt.Printf("Historical topic message: %v\n", msg)
	}

	// Demonstrate history size limit
	fmt.Println("\nDemonstrating history size limit...")

	// Create a topic with small history
	limitedTopic := pubsub.NewTopicWithHistory(2)

	// Publish more messages than the history size
	limitedTopic.Publish("Limited 1")
	limitedTopic.Publish("Limited 2")
	limitedTopic.Publish("Limited 3") // This will push "Limited 1" out of history

	// Subscribe and check what's in history
	limitedCh := limitedTopic.Subscribe()

	// Should only receive the last 2 messages
	for i := 0; i < 2; i++ {
		msg := <-limitedCh
		fmt.Printf("Limited history message: %v\n", msg)
	}

	// Cleanup
	ps.Unsubscribe("news", ch)
	topic.Unsubscribe(topicCh)
	limitedTopic.Unsubscribe(limitedCh)
}
