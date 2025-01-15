package pubsub

import (
	"sync"
)

type Opt func(*PubSub)

func WithHistorySize(size int) Opt {
	return func(ps *PubSub) {
		ps.historySize = size
		ps.enableHistory = true
	}
}

type PubSub struct {
	mu            sync.RWMutex
	subscribers   map[string][]chan interface{}
	history       map[string][]interface{}
	historySize   int
	enableHistory bool
}

func NewPubSub(opt ...Opt) *PubSub {
	ps := &PubSub{
		subscribers: make(map[string][]chan interface{}),
	}

	for _, o := range opt {
		o(ps)
	}

	if ps.enableHistory {
		ps.history = make(map[string][]interface{})
	}

	return ps
}

func (ps *PubSub) SubscribeFunc(topic string, withHistory bool, f func(msg interface{})) {
	ch := ps.Subscribe(topic, withHistory)
	go func() {
		for msg := range ch {
			f(msg)
		}
	}()
}

func (ps *PubSub) Subscribe(topic string, withHistory bool) <-chan interface{} {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	ch := make(chan interface{})

	if withHistory {
		// Is it confusing having a buffer withHistory and no buffer without
		ch = make(chan interface{}, ps.historySize*2)
		if history, ok := ps.history[topic]; ok {
			for _, msg := range history {
				ch <- msg
			}
		}
	}
	ps.subscribers[topic] = append(ps.subscribers[topic], ch)

	return ch
}

func (ps *PubSub) SubscribeAllFunc(withHistory bool, f func(msg interface{})) {
	ps.SubscribeFunc("*", withHistory, f)
}

func (ps *PubSub) SubscribeAll(withHistory bool) <-chan interface{} {
	return ps.Subscribe("*", withHistory)
}

func (ps *PubSub) publishToTopicWithHistory(topic string, msg interface{}) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	// Add message to history
	if ps.enableHistory {
		ps.history[topic] = append(ps.history[topic], msg)
		if len(ps.history[topic]) > ps.historySize {
			ps.history[topic] = ps.history[topic][1:]
		}
	}

	for _, ch := range ps.subscribers[topic] {
		go func(c chan interface{}) {
			c <- msg
		}(ch)
	}
}

func (ps *PubSub) Publish(topic string, msg interface{}) {
	ps.publishToTopicWithHistory(topic, msg)
	ps.publishToTopicWithHistory("*", msg)
}

func (ps *PubSub) UnsubscribeAll(sub <-chan interface{}) {
	ps.Unsubscribe("*", sub)
}

func (ps *PubSub) Unsubscribe(topic string, sub <-chan interface{}) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	subs := ps.subscribers[topic]
	for i, ch := range subs {
		if ch == sub {
			ps.subscribers[topic] = append(subs[:i], subs[i+1:]...)
			close(ch)
			break
		}
	}
}
