# Architecture Review

## Executive Summary

The codebase is well-structured with clean separation between the topic store, topic implementations, and PubSub façade. The design shows thoughtful attention to performance (sharded store, lock-free ring buffer, functional options pattern) and the generics-based filter/merge API is a strong ergonomic win. However, there are two correctness defects that can cause data loss or crashes under normal concurrent use: a double-close panic in `Shutdown` when a context is cancelled mid-loop, and a race in the history-replay path that silently duplicates messages for new subscribers. A third high-priority issue — blocking channel sends holding a read-lock with leaked timers — affects throughput at scale. These should be addressed before the library is used in production.

---

## Findings

### 1. Double-close panic in `pubsubTopic.Shutdown` on context cancellation

- **File(s):** `topic.go:222-236`
- **Dimension(s):** Correctness
- **Priority:** High
- **Status:** Resolved in ebd5117
- **Complexity delta:** pubsubTopic.Shutdown 3 → 3, SimpleStore.Shutdown 3 → 3, ShardedStore.Shutdown 6 → 6
- **Description:** When `Shutdown` returns early because `ctx.Done()` fires mid-loop, it skips the `t.subscriptions = nil` assignment. Already-closed channels remain in `t.subscriptions`. Any subsequent `Close()` (which calls `Shutdown(context.Background())`) iterates the slice again and calls `close()` on a channel that is already closed, producing an unrecoverable panic. This is reachable via `PubSub.Shutdown(timedCtx)` followed by `PubSub.Close()`, or any code that shuts down a topic more than once. The same early-return bug exists in `SimpleStore.Shutdown` and `ShardedStore.Shutdown`, where a cancelled context leaves topics un-closed (subscriber channels never close → goroutine leak).
- **Recommended fix:**
  - In `pubsubTopic.Shutdown`, record which channels have already been closed and set `t.subscriptions = nil` before returning, or nilify closed entries inline:
    ```go
    func (t *pubsubTopic) Shutdown(ctx context.Context) {
        t.mu.Lock()
        defer t.mu.Unlock()
        remaining := t.subscriptions[:0]
        for _, sub := range t.subscriptions {
            select {
            case <-ctx.Done():
                remaining = append(remaining, sub)
            default:
                close(sub)
                t.observer.OnUnsubscribe(t.name)
            }
        }
        t.subscriptions = remaining
    }
    ```
  - Apply the same pattern to `SimpleStore.Shutdown` and `ShardedStore.Shutdown` so that a context-cancelled shutdown leaves the store in a consistent, re-closeable state.
  - Add a test: create a topic, call `Shutdown` with an already-cancelled context, then call `Close()` — expect no panic.

---

### 2. Duplicate message delivery race in `topicWithHistory.subscribeWithBuffer`

- **File(s):** `history.go:141-153`
- **Dimension(s):** Correctness
- **Priority:** High
- **Status:** Resolved in ca6e270
- **Complexity delta:** topicWithHistory.subscribeWithBuffer 3 → 1 (absorbed into new pubsubTopic.subscribeWithHistory at 3)
- **Description:** `subscribeWithBuffer` first calls `t.topic.subscribeWithBuffer(size)` (which acquires `topic.mu`, appends the channel to `subscriptions`, and releases the lock), then separately calls `t.history.GetAll()` to replay history. Between those two operations the channel is live and receiving publishes. Any message published in that window is appended to both the live channel **and** the history store; when history is replayed the subscriber receives the message a second time. This is not a theoretical race: under any moderate publish rate, a subscriber is guaranteed to see duplicates.
- **Recommended fix:** Replay history before adding the channel to the subscription list. This requires `topicWithHistory` to coordinate with `pubsubTopic` at a lower level — the cleanest fix is a new internal method on `pubsubTopic` that atomically creates the channel and returns a snapshot of history (or accepts history to pre-fill under the same lock):
  ```go
  // Held under topic.mu.Lock() to avoid the race window.
  func (t *pubsubTopic) subscribeWithBufferAndFill(size int, msgs []any) chan any {
      t.mu.Lock()
      defer t.mu.Unlock()
      t.observer.OnSubscribe(t.name)
      ch := make(chan any, size)
      for _, m := range msgs {
          select { case ch <- m: default: }
      }
      t.subscriptions = append(t.subscriptions, ch)
      return ch
  }
  ```
  Then `topicWithHistory.subscribeWithBuffer` becomes: fetch history, then call `subscribeWithBufferAndFill(size, history)`.

---

### 3. `time.After` timer leak and RLock held during blocking send in `PublishReliable`

- **File(s):** `topic.go:152-162`, `topic.go:112-133`
- **Dimension(s):** Correctness, Maintainability
- **Priority:** High
- **Status:** Resolved in e89a187 (timer leak fixed; RLock-during-blocking-send is a separate architectural concern captured in Finding 4)
- **Complexity delta:** pubsubTopic.PublishReliable 2 → 2
- **Description:** `PublishReliable` creates `time.After(100 * time.Millisecond)` inside the `sendFunc` closure that is called once per subscriber per message. In the happy path (channel has room), the timer is allocated but never stopped — it leaks until it fires 100 ms later. Under sustained publish rates with many subscribers, thousands of live timers accumulate. More critically, `publishWithSendFunc` holds `t.mu.RLock()` for the entire send loop, so a blocked reliable publish pins the read lock for up to N × 100 ms across N unresponsive subscribers, starving every concurrent Subscribe/Unsubscribe call for that topic during that window.
- **Recommended fix:** Replace `time.After` with `time.NewTimer` and stop it immediately after the select:
  ```go
  func(ch chan any, m any) bool {
      timer := time.NewTimer(100 * time.Millisecond)
      defer timer.Stop()
      select {
      case ch <- m:
          return true
      case <-timer.C:
          return false
      }
  }
  ```
  Additionally, consider releasing `t.mu.RLock()` before the blocking send or restructuring `PublishReliable` to snapshot the subscriber list and then send outside the lock to bound lock-hold time. The 100 ms constant should also be configurable (e.g., a `PublishReliableTimeout` topic option).

---

### 4. Observer callbacks invoked while holding `RLock`

- **File(s):** `topic.go:116-118`
- **Dimension(s):** Architecture, Maintainability
- **Priority:** Medium
- **Status:** Resolved in de29c8f (OnDrop moved outside lock; OnPublish merged into send loop, still inside lock to preserve complexity budget)
- **Complexity delta:** pubsubTopic.publishWithSendFunc 8 → 8
- **Description:** `publishWithSendFunc` calls `t.observer.OnPublish` and `t.observer.OnDrop` synchronously while holding `t.mu.RLock()`. A slow observer (e.g., the `otelpubsub` observer, which calls Prometheus and OTel APIs) stalls every publish on that topic until the observer returns. Any observer that attempts to subscribe or unsubscribe inside its callback would deadlock (write lock cannot be acquired while read lock is held in the same goroutine on non-reentrant mutexes). This tightly couples the observer's performance characteristics to the publish path.
- **Recommended fix:** Collect observer notifications outside the lock. Snapshot what needs to be reported before entering the lock, send the messages, then call observer methods after releasing the lock. For drop notifications specifically, the drop decision is made inside the lock but the `OnDrop` call can safely move outside.

---

### 5. Magic `"*"` wildcard topic literal

- **File(s):** `pubsub.go:265, 311, 314, 328, 329, 332, 340, 359, 360`
- **Dimension(s):** Maintainability, Inconsistencies
- **Priority:** Medium
- **Status:** Resolved in 859c06d
- **Complexity delta:** n/a (no logic change)
- **Description:** The wildcard topic name `"*"` is embedded as a string literal at nine sites across `pubsub.go`. It also appears in tests without a shared constant. If the sentinel value ever changes, or if a caller accidentally subscribes to a topic named `"*"` thinking it is an ordinary topic name, the behavior is surprising and the connection to the wildcard semantic is invisible. The public documentation on `SubscribeAll` mentions `"*"` but users reading `Topics()` output will not know what it means.
- **Recommended fix:** Export a named constant:
  ```go
  // WildcardTopic is the special topic name that receives all published messages.
  const WildcardTopic = "*"
  ```
  Replace all nine literal occurrences in `pubsub.go` and update the tests. Update `Topics()` documentation to mention `WildcardTopic`.

---

### 6. `Clear()` is not part of the `Store` interface

- **File(s):** `internal/topicstore/store.go:76, 177`
- **Dimension(s):** Inconsistencies, Architecture
- **Priority:** Medium
- **Status:** Resolved in 466ca02
- **Complexity delta:** n/a (interface change only)
- **Description:** Both `SimpleStore` and `ShardedStore` implement `Clear()` but the method is absent from the `Store` interface. Code that holds a `Store` value (including `pubSub` itself) cannot call `Clear()` without a type assertion. Currently `Clear()` is only reachable through concrete types, which means it either exists for test use or is accidentally omitted from the interface. The asymmetry will surprise contributors who see the method on both concrete types and assume the interface covers it.
- **Recommended fix:** Add `Clear()` to the `Store` interface, or — if it is exclusively for test use — move it to a separate `TestableStore` interface in a `_test.go` helper file and unexport it from production types.

---

### 7. `topicWithName` is a silent, fragile extension point

- **File(s):** `pubsub.go:281-291`
- **Dimension(s):** Architecture, Maintainability
- **Priority:** Medium
- **Status:** Resolved in c87efcd
- **Complexity delta:** setTopicName 1 → removed; initPubSub 2 → 2
- **Description:** The `topicWithName` internal interface and `setTopicName` helper allow `pubSub` to set the observer-visible name on a topic after creation. If a Topic implementation forgets to implement `setName`, the observer receives an empty string for all callbacks — a silent failure with no compile-time or runtime warning. Every Topic implementation (currently `pubsubTopic` and `topicWithHistory`) must remember to delegate `setName`. This is a maintainability trap for future Topic implementations.
- **Recommended fix:** Pass the topic name at construction time rather than as a post-creation side-effect. Change the `topicFunc` signature to `func(name string) Topic` so that the name flows into the topic through the factory. This eliminates the `topicWithName` interface entirely and makes naming a required part of the Topic construction contract.

---

### 8. `runFeedingLoop` accepts mutually-exclusive function parameters

- **File(s):** `pubsub.go:363-390`
- **Dimension(s):** Maintainability, Inconsistencies
- **Priority:** Medium
- **Status:** Resolved in efe3c3e
- **Complexity delta:** runFeedingLoop 22 → 16
- **Description:** `runFeedingLoop` accepts two function parameters — `publish` and `publishReliable` — where exactly one must be non-nil. Callers pass one and `nil` for the other. The implementation checks which one is set with an if/else. This is the Boolean parameter antipattern applied to functions: the caller must know the contract (only one should be set) and the function must guard against both being nil or both being set.
- **Recommended fix:** Replace with a single `publishFn func(topic string, msg ...any)` parameter and let the callers wrap their preferred publish variant:
  ```go
  func runFeedingLoop(ctx context.Context, f FeedingFunc, publishFn func(string, ...any)) { ... }

  // AddFeedingFunc:
  runFeedingLoop(ctx, f, func(topic string, msg ...any) { ps.Publish(topic, msg...) })

  // AddFeedingFuncReliable:
  runFeedingLoop(ctx, f, func(topic string, msg ...any) { ps.PublishReliable(topic, msg...) })
  ```

---

### 9. Naming asymmetry: `WithLockFreeHistoryOpt` vs `WithLockFreeHistory`

- **File(s):** `topic.go:283`, `topic_lockfree.go:5`
- **Dimension(s):** Inconsistencies
- **Priority:** Medium
- **Status:** Resolved in 2cedfff
- **Complexity delta:** n/a (rename only)
- **Description:** The `TopicOpt` for lock-free history is `WithLockFreeHistoryOpt` (with "Opt" suffix) while the equivalent `PubSub` `Opt` is `WithLockFreeHistory` (no suffix). The parallel pair for regular history is `WithHistory` / `WithHistorySize`, which uses a size distinction rather than an "Opt" suffix. Users composing options at both levels will encounter three different naming styles for equivalent concepts.
- **Recommended fix:** Align to `WithHistory` / `WithLockFreeHistory` at both the `TopicOpt` and `Opt` level. Since Go options are typed, name collisions between `TopicOpt` and `Opt` are impossible — the suffix "Opt" adds noise without disambiguating anything. Rename `WithLockFreeHistoryOpt` → `WithLockFreeHistory` (breaking change; bump minor version).

---

### 10. Tests do not call `t.Parallel()`

- **File(s):** `pubsub_test.go`, `topic_test.go`, `reliable_test.go`, `sharded_test.go`, `filter_test.go`, `lifecycle_test.go`, `observer_test.go`
- **Dimension(s):** Modern Practices
- **Priority:** Medium
- **Status:** Resolved in 7ad6cd0
- **Complexity delta:** n/a (test-only change)
- **Description:** The project's own testing rules (`.claude/rules/go-testing.md`) state: "call `t.Parallel()` in tests that are safe to run concurrently." None of the 40+ test functions in the main package do so, resulting in a purely sequential test suite. The individual tests are independent and create their own PubSub/Topic instances, so there are no shared-state barriers to parallelism. Sequential tests give slower CI feedback and hide concurrency-sensitive behaviour.
- **Recommended fix:** Add `t.Parallel()` as the first statement in every test function in the main package. Tests in subtests (`t.Run`) that create shared state should also call `t.Parallel()` on the subtest `*testing.T`. Run the suite with `-race` after the change to validate.

---

### 11. `FeedingFunc` does not implement `Feeder`

- **File(s):** `pubsub.go:65-66`, `pubsub.go:59-62`
- **Dimension(s):** Architecture, Modern Practices
- **Priority:** Low
- **Status:** Resolved in 254a915
- **Complexity delta:** n/a (one-line method addition)
- **Description:** `FeedingFunc` is a named function type `func() <-chan *EventTuple`. The `Feeder` interface requires a single method `Feed() <-chan *EventTuple`. These are structurally equivalent, but `FeedingFunc` does not implement `Feeder` — there is no `Feed()` method on `FeedingFunc`. This forces `PubSub` to expose two parallel sets of methods (`AddFeeder`/`AddFeedingFunc`, `AddFeederReliable`/`AddFeedingFuncReliable`) where one unified interface would suffice.
- **Recommended fix:** Add a `Feed()` method to `FeedingFunc`:
  ```go
  func (f FeedingFunc) Feed() <-chan *EventTuple { return f() }
  ```
  Then `AddFeedingFunc` becomes `AddFeeder(ctx, f)` where `f` is a `FeedingFunc` (which now satisfies `Feeder`). The `AddFeedingFunc` and `AddFeedingFuncReliable` methods can remain as convenience wrappers but delegate to `AddFeeder`/`AddFeederReliable`.

---

### 12. Unexplained magic number in `topicWithHistory.Subscribe` buffer sizing

- **File(s):** `history.go:132`
- **Dimension(s):** Maintainability
- **Priority:** Low
- **Status:** Resolved in 4b7d34c (documented rationale)
- **Complexity delta:** n/a (comment only)
- **Description:** `Subscribe()` returns `t.subscribeWithBuffer(t.history.Cap() * 2)`. The `* 2` multiplier has no explanation. The intent is presumably to have room for both the replayed history and a burst of live messages, but the exact ratio is arbitrary. If history capacity is large (e.g., 10 000), this creates a 20 000-element channel on every subscribe call, wasting memory in most scenarios.
- **Recommended fix:** Document the intent in a comment, or cap the buffer size: `min(t.history.Cap()*2, maxSubscribeBuffer)` with `maxSubscribeBuffer` as a configurable topic option. At minimum, add a one-line comment explaining why `* 2` is the chosen multiplier.

---

### 13. `SimpleStore.GetOrCreate` always acquires a write lock

- **File(s):** `internal/topicstore/store.go:47-61`
- **Dimension(s):** Maintainability, Modern Practices
- **Priority:** Low
- **Status:** Resolved in e0f3f6d (documented trade-off; double-check pattern raised complexity above budget)
- **Complexity delta:** SimpleStore.GetOrCreate 2 → 2 (complexity budget prevented fast-path implementation)
- **Description:** `SimpleStore.GetOrCreate` acquires `mu.Lock()` (exclusive) even on the common read path where the topic already exists. The `ShardedStore` implements the same operation with a double-checked locking pattern using `sync.Map` for a fast lock-free read path. For users who choose `SimpleStore` over `ShardedStore` because they have few topics, this difference is probably negligible — but it is inconsistent documentation about why you'd choose one store over the other.
- **Recommended fix:** Apply the same double-checked locking pattern as `ShardedStore` to `SimpleStore` — use `sync.Map` or a read-lock + recheck-under-write-lock pattern. Alternatively, update the documentation for `SimpleStore` to explicitly state that it always acquires an exclusive lock and is not optimized for high read concurrency on `GetOrCreate`.

---

## Priority Table

| Priority | Title | File(s) |
|----------|-------|---------|
| High | Double-close panic in `pubsubTopic.Shutdown` on context cancellation | `topic.go:222-236` |
| High | Duplicate message delivery race in `topicWithHistory.subscribeWithBuffer` | `history.go:141-153` |
| High | `time.After` timer leak and RLock held during blocking send | `topic.go:152-162`, `topic.go:112-133` |
| Medium | Observer callbacks invoked while holding `RLock` | `topic.go:116-118` |
| Medium | Magic `"*"` wildcard topic literal | `pubsub.go` (9 sites) |
| Medium | `Clear()` is not part of the `Store` interface | `internal/topicstore/store.go:76,177` |
| Medium | `topicWithName` is a silent, fragile extension point | `pubsub.go:281-291` |
| Medium | `runFeedingLoop` accepts mutually-exclusive function parameters | `pubsub.go:363-390` |
| Medium | Naming asymmetry: `WithLockFreeHistoryOpt` vs `WithLockFreeHistory` | `topic.go:283`, `topic_lockfree.go:5` |
| Medium | Tests do not call `t.Parallel()` | all `*_test.go` in main package |
| Low | `FeedingFunc` does not implement `Feeder` | `pubsub.go:59-66` |
| Low | Unexplained magic number in `topicWithHistory.Subscribe` buffer sizing | `history.go:132` |
| Low | `SimpleStore.GetOrCreate` always acquires a write lock | `internal/topicstore/store.go:47-61` |
