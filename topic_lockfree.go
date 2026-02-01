package pubsub

// WithLockFreeHistory is an option for NewPubSub/NewShardedPubSub that
// enables lock-free message history for all topics.
func WithLockFreeHistory(size int) Opt {
	return func(ps *pubSub) {
		ps.topicFunc = func() Topic { return NewTopic(WithLockFreeHistoryOpt(size), WithTopicObserver(ps.observer)) }
	}
}
