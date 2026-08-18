# webhook-ingest

A Go service that receives call-completion webhooks from our telephony provider,
stores them, and updates per-account call statistics.

It is in production. It is misbehaving.

## The incident

Last week operations filed this:

> Duplicate call records are showing up in the dashboard, and account
> call-counts are drifting higher than the actual number of calls. Calls are
> landing but their recordings never get marked processed — and there's nothing
> in the logs about it. On top of that, every time we deploy, whatever was in
> flight seems to just disappear.

Nobody has had time to look into it. That's your job.

**The test suite in this repo does not cover everything that's broken.**

## Your task

1. **Find and fix the defects.** Start from the symptoms above. Add tests that
   demonstrate each problem before you fix it.

2. **Make ingestion idempotent.** Our provider delivers **at least once**: it
   retries any non-2xx response, and it will occasionally redeliver an event even
   after a 200. The `event_id` field is stable across redeliveries. Ingesting the
   same event twice must not double-count anything.

   How you guarantee that is your call. Postgres and Redis are both running and
   connected. Pick an approach and be ready to defend it.

3. **Write a short document** (`SOLUTION.md`, about half a page):
   - What was broken, and why
   - Why you chose your deduplication strategy over the alternatives
   - What you would change if this had to handle 10,000 webhooks/sec

## Running it

```bash
docker compose up -d --build   # Postgres, Redis, and the service
curl localhost:8080/healthz    # -> ok
go test ./...                  # the visible test suite
```

`make reset` tears everything down, wipes the volumes, and starts fresh.

**Already running Postgres or Redis locally?** The published host ports are
overridable — copy `.env.example` to `.env` and change `APP_PORT`,
`POSTGRES_PORT`, `REDIS_PORT`, then point `DATABASE_URL` and `REDIS_ADDR` at
your chosen ports so `go test ./...` finds them.

Tests run against the Postgres started by Compose, so bring the stack up first.
They clean up after themselves and are safe to run repeatedly.

Migrations are plain SQL in `migrations/`, applied by Postgres on first start of
an empty volume. Add new ones as `002_*.sql`, `003_*.sql` and run `make reset`.

## The API

**`POST /webhooks/calls`**

```json
{
  "event_id":      "evt_01H8XK2M9P",
  "call_id":       "call_9f2ab31c",
  "account_id":    "acc_123",
  "status":        "completed",
  "duration_sec":  143,
  "recording_url": "https://recordings.example.com/9f2ab31c.wav",
  "occurred_at":   "2026-08-13T09:12:00Z"
}
```

`status` is one of `completed`, `failed`, `no_answer`.

**`GET /accounts/{account_id}/stats`** — returns the in-memory aggregate. The
durable copy of the same numbers lives in the `account_stats` table.

**`GET /healthz`**

## Layout

```
cmd/server/           entrypoint and wiring
internal/config/      environment configuration
internal/store/       Postgres repository
internal/stats/       in-memory per-account totals
internal/ingest/      webhook ingestion and processing
internal/httpapi/     routes and handlers
internal/redisclient/ Redis connection (connected; nothing uses it yet)
internal/testutil/    shared test setup
migrations/           schema
```

## Ground rules

- **Time box: 4 hours.** We would rather see two defects genuinely understood
  than four papered over. If you run out of time, say what you would have done
  next in `SOLUTION.md`.
- **AI tools are allowed.** Use whatever you normally use. We will spend 30
  minutes walking through your code together afterwards, so make sure you can
  explain why every change you kept is correct.
- Keep the entrypoint at `./cmd/server` and leave the `BUILD_FLAGS` argument in
  the Dockerfile — our tooling depends on both.
- The infrastructure works out of the box. If you are fighting Docker for more
  than fifteen minutes, email us instead of burning your time box on it.

## Submitting

Push to a **public GitHub repository** and send us the link. Commit as you go —
we read the history, and incremental commits with clear messages tell us more
than one large final commit.
