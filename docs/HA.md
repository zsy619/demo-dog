# High-Availability Mode (Round 31)

demo-dog supports active/passive HA through WAL replication. Two nodes
run side-by-side: one **primary** accepts writes, one **follower** tails
the primary's WAL and applies every record locally. Failover is manual
— flip a flag, restart, and the follower becomes the new primary.

## Why not Raft?

Implementing strong consensus (Raft/Paxos) requires a state machine
library (`hashicorp/raft` is the canonical choice) which brings 12+
transitive dependencies and a 30-day-onboarding cliff. The stdlib-only
constraint forbids it.

For most demos and small teams, the simpler primary/follower model is
sufficient:

* 2-node HA is fine for many workloads
* The follower can serve read queries (`/api/v1/query`) so dashboards
  keep working when the primary is down
* Manual failover is acceptable for ops teams running a single
  observability stack

If you need true multi-leader consensus, run demo-dog behind etcd or
consul and use their leader-election service to flip the route.

## Setup

### Node A (initial primary)

```bash
dog-collector \
  --role=primary \
  --listen=:8088 \
  --snapshot-path=/var/lib/demo-dog/snap.bin \
  --wal-path=/var/lib/demo-dog/wal.bin
```

### Node B (initial follower)

```bash
dog-collector \
  --role=follower \
  --peer=node-a.internal:8088 \
  --listen=:8088 \
  --snapshot-path=/var/lib/demo-dog/snap.bin \
  --wal-path=/var/lib/demo-dog/wal.bin
```

The follower starts a goroutine that polls the primary every 2s:

```
GET http://node-a.internal:8088/replica/wal?from=<offset>
```

Each response is an NDJSON stream of WAL records. The follower
applies each record to its local engine using the same ingest path
the primary used.

### Health check

Both nodes expose the replica state on `/api/health`:

```json
{
  "status": "ok",
  "replica": {
    "role": "primary",
    "offset": 12345,
    "synced": 12345,
    "dropped": 0,
    "last_sync": "2026-08-19T10:00:00Z"
  }
}
```

A healthy follower shows `offset` close to (but always ≤) the
primary's. `dropped > 0` means the follower fell behind faster
than it could drain the buffer — investigate network/IO.

## Failover

When the primary is unresponsive:

1. Verify the primary is actually dead, not just slow. Check
   `/api/health` on the primary directly via an out-of-band
   connection.
2. On node B (the follower), restart with `--role=primary`. It
   resumes from the last offset it applied.
3. On node A (when it recovers), restart with `--role=follower
   --peer=node-b`. It catches up.

The new primary keeps accepting writes; clients don't notice the
switch as long as your service mesh or DNS follows.

### What you lose during failover

* **Best-effort mode (default):** writes received on the primary
  after the last successful replication poll but before the
  primary died. These are lost unless the primary's local WAL on
  `/var/lib/demo-dog/wal.bin` was fsynced (it is, on every append).
* **At-least-once mode (recommended for production):** enabled with
  `--replication-mode=at-least-once`. The primary retains every
  record until every follower has POSTed `/replica/ack`. A record
  is only GC'd when min(ack offsets) crosses it. This mode closes
  the data-loss window in exchange for a bounded retention buffer
  (default 100k records, configurable via `--retain-records`).

If the primary recovers and rejoins as a follower, those records
arrive and are applied; nothing is permanently lost in either mode.

### What you don't lose

* Snapshots taken on the primary before failure. The new primary
  loads its own snapshot on startup; the previous state is preserved
  across the failover.
* In-flight reads. The follower is a read replica; queries against
  it succeed even when the primary is down.

## Limits

* **Two nodes only.** The wire protocol assumes one primary and one
  follower. Adding more followers means more polling load on the
  primary; supported but not tested.
* **Best-effort, not at-least-once.** Records the follower hasn't
  yet pulled from the primary when the primary dies are lost.
  For at-least-once semantics, the follower would need to
  acknowledge offsets back to the primary and the primary would
  need to wait before fsyncing — that's the next round.
* **No automatic failover.** A monitoring system (your existing
  Prometheus + Alertmanager) watches `/api/health` and triggers
  the restart.

## Operational metrics

| Counter | Meaning | Alert threshold |
|---|---|---|
| `replica.offset` | last replicated offset | n/a |
| `replica.synced` | total records applied | n/a |
| `replica.dropped` | records the follower couldn't absorb | > 0 sustained |
| `replica.last_sync` | last successful poll | > 30s stale |

## When to upgrade to Raft

Move to etcd/Consul-based election when:

1. You need < 5 second failover (manual failover takes 30s+).
2. You have more than 2 nodes in the cluster.
3. You can't tolerate the small at-most-once window between
   primary death and follower catch-up.

For everything else, the stdlib HA mode is enough.
