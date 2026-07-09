# Cluster Foundation

Version `v0.1.36` completes the foundation Cluster epic without starting distributed coordination or any external service connection.

## Queue adapters

The queue package exposes one interface for local and future distributed queues:

```text
queue.ClusterQueue
queue.Info
queue.Options
queue.NewQueue
queue.NewRedisStreamsQueue
queue.NewRabbitMQQueue
queue.NewNATSQueue
```

Supported driver names:

```text
memory
redis_streams
rabbitmq
nats
```

Redis Streams, RabbitMQ and NATS adapters are offline-compilable foundation adapters in this version. They validate endpoint/name/group configuration and expose diagnostics, but they use the local bounded FIFO queue internally and do not open network connections.

## Agent registry

`internal/cluster` provides a local registry for technical Agent snapshots:

```text
cluster.Agent
cluster.Registry
cluster.LocalAgent
cluster.NewRegistry
cluster.Registry.Register
cluster.Registry.UpdateHeartbeat
cluster.Registry.List
```

The registry stores only technical identity, hostname, runtime version, status, capacity, active job count and metadata. It does not store Media Hub tenant, customer, subscription or catalog decisions.

## Load balancer

`cluster.SelectLeastLoaded` chooses the available agent with the lowest `active_jobs / capacity` ratio. Ties are deterministic by Agent ID.

## Leader election

`cluster.ElectLeader` chooses the available agent with the lowest fingerprint. This is deterministic and local; it is not a distributed lock.

## Failover

`cluster.PlanFailover` creates a mechanical plan that maps queued job IDs to available replacement Agent IDs. It does not mutate the registry, start jobs or decide whether a Media Hub workflow should be retried.
