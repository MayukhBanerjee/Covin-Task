# SOLUTION.md

## 1. What was broken, and why

When I read the ops report, I traced each reported symptom directly to its root cause in the code. Here is what was happening at runtime and how I fixed it:

### Symptom A: "Duplicate call records and account call-counts drifting higher"
* **What I found:** The ingestion flow in `internal/ingest/service.go` relied on a two-step check: it queried `EventExists(ctx, eventID)` and, if false, proceeded to `InsertEvent(...)` and `IncrementAccountStats(...)`. Under at-least-once delivery or network retries, concurrent webhook deliveries of the same `event_id` both passed the `SELECT` query before either had committed the `INSERT` (a classic TOCTOU race condition). Because the `events` table also lacked a `UNIQUE` constraint on `event_id`, both transactions succeeded, inserting duplicate event rows and double-incrementing `account_stats`.
* **How I fixed it:**
  1. I added migration `migrations/002_event_id_unique.sql` to enforce `UNIQUE (event_id)` at the database level.
  2. I replaced the separate `SELECT` + `INSERT` with an atomic `InsertEventIdempotent` method using `INSERT INTO events ... ON CONFLICT (event_id) DO NOTHING`.
  3. In `Ingest()`, I inspect the query's affected rows. If 0 rows were inserted, I immediately log the duplicate and return `nil` (HTTP 200), preventing any duplicate stats increment or call upsert.
  4. I wrote two regression tests: `TestDuplicateDeliveryDoesNotDoubleCountStats` (for sequential retries) and `TestConcurrentDuplicateDeliveryIsIdempotent` (for parallel racing deliveries).

### Symptom B: Data race in `stats.Cache.Record()`
* **What I found:** While inspecting the in-memory cache in `internal/stats/cache.go`, I noticed `Cache` declared a `sync.RWMutex`, but `Record()` mutated the map and pointer fields without acquiring the lock at all. Under concurrent traffic, this was a data race that caused lost updates and memory corruption.
* **How I fixed it:** I added `c.mu.Lock()` and `defer c.mu.Unlock()` to `Record()`, and wrote `TestCacheRecordConcurrentIsSafe` which reproduces the race under `go test -race` when unpatched.

### Symptom C: "Recordings never get marked processed — and nothing in the logs"
* **What I found:** In `internal/ingest/service.go`, the background goroutine `go func() { s.processRecording(ctx, rec) }()` was passed the HTTP request's `ctx`. In Go's `net/http`, request contexts are cancelled immediately after the handler finishes writing the response. By the time `processRecording` finished its 50ms simulated work, the context was already dead. `s.store.MarkRecordingProcessed(ctx, ...)` failed with `context canceled`, and the error was silently swallowed in an empty `if err != nil { // TODO: handle }` block.
* **How I fixed it:** I passed an independent `context.Background()` to the background worker, and added structured error logging if the recording step fails. I added `TestRecordingIsMarkedProcessed` to verify that `recording_processed` is successfully updated in Postgres.

### Symptom D: "Every time we deploy, whatever was in flight seems to just disappear"
* **What I found:** In `cmd/server/main.go`, the graceful shutdown routine called `srv.Shutdown(shutdownCtx)` to drain active HTTP connections, but returned immediately without waiting for in-flight background goroutines. Any recording processing job running during a deployment was killed abruptly when the process exited.
* **How I fixed it:** I added a `sync.WaitGroup` to `ingest.Service` to track active background workers. When spawning a recording task, I increment the waitgroup and decrement on completion. I added `Service.Shutdown()`, which is called right after `srv.Shutdown()` in `main.go` to cleanly drain all background work before process termination.

---

## 2. Why I chose this deduplication strategy over alternatives

I chose **Postgres atomic `INSERT ... ON CONFLICT (event_id) DO NOTHING` backed by a database `UNIQUE` constraint**.

Here is why I selected this approach and why I rejected the alternatives:

1. **Why Postgres `ON CONFLICT` was my primary choice:**
   - **Zero Distributed State & True Atomicity:** It uses the existing primary storage without introducing dual-write failure modes. Postgres guarantees row-level atomicity under snapshot isolation; concurrent requests for the same `event_id` are serialized at the constraint level.
   - **Durability Guarantee:** Once written, deduplication state survives server restarts, database failovers, and redeployments.
   - **Cost Efficiency:** It eliminates the separate `SELECT` query, performing insertion and deduplication in a single database round-trip for the common path.

2. **Why I rejected Redis `SET key NX` as the primary source of truth:**
   - **Distributed Dual-Write Problem:** If Redis `SET NX` succeeds but the subsequent Postgres insert fails (e.g. database timeout or constraint error), the event key is locked in Redis but missing in Postgres. When the webhook provider retries, Redis flags it as a duplicate, and the event is permanently lost.
   - **Durability Limits:** Redis is an in-memory cache. If Redis restarts or evicts keys under memory pressure, duplicate webhooks would bypass the deduplication filter.
   *Note: In a high-throughput scenario, Redis `SET NX` makes an excellent non-authoritative caching fence in front of Postgres, but Postgres must remain the authoritative source of truth.*

3. **Why I rejected Postgres Advisory Locks (`pg_advisory_xact_lock`):**
   - Advisory locks require hashing the `event_id` into a 64-bit integer (creating hash collision risks) and hold lock resources throughout transaction duration, adding unnecessary lock contention compared to a native unique constraint.

---

## 3. What I would change if this had to handle 10,000 webhooks/second

At 10,000 requests/second (~600,000 RPM), synchronous database writes per HTTP request will saturate connection pools and disk I/O. Here is the architecture I would implement:

### 1. Asynchronous Ingestion via Message Queue (Kafka / NATS JetStream)
- The HTTP ingest endpoint should only do two things: validate payload schema and publish the raw event to a partitioned message stream (partitioned by `account_id`), then immediately return `202 Accepted` or `200 OK`.
- This decouples provider webhook ingestion latency (<5ms) from database processing and absorbs sudden traffic spikes without dropping requests.

### 2. High-Throughput Batch Deduplication & Storage
- Worker pools consuming from the queue would process events in micro-batches (e.g. 500 events per batch) using multi-row `INSERT ... ON CONFLICT DO NOTHING`.
- A durable Redis Cluster with `SET event_id NX EX 86400` can sit ahead of the workers as a fast-path filter, filtering out 99%+ of redeliveries before they ever generate database queries.

### 3. Solving Hot-Row Contention on `account_stats`
- At 10k rps, updating `account_stats` row-by-row causes severe row-lock contention for large accounts.
- I would replace immediate row updates with an append-only event log pattern or use Redis `HINCRBY` / `INCRBY` counters in memory, with background workers periodically flushing aggregate diffs to Postgres in batches.

### 4. Connection Pooling and DB Scaling
- Place **PgBouncer** in transaction-pooling mode between workers and Postgres to multiplex tens of thousands of application connections into a lean pool of server connections.
- Route read-heavy dashboard and stats queries (`GET /accounts/{id}/stats`) to read replicas or a distributed cache instead of the primary transactional DB.
