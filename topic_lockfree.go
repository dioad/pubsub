package pubsub

// WithLockFreeHistorySize is an option for NewPubSub/NewShardedPubSub that
// enables lock-free message history for all topics.
func WithLockFreeHistorySize(size int) Opt {
	return func(ps *pubSub) {
		ps.topicFunc = func(name string) Topic {
			return NewTopic(WithLockFreeHistory(size), WithTopicObserver(ps.observer), WithTopicName(name))
		}
	}
}
