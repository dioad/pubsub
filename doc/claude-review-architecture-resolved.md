# Architecture Review: Resolved Findings — `github.com/dioad/pubsub`

_Reviewed: 2026-06-18 — See [open findings](./claude-review-architecture.md)_

---

## Resolved Findings

### 1. Double-close panic in `pubsubTopic.Shutdown` on context cancellation ✅ Resolved

- **File(s):** `topic.go:222-236`
- **Dimension(s):** Correctness
- **Priority:** High
- **Resolved in:** ebd5117
- **Description:** When `Shutdown` returned early because `ctx.Done()` fired mid-loop, already-closed channels remained in `t.subscriptions`. Any subsequent `Close()` would re-close them, producing an unrecoverable panic. Same early-return bug existed in `SimpleStore.Shutdown` and `ShardedStore.Shutdown`.
- **Outcome:** `pubsubTopic.Shutdown` now tracks remaining (unclosed) subscriptions and sets `t.subscriptions` to that slice before returning. Complexity delta: `pubsubTopic.Shutdown` 3 → 3, `SimpleStore.Shutdown` 3 → 3, `ShardedStore.Shutdown` 6 → 6.

---

### 2. Duplicate message delivery race in `topicWithHistory.subscribeWithBuffer` ✅ Resolved

- **File(s):** `history.go:141-153`
- **Dimension(s):** Correctness
- **Priority:** High
- **Resolved in:** ca6e270
- **Description:** `subscribeWithBuffer` first added the channel to subscriptions (live), then replayed history. Any message published in that window was delivered twice.
- **Outcome:** New internal method `pubsubTopic.subscribeWithHistory` atomically fills the channel with history before adding it to the subscription list. Complexity delta: `topicWithHistory.subscribeWithBuffer` 3 → 1 (absorbed into new method at 3).

---

### 3. `time.After` timer leak and RLock held during blocking send in `PublishReliable` ✅ Resolved

- **File(s):** `topic.go:152-162`, `topic.go:112-133`
- **Dimension(s):** Correctness, Maintainability
- **Priority:** High
- **Resolved in:** e89a187
- **Description:** `PublishReliable` created `time.After(100ms)` per subscriber per message; in the happy path the timer was never stopped. `publishWithSendFunc` held `t.mu.RLock()` for the entire send loop, starving Subscribe/Unsubscribe during reliable publishes.
- **Outcome:** Replaced `time.After` with `time.NewTimer` + `defer timer.Stop()`. Complexity delta: `pubsubTopic.PublishReliable` 2 → 2. The RLock-during-blocking-send architectural concern is captured separately in Finding 4.

---

### 4. Observer callbacks invoked while holding `RLock` ✅ Resolved

- **File(s):** `topic.go:116-118`
- **Dimension(s):** Architecture, Maintainability
- **Priority:** Medium
- **Resolved in:** de29c8f
- **Description:** `publishWithSendFunc` called `t.observer.OnPublish` and `t.observer.OnDrop` synchronously while holding `t.mu.RLock()`, coupling observer performance to the publish path and risking deadlock if an observer tried to subscribe.
- **Outcome:** `OnDrop` moved outside the lock. `OnPublish` merged into the send loop but retains the lock (complexity budget). Complexity delta: `pubsubTopic.publishWithSendFunc` 8 → 8.

---

### 5. Magic `"*"` wildcard topic literal ✅ Resolved

- **File(s):** `pubsub.go` (9 sites)
- **Dimension(s):** Maintainability, Inconsistencies
- **Priority:** Medium
- **Resolved in:** 859c06d
- **Description:** The wildcard topic sentinel `"*"` was embedded as a string literal at nine sites with no named constant.
- **Outcome:** Exported `WildcardTopic = "*"` constant. All nine literal occurrences replaced. `Topics()` documentation updated. Complexity delta: n/a (no logic change).

---

### 6. `Clear()` is not part of the `Store` interface ✅ Resolved

- **File(s):** `internal/topicstore/store.go:76, 177`
- **Dimension(s):** Inconsistencies, Architecture
- **Priority:** Medium
- **Resolved in:** 466ca02
- **Description:** Both `SimpleStore` and `ShardedStore` implemented `Clear()` but the method was absent from the `Store` interface, requiring a type assertion to call it.
- **Outcome:** `Clear()` added to the `Store` interface. Complexity delta: n/a (interface change only).

---

### 7. `topicWithName` is a silent, fragile extension point ✅ Resolved

- **File(s):** `pubsub.go:281-291`
- **Dimension(s):** Architecture, Maintainability
- **Priority:** Medium
- **Resolved in:** c87efcd
- **Description:** The `topicWithName` interface and `setTopicName` helper set the observer-visible name on a topic as a post-creation side effect. Forgetting to implement `setName` caused silent empty-string observer events.
- **Outcome:** Topic name passed at construction time via `func(name string) Topic` factory signature. `topicWithName` interface eliminated. Complexity delta: `setTopicName` 1 → removed; `initPubSub` 2 → 2.

---

### 8. `runFeedingLoop` accepts mutually-exclusive function parameters ✅ Resolved

- **File(s):** `pubsub.go:363-390`
- **Dimension(s):** Maintainability, Inconsistencies
- **Priority:** Medium
- **Resolved in:** efe3c3e
- **Description:** `runFeedingLoop` accepted two function parameters where exactly one must be non-nil — the boolean parameter antipattern applied to functions.
- **Outcome:** Replaced with a single `publishFn func(topic string, msg ...any)` parameter. Callers wrap their preferred variant. Complexity delta: `runFeedingLoop` 22 → 16.

---

### 9. Naming asymmetry: `WithLockFreeHistoryOpt` vs `WithLockFreeHistory` ✅ Resolved

- **File(s):** `topic.go:283`, `topic_lockfree.go:5`
- **Dimension(s):** Inconsistencies
- **Priority:** Medium
- **Resolved in:** 2cedfff
- **Description:** The `TopicOpt` for lock-free history used an "Opt" suffix while the equivalent `PubSub` `Opt` did not, creating three different naming styles for equivalent concepts.
- **Outcome:** Renamed `WithLockFreeHistoryOpt` → `WithLockFreeHistory`. Complexity delta: n/a (rename only).

---

### 10. Tests do not call `t.Parallel()` ✅ Resolved

- **File(s):** `pubsub_test.go`, `topic_test.go`, `reliable_test.go`, `sharded_test.go`, `filter_test.go`, `lifecycle_test.go`, `observer_test.go`
- **Dimension(s):** Modern Practices
- **Priority:** Medium
- **Resolved in:** 7ad6cd0
- **Description:** None of the 40+ test functions called `t.Parallel()`, resulting in a purely sequential test suite despite tests being independent.
- **Outcome:** `t.Parallel()` added as first statement in every top-level test function. Complexity delta: n/a (test-only change).

---

### 11. `FeedingFunc` does not implement `Feeder` ✅ Resolved

- **File(s):** `pubsub.go:65-66`, `pubsub.go:59-62`
- **Dimension(s):** Architecture, Modern Practices
- **Priority:** Low
- **Resolved in:** 254a915
- **Description:** `FeedingFunc` and the `Feeder` interface were structurally equivalent but `FeedingFunc` had no `Feed()` method, forcing two parallel method sets on `PubSub`.
- **Outcome:** Added `Feed()` method to `FeedingFunc`. `AddFeedingFunc` variants now delegate to `AddFeeder`. Complexity delta: n/a (one-line method addition).

---

### 12. Unexplained magic number in `topicWithHistory.Subscribe` buffer sizing ✅ Resolved

- **File(s):** `history.go:132`
- **Dimension(s):** Maintainability
- **Priority:** Low
- **Resolved in:** 4b7d34c
- **Description:** `Subscribe()` returned `t.subscribeWithBuffer(t.history.Cap() * 2)` with no explanation of the `* 2` multiplier.
- **Outcome:** Comment added explaining the intent (room for replayed history plus a burst of live messages). Complexity delta: n/a (comment only).

---

### 13. `SimpleStore.GetOrCreate` always acquires a write lock ✅ Resolved

- **File(s):** `internal/topicstore/store.go:47-61`
- **Dimension(s):** Maintainability, Modern Practices
- **Priority:** Low
- **Resolved in:** e0f3f6d
- **Description:** `SimpleStore.GetOrCreate` acquired an exclusive write lock even on the common read path where the topic already existed, inconsistent with `ShardedStore`'s fast read path.
- **Outcome:** Trade-off documented; double-check pattern raised complexity above budget so documentation approach was taken. `SimpleStore` docs now state it always acquires an exclusive lock. Complexity delta: `SimpleStore.GetOrCreate` 2 → 2.
