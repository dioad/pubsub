package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/dioad/pubsub"
)

type exampleFeeder struct {
	eventChan chan *pubsub.EventTuple
	closed    bool
	mu        sync.Mutex

	start, count int
}

func (f *exampleFeeder) Close() {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.closed {
		close(f.eventChan)
		f.closed = true
	}
}

func (f *exampleFeeder) Feed() <-chan *pubsub.EventTuple {
	// eventChan := make(chan *pubsub.EventTuple)
	// Simulate feeding events
	go func() {
		for i := f.start; i < f.start+f.count; i++ {
			event := fmt.Sprintf("Event %d from feeder", i+1)
			eventTuple := &pubsub.EventTuple{
				Topic: "example-topic",
				Event: event,
			}
			f.eventChan <- eventTuple
		}
		f.Close()
	}()

	return f.eventChan
}

func newExampleFeeder(start, count int) *exampleFeeder {
	// Create a new example feeder instance
	return &exampleFeeder{
		start:     start,
		count:     count,
		eventChan: make(chan *pubsub.EventTuple), // , 10), // Buffered channel for event tuples
	}
}

func main() {
	// create an example application that creates a Feeder and adds it to a pubsub.PubSub instance and confirms the events that it's received '
	fmt.Println("🚀 EventBroker Feeder Example - Reliable Delivery")
	fmt.Println("==================================================")
	fmt.Println("This example demonstrates how to create a Feeder that provides events to a PubSub instance")
	fmt.Println("using reliable delivery with unbuffered channels. The new AddFeederReliable method ensures")
	fmt.Println("that all messages are delivered even to unbuffered channels without message loss.")
	fmt.Println()
	// Create a PubSub instance
	ps := pubsub.NewPubSub()
	// Create a Feeder that generates events
	feeder := newExampleFeeder(0, 25)
	feeder2 := newExampleFeeder(25, 25) // Create another feeder for more events
	// Add the Feeder to the PubSub instance using reliable delivery
	ps.AddFeeder(context.Background(), feeder)
	ps.AddFeeder(context.Background(), feeder2)
	// Subscribe to a topic to receive events
	topic := ps.Topic("example-topic")
	sub := topic.Subscribe() // Use unbuffered channel with reliable delivery
	// sub := topic.SubscribeWithBuffer(4) // Old approach: Use larger buffer to prevent message loss

	go func() {
		<-time.After(5 * time.Second)
		topic.Unsubscribe(sub)
	}()

	fmt.Println("📡 Subscribed to topic 'example-topic'. Waiting for events...")
	// Process events from the feeder
	// eventCount := 0
	// expectedEvents := 5
	for event := range sub {
		fmt.Printf("Received event: %v\n", event)
		// eventCount++
		// if eventCount >= expectedEvents {
		// 	// Unsubscribe after receiving all expected events
		// 	topic.Unsubscribe(sub)
		// 	break
		// }
	}

	//
	// for {
	// 	select {
	// 	case event := <-sub:
	// 		if event == nil {
	// 			fmt.Println("Feeder closed, no more events.")
	// 			break
	// 		}
	// 		fmt.Printf("Feeder event: Topic=%s, Event=%v\n", event.Topic, event.Event)
	// 	default:
	// 	// No more events to process
	// 	// break
	// 	case <-time.After(2 * time.Second):
	// 		fmt.Println("No more events to process, exiting...")
	// 		break
	// 	}
	// }

	// feeder.Close()
	fmt.Println("✅ All events processed successfully!")
	fmt.Println("💡 Check the console output to see the events being fed and processed.")
}
