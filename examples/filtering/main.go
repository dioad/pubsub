package main

import (
	"fmt"
	"time"

	"github.com/dioad/pubsub"
)

// Define some message types for demonstration
type TextMessage struct {
	Content string
}

type NumericMessage struct {
	Value int
}

type StatusMessage struct {
	Status  string
	Success bool
}

func main() {
	fmt.Println("PubSub Type Filtering Example")
	fmt.Println("============================")

	// Create a topic
	topic := pubsub.NewTopic()

	// Subscribe with type filtering for TextMessage
	textCh := pubsub.Subscribe[TextMessage](topic)

	// Subscribe with type filtering for NumericMessage
	numericCh := pubsub.Subscribe[NumericMessage](topic)

	// Subscribe with type filtering and additional filtering for StatusMessage
	// Only receive successful status messages
	successCh := pubsub.SubscribeWithFilter(topic, func(msg StatusMessage) bool {
		return msg.Success
	})

	// Start goroutines to process each channel
	go processTextMessages(textCh)
	go processNumericMessages(numericCh)
	go processSuccessMessages(successCh)

	// Publish different types of messages
	fmt.Println("Publishing mixed message types...")
	topic.Publish(
		TextMessage{Content: "Hello, World!"},
		NumericMessage{Value: 42},
		StatusMessage{Status: "Processing", Success: true},
		TextMessage{Content: "Another text message"},
		StatusMessage{Status: "Failed", Success: false},
		NumericMessage{Value: 100},
		StatusMessage{Status: "Completed", Success: true},
	)

	// Wait for messages to be processed
	time.Sleep(100 * time.Millisecond)

	fmt.Println("\nFiltering with FilterChan Example")
	fmt.Println("================================")

	// Create a raw subscription
	rawCh := topic.Subscribe()

	// Filter for even numeric messages
	evenNumericCh := pubsub.FilterChan(rawCh, func(msg NumericMessage) bool {
		return msg.Value%2 == 0
	})

	// Process even numeric messages
	go func() {
		for msg := range evenNumericCh {
			fmt.Printf("Even numeric message: %d\n", msg.Value)
		}
	}()

	// Publish more numeric messages
	fmt.Println("Publishing numeric messages...")
	topic.Publish(
		NumericMessage{Value: 1},
		NumericMessage{Value: 2},
		NumericMessage{Value: 3},
		NumericMessage{Value: 4},
	)

	// Wait for messages to be processed
	time.Sleep(100 * time.Millisecond)

	fmt.Println("\nCastChan Example")
	fmt.Println("===============")

	// Create a raw channel
	rawInterfaceCh := make(chan any)

	// Cast to TextMessage channel
	castCh := pubsub.CastChan[TextMessage](rawInterfaceCh)

	// Process cast messages
	go func() {
		for msg := range castCh {
			fmt.Printf("Cast message: %s\n", msg.Content)
		}
	}()

	// Send messages to the raw channel
	go func() {
		rawInterfaceCh <- TextMessage{Content: "Cast message 1"}
		rawInterfaceCh <- NumericMessage{Value: 999} // This will be filtered out
		rawInterfaceCh <- TextMessage{Content: "Cast message 2"}
		time.Sleep(50 * time.Millisecond)
		close(rawInterfaceCh)
	}()

	// Wait for messages to be processed
	time.Sleep(100 * time.Millisecond)
}

func processTextMessages(ch <-chan TextMessage) {
	for msg := range ch {
		fmt.Printf("Text message received: %s\n", msg.Content)
	}
}

func processNumericMessages(ch <-chan NumericMessage) {
	for msg := range ch {
		fmt.Printf("Numeric message received: %d\n", msg.Value)
	}
}

func processSuccessMessages(ch <-chan StatusMessage) {
	for msg := range ch {
		fmt.Printf("Success status message: %s\n", msg.Status)
	}
}
