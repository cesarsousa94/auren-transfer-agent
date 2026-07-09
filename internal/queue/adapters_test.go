package queue

import (
	"context"
	"testing"

	"github.com/auren/auren-transfer-agent/internal/worker"
)

func TestNewQueueMemoryInfo(t *testing.T) {
	q, err := NewQueue(Options{Driver: MemoryDriver, MemoryCapacity: 2})
	if err != nil {
		t.Fatalf("NewQueue memory: %v", err)
	}
	defer q.Close()
	info := q.Info()
	if info.Driver != MemoryDriver || info.Mode != QueueModeLocal || info.Capacity != 2 {
		t.Fatalf("unexpected info: %#v", info)
	}
}

func TestRedisStreamsQueueUsesFoundationAdapter(t *testing.T) {
	q, err := NewRedisStreamsQueue(RedisStreamsOptions{Address: "redis://localhost:6379", Stream: "jobs", ConsumerGroup: "agents", Capacity: 1})
	if err != nil {
		t.Fatalf("NewRedisStreamsQueue: %v", err)
	}
	defer q.Close()
	job, err := worker.NewJob(worker.JobInput{SourceURL: "https://example.test/movie.mp4", DestinationKey: "movie.mp4"})
	if err != nil {
		t.Fatalf("NewJob: %v", err)
	}
	if err := q.Enqueue(context.Background(), job); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	info := q.Info()
	if info.Driver != RedisStreamsDriver || info.Mode != QueueModeFoundationAdapter || info.Name != "jobs" || info.ConsumerGroup != "agents" || info.Length != 1 {
		t.Fatalf("unexpected redis info: %#v", info)
	}
}

func TestRabbitMQQueueUsesFoundationAdapter(t *testing.T) {
	q, err := NewRabbitMQQueue(RabbitMQOptions{URL: "amqp://localhost/", QueueName: "jobs", Capacity: 3})
	if err != nil {
		t.Fatalf("NewRabbitMQQueue: %v", err)
	}
	defer q.Close()
	info := q.Info()
	if info.Driver != RabbitMQDriver || info.Mode != QueueModeFoundationAdapter || info.Endpoint != "amqp://localhost/" || info.Name != "jobs" || info.Capacity != 3 {
		t.Fatalf("unexpected rabbitmq info: %#v", info)
	}
}

func TestNATSQueueUsesFoundationAdapter(t *testing.T) {
	q, err := NewNATSQueue(NATSOptions{URL: "nats://localhost:4222", Subject: "jobs", QueueGroup: "agents", Capacity: 4})
	if err != nil {
		t.Fatalf("NewNATSQueue: %v", err)
	}
	defer q.Close()
	job, err := worker.NewJob(worker.JobInput{SourceURL: "https://example.test/movie.mp4", DestinationKey: "movie.mp4"})
	if err != nil {
		t.Fatalf("NewJob: %v", err)
	}
	if err := q.Enqueue(context.Background(), job); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	info := q.Info()
	if info.Driver != NATSDriver || info.Mode != QueueModeFoundationAdapter || info.Endpoint != "nats://localhost:4222" || info.Name != "jobs" || info.ConsumerGroup != "agents" || info.Length != 1 {
		t.Fatalf("unexpected nats info: %#v", info)
	}
}

func TestClusterQueueRequiresAdapterConfiguration(t *testing.T) {
	if _, err := NewQueue(Options{Driver: RedisStreamsDriver, MemoryCapacity: 1}); err == nil {
		t.Fatal("expected redis configuration error")
	}
	if _, err := NewQueue(Options{Driver: RabbitMQDriver, MemoryCapacity: 1}); err == nil {
		t.Fatal("expected rabbitmq configuration error")
	}
	if _, err := NewQueue(Options{Driver: NATSDriver, MemoryCapacity: 1}); err == nil {
		t.Fatal("expected nats configuration error")
	}
}
