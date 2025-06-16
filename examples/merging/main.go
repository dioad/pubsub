package main

import (
	"fmt"
	"time"

	"github.com/dioad/pubsub"
)

// Define a message type for demonstration
type Message struct {
	Source  string
	Content string
}

func main() {
	fmt.Println("PubSub Channel Merging Example")
	fmt.Println("=============================")

	// Create multiple topics
	topicA := pubsub.NewTopic()
	topicB := pubsub.NewTopic()
	topicC := pubsub.NewTopic()

	// Subscribe to each topic
	chA := topicA.Subscribe()
	chB := topicB.Subscribe()
	chC := topicC.Subscribe()

	// Merge the channels
	fmt.Println("Merging channels from multiple topics...")
	mergedCh := pubsub.Merge[Message](chA, chB, chC)

	// Process messages from the merged channel
	go func() {
		for msg := range mergedCh {
			fmt.Printf("Merged channel received: [%s] %s\n", msg.Source, msg.Content)
		}
	}()

	// Publish messages to different topics
	fmt.Println("Publishing messages to different topics...")
	topicA.Publish(Message{Source: "Topic A", Content: "Message from A-1"})
	topicB.Publish(Message{Source: "Topic B", Content: "Message from B-1"})
	topicC.Publish(Message{Source: "Topic C", Content: "Message from C-1"})
	topicA.Publish(Message{Source: "Topic A", Content: "Message from A-2"})
	topicB.Publish(Message{Source: "Topic B", Content: "Message from B-2"})

	// Wait for messages to be processed
	time.Sleep(100 * time.Millisecond)

	fmt.Println("\nMerging PubSub Topics Example")
	fmt.Println("============================")

	// Create a PubSub instance
	ps := pubsub.NewPubSub()

	// Subscribe to multiple topics
	psCh1 := ps.Subscribe("alerts")
	psCh2 := ps.Subscribe("notifications")

	// Merge the channels
	mergedPsCh := pubsub.Merge[Message](psCh1, psCh2)

	// Process messages from the merged channel
	go func() {
		for msg := range mergedPsCh {
			fmt.Printf("Merged PubSub received: [%s] %s\n", msg.Source, msg.Content)
		}
	}()

	// Publish messages to different topics
	fmt.Println("Publishing messages to different PubSub topics...")
	ps.Publish("alerts", Message{Source: "Alerts", Content: "System alert"})
	ps.Publish("notifications", Message{Source: "Notifications", Content: "User notification"})
	ps.Publish("alerts", Message{Source: "Alerts", Content: "Critical alert"})

	// Wait for messages to be processed
	time.Sleep(100 * time.Millisecond)

	fmt.Println("\nAdvanced Merging with Filtering Example")
	fmt.Println("=====================================")

	// Filter messages from Topic A
	filteredChA := pubsub.FilterChan(chA, func(msg Message) bool {
		return msg.Source == "Topic A" && msg.Content == "Special message"
	})

	// Filter messages from Topic B
	filteredChB := pubsub.FilterChan(chB, func(msg Message) bool {
		return msg.Source == "Topic B" && msg.Content == "Special message"
	})

	// Create a new channel to manually merge the filtered channels
	mergedFilteredCh := make(chan Message, 10)

	// Start goroutines to forward messages from filtered channels to the merged channel
	go func() {
		for msg := range filteredChA {
			mergedFilteredCh <- msg
		}
	}()

	go func() {
		for msg := range filteredChB {
			mergedFilteredCh <- msg
		}
	}()

	// Process messages from the merged filtered channel
	go func() {
		for msg := range mergedFilteredCh {
			fmt.Printf("Merged filtered received: [%s] %s\n", msg.Source, msg.Content)
		}
	}()

	// Publish special messages
	fmt.Println("Publishing special messages...")
	topicA.Publish(Message{Source: "Topic A", Content: "Regular message"})
	topicA.Publish(Message{Source: "Topic A", Content: "Special message"})
	topicB.Publish(Message{Source: "Topic B", Content: "Regular message"})
	topicB.Publish(Message{Source: "Topic B", Content: "Special message"})

	// Wait for messages to be processed
	time.Sleep(100 * time.Millisecond)

	fmt.Println("\nExample completed")
}
