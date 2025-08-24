# Raft (Educational Implementation)

A minimal educational Go implementation of core pieces of the Raft consensus algorithm: leader election, heartbeats, log replication, and simple persistence (current term, votedFor, log) across restarts. It uses gRPC + Protocol Buffers for node-to-node RPC.

WARNING: This is NOT production-ready Raft. It omits many safety & liveness features (log compaction / snapshots, membership changes, timing adaptivity, backoff, flow control, leadership transfer, client session semantics, linearizable read optimization, log application state machine, etc.).

## Features

- Fixed 5-node static cluster (IDs 1..5)
- Leader election with randomized timeouts
- Heartbeat / AppendEntries
- Log replication & basic majority commit
- Broadcast of arbitrary string messages (appended as log entries)
- Persistent state per node saved under `state/` (Protocol Buffer serialization)
- gRPC / Protobuf based RPC definitions (`proto/`)

## Project Layout

- `main.go` – entry point (start a node or send a broadcast)
- `cluster/` – node logic (election, replication, timers, helpers)
- `proto/` – generated protobuf / gRPC code (`raft.proto` not committed if missing)
- `state/` – per-node persisted state files (created at runtime)

## Requirements

- Go 1.24.1 (as declared in `go.mod`)

## Getting Started

1. Open 5 terminals and start each node (IDs 1..5):

   `go run . -id 1`
   `go run . -id 2`
   `go run . -id 3`
   `go run . -id 4`
   `go run . -id 5`

2. After a short period a leader is elected (watch logs: "becoming leader").

3. From any terminal (or a separate one) you can attempt to broadcast a message via (only succeeds on the leader):

   `go run . -id <leaderID> -msg "hello raft"`

4. Followers will receive the AppendEntries and replicate the new log entry.

## Observing Behavior

- Elections happen when followers' election timers expire without heartbeats.
- Persistent files allow a node restart to retain term / vote / log.
- Logs show replication success / failure and nextIndex backtracking.

## Limitations / TODO

- No application of committed log entries to a deterministic state machine.
- No client command routing (you must call the leader manually).
- No snapshots or log compaction.
- No dynamic cluster membership changes.
- No RPC retry / exponential backoff / connection pooling.
- Minimal error handling; many fatal logs.
- Lacks tests & metrics.

Potential future enhancements: apply log to a KV store, add snapshotting, implement linearizable reads, membership changes, and testing (Jepsen-style fault cases).

## Development Notes

Regenerate protobuf / gRPC code (requires `protoc` plus the Go plugins in your PATH) using the Taskfile:

`task generate`

One-time installation of the required protoc plugins if you have not already:

`go install google.golang.org/protobuf/cmd/protoc-gen-go@latest`
`go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest`

(If you previously used `buf generate`, note that `buf` has been removed in favor of a simple Taskfile wrapper around `protoc`.)

You can also run the app through Taskfile (these just expand to `go run .`):

Start a node: `task start -- -id 1`

Broadcast a message (leader only): `task start -- -id <leaderID> -msg "hello raft"`

## Cleaning State

Delete files under `state/` to reset all node histories (stop nodes first).

## License

Apache 2.0 - see `LICENSE`.

## References

- Diego Ongaro & John Ousterhout, "In Search of an Understandable Consensus Algorithm (Raft)"
- https://raft.github.io/

---

Educational use only; expect bugs and incomplete safety guarantees.
