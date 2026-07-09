# Worker Engine Foundation

The Worker Engine foundation currently covers:

- **5.1 — Job Model**;
- **5.2 — Queue**;
- **5.3 — Worker**;
- **5.4 — Worker Pool**;
- **5.5 — Scheduler**;
- **5.6 — Dispatcher**;
- **5.7 — Retry**;
- **5.8 — Heartbeat**;
- **5.9 — Persistence**;
- **5.10 — REST API**.

This version can model jobs, enqueue them, claim them with workers, execute them through a handler, schedule polling ticks, summarize dispatcher cycles, mechanically retry failed jobs while attempts remain, build local heartbeat records, persist queue snapshots and expose worker REST route contracts. It still does not perform real downloads, uploads, resolver actions, remote heartbeat delivery or HTTP network listening.

## Job model

The canonical job model lives in `internal/worker`.

Current public contracts:

```text
worker.Job
worker.JobInput
worker.JobType
worker.JobStatus
worker.NewJob
worker.ValidateJob
worker.Job.Clone
worker.Job.WithStatus
worker.Job.WithAttemptStatus
worker.IsTerminalJobStatus
worker.IsSupportedJobType
worker.IsSupportedJobStatus
worker.SupportedJobStatuses
```

The foundation job type is:

```text
transfer
```

The foundation statuses are:

```text
pending
queued
running
succeeded
failed
```

## Queue

The canonical queue contract lives in `internal/queue`.

Current public contracts:

```text
queue.Queue
queue.MemoryQueue
queue.NewMemoryQueue
queue.MemoryDriver
queue.ErrQueueFull
queue.ErrQueueClosed
```

The memory queue is bounded, FIFO, concurrency-safe and process-local. It accepts only `pending` or `queued` jobs and stores them as `queued`.


## Persistence

Queue persistence lives in `internal/queue` and stores local snapshots as validated JSON.

Current persistence contracts:

```text
queue.PersistenceVersion
queue.Snapshot
queue.StoreResult
queue.FileStore
queue.DefaultPersistencePath
queue.NewFileStore
queue.FileStore.Ensure
queue.FileStore.Load
queue.FileStore.Save
queue.NewSnapshot
queue.ValidateSnapshot
queue.Restore
```

The canonical default path is derived from `runtime.data_dir`:

```text
./data/worker/queue.json
```

Writes are atomic: a private temporary file is written, chmodded and renamed over the final path. Malformed existing files are rejected rather than silently replaced.

## Worker

The worker contract lives in `internal/worker`.

Current execution contracts:

```text
worker.JobQueue
worker.Handler
worker.HandlerFunc
worker.HandlerResult
worker.RunResult
worker.WorkerOptions
worker.Worker
worker.NewWorker
worker.Worker.RunOnce
worker.NoopHandler
```

`RunOnce` polls the queue once. If no job is available, it returns an idle result without treating that as an error. When a job is available, the worker marks it as `running`, increments `attempt`, delegates to the handler and returns either `succeeded` or `failed` for that execution.

## Worker pool

The pool contract provides bounded local concurrency without starting background loops by itself.

Current pool contracts:

```text
worker.PoolOptions
worker.PoolStats
worker.Pool
worker.NewPool
worker.Pool.RunOnce
worker.Pool.Size
worker.Pool.WorkerIDs
worker.Pool.Stats
```

`Pool.RunOnce` asks each configured worker to poll once. This means one call can execute at most `worker.concurrency` queued jobs.

## Scheduler

The scheduler contract lives in `internal/scheduler`.

Current scheduler contracts:

```text
scheduler.Task
scheduler.RunResult
scheduler.FixedIntervalScheduler
scheduler.NewFixedInterval
scheduler.FixedIntervalScheduler.Interval
scheduler.FixedIntervalScheduler.RunOnce
scheduler.FixedIntervalScheduler.Start
```

Bootstrap wires a fixed-interval scheduler using `queue.poll_interval`, but does not start the scheduler loop yet. The scheduler task delegates to the dispatcher rather than calling the worker pool directly. The Agent remains foreground-only and exits after initialization in the foundation line.

## Dispatcher

The dispatcher contract lives in `internal/dispatcher`.

Current dispatcher contracts:

```text
dispatcher.PoolRunner
dispatcher.RetryQueue
dispatcher.DispatchResult
dispatcher.Options
dispatcher.Dispatcher
dispatcher.New
dispatcher.Dispatcher.RunOnce
```

`Dispatcher.RunOnce` executes one pool cycle, summarizes executed/succeeded/failed jobs and requeues failed jobs when the retry policy allows it.

## Retry

Retry is mechanical and attempts-based only. It does not decide whether a job is commercially valid or operationally desirable.

Current retry contracts:

```text
dispatcher.RetryDecision
dispatcher.RetryPolicy
dispatcher.AttemptsRetryPolicy
dispatcher.NewAttemptsRetryPolicy
dispatcher.NormalizeRetryReason
```

A failed job is eligible when:

```text
job.status == failed && job.attempt < job.max_attempts
```

Eligible jobs are requeued as `pending`; the queue stores them as `queued`. No exponential backoff or remote scheduling is applied yet.

## Heartbeat

The heartbeat foundation lives in `internal/heartbeat`.

Current heartbeat contracts:

```text
heartbeat.QueueStats
heartbeat.Input
heartbeat.Record
heartbeat.NewRecord
heartbeat.ValidateRecord
heartbeat.Record.Clone
heartbeat.StatusIdle
heartbeat.StatusReady
```

The heartbeat record is a local snapshot containing identity, runtime, worker and queue diagnostics. It is not sent to Media Hub yet.


## Worker REST API

Worker REST route contracts live in `internal/server`. The bootstrap registers them, but the Agent does not open a listening socket in this foundation line.

Current REST routes:

```text
GET /worker
GET /worker/jobs
POST /worker/jobs
```

Current REST contracts:

```text
server.WorkerAPIOptions
server.WorkerResponse
server.WorkerJobsResponse
server.CreateJobRequest
server.CreateJobResponse
server.WorkerHandler
server.WorkerJobsHandler
server.CreateWorkerJobHandler
server.WorkerRoutes
```

`POST /worker/jobs` validates a mechanical job, enqueues it and saves the queue snapshot when a queue persister is configured.

## Business-rule boundary

The Agent still does not decide what should be transferred, where media belongs, which customer owns a transfer or how a Media Hub workflow should behave.

`worker.Job` contains only mechanical transfer fields:

```text
id, external_id, type, status, priority, attempt, max_attempts, source_url, destination_key, headers, metadata, created_at, updated_at
```

Auren Media Hub remains responsible for producing the job payload and all business decisions.


## Cluster queue foundation — v0.1.36

The queue package now exposes a foundation cluster contract:

```text
queue.ClusterQueue
queue.Info
queue.Options
queue.NewQueue
queue.NewRedisStreamsQueue
queue.NewRabbitMQQueue
queue.NewNATSQueue
```

`memory` remains the default queue driver. `redis_streams`, `rabbitmq` and `nats` adapters validate their configuration and expose stable diagnostics, but they do not open external network connections in this foundation version. Internally they use the same bounded FIFO mechanics as the local queue so worker, dispatcher, scheduler and persistence contracts remain unchanged.

Supported driver names:

```text
memory
redis_streams
rabbitmq
nats
```
