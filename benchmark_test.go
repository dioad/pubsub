package pubsub

import (
	"testing"
)

// BenchmarkTopicPublish measures the performance of publishing messages to a topic
func BenchmarkTopicPublish(b *testing.B) {
	topic := NewTopic()

	// Subscribe to ensure messages are being processed
	_ = topic.Subscribe()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		topic.Publish(i)
	}
}

// BenchmarkTopicSubscribe measures the performance of subscribing to a topic
func BenchmarkTopicSubscribe(b *testing.B) {
	topic := NewTopic()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ch := topic.Subscribe()
		topic.Unsubscribe(ch)
	}
}

// BenchmarkTopicWithHistoryPublish measures the performance of publishing to a topic with history
func BenchmarkTopicWithHistoryPublish(b *testing.B) {
	topic := NewTopic(WithHistory(100))

	// Subscribe to ensure messages are being processed
	_ = topic.Subscribe()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		topic.Publish(i)
	}
}

// BenchmarkPubSubPublish measures the performance of publishing messages through PubSub
func BenchmarkPubSubPublish(b *testing.B) {
	ps := NewPubSub()

	// Subscribe to ensure messages are being processed
	_ = ps.Subscribe("test-topic")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ps.Publish("test-topic", i)
	}
}

// BenchmarkPubSubWithHistoryPublish measures the performance of publishing through PubSub with history
func BenchmarkPubSubWithHistoryPublish(b *testing.B) {
	ps := NewPubSub(WithHistorySize(100))

	// Subscribe to ensure messages are being processed
	_ = ps.Subscribe("test-topic")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ps.Publish("test-topic", i)
	}
}

// BenchmarkFilterChan measures the performance of filtering messages
func BenchmarkFilterChan(b *testing.B) {
	topic := NewTopic()
	ch := topic.Subscribe()

	// Create a filter that accepts all integers
	filteredCh := FilterChan(ch, func(i int) bool { return true })

	// Start a goroutine to consume messages from the filtered channel
	go func() {
		for range filteredCh {
			// Just consume the messages
		}
	}()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		topic.Publish(i)
	}
}

// BenchmarkSubscribeWithFilter measures the performance of subscribing with a filter
func BenchmarkSubscribeWithFilter(b *testing.B) {
	topic := NewTopic()

	// Create a filtered subscription
	filteredCh := SubscribeWithFilter(topic, func(i int) bool { return i%2 == 0 })

	// Start a goroutine to consume messages from the filtered channel
	go func() {
		for range filteredCh {
			// Just consume the messages
		}
	}()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		topic.Publish(i)
	}
}

// BenchmarkMerge measures the performance of merging channels
func BenchmarkMerge(b *testing.B) {
	topic1 := NewTopic()
	topic2 := NewTopic()

	ch1 := topic1.Subscribe()
	ch2 := topic2.Subscribe()

	// Merge the channels
	merged := Merge[int](ch1, ch2)

	// Start a goroutine to consume messages from the merged channel
	go func() {
		for range merged {
			// Just consume the messages
		}
	}()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if i%2 == 0 {
			topic1.Publish(i)
		} else {
			topic2.Publish(i)
		}
	}
}

// BenchmarkShardedPubSubPublish measures the performance of the sharded PubSub
func BenchmarkShardedPubSubPublish(b *testing.B) {
	ps := NewShardedPubSub()

	// Subscribe to ensure messages are being processed
	_ = ps.Subscribe("test-topic")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ps.Publish("test-topic", i)
	}
}

// BenchmarkShardedPubSubWithHistoryPublish measures sharded PubSub with history
func BenchmarkShardedPubSubWithHistoryPublish(b *testing.B) {
	ps := NewShardedPubSub(WithHistorySize(100))

	// Subscribe to ensure messages are being processed
	_ = ps.Subscribe("test-topic")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ps.Publish("test-topic", i)
	}
}

// BenchmarkShardedPubSubWithLockFreeHistory measures sharded PubSub with lock-free history
func BenchmarkShardedPubSubWithLockFreeHistory(b *testing.B) {
	ps := NewShardedPubSub(WithLockFreeHistory(100))

	// Subscribe to ensure messages are being processed
	_ = ps.Subscribe("test-topic")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ps.Publish("test-topic", i)
	}
}

// BenchmarkTopicWithLockFreeHistoryPublish measures lock-free history topic
func BenchmarkTopicWithLockFreeHistoryPublish(b *testing.B) {
	topic := NewTopic(WithLockFreeHistoryOpt(100))

	// Subscribe to ensure messages are being processed
	_ = topic.Subscribe()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		topic.Publish(i)
	}
}

// BenchmarkShardedPubSubParallel measures parallel publish performance
func BenchmarkShardedPubSubParallel(b *testing.B) {
	ps := NewShardedPubSub(WithLockFreeHistory(100))

	// Subscribe to multiple topics
	_ = ps.Subscribe("topic-1")
	_ = ps.Subscribe("topic-2")
	_ = ps.Subscribe("topic-3")
	_ = ps.Subscribe("topic-4")

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			topic := "topic-" + string(rune('1'+i%4))
			ps.Publish(topic, i)
			i++
		}
	})
}

// BenchmarkOriginalPubSubParallel measures parallel publish for comparison
func BenchmarkOriginalPubSubParallel(b *testing.B) {
	ps := NewPubSub(WithHistorySize(100))

	// Subscribe to multiple topics
	_ = ps.Subscribe("topic-1")
	_ = ps.Subscribe("topic-2")
	_ = ps.Subscribe("topic-3")
	_ = ps.Subscribe("topic-4")

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			topic := "topic-" + string(rune('1'+i%4))
			ps.Publish(topic, i)
			i++
		}
	})
}
