package queue

import (
	"context"
	"fmt"
	"strings"

	"github.com/auren/auren-transfer-agent/internal/worker"
)

const (
	// RedisStreamsDriver is the foundation cluster queue driver name for Redis Streams.
	RedisStreamsDriver = "redis_streams"

	// RabbitMQDriver is the foundation cluster queue driver name for RabbitMQ.
	RabbitMQDriver = "rabbitmq"

	// NATSDriver is the foundation cluster queue driver name for NATS.
	NATSDriver = "nats"

	// QueueModeLocal means operations are handled by the local in-process queue.
	QueueModeLocal = "local"

	// QueueModeFoundationAdapter means a cluster adapter contract is present but no external connection is opened.
	QueueModeFoundationAdapter = "foundation_adapter"
)

// Options configures creation of a queue implementation.
type Options struct {
	Driver             string
	MemoryCapacity     int
	RedisAddress       string
	RedisStream        string
	RedisConsumerGroup string
	RabbitMQURL        string
	RabbitMQQueue      string
	NATSURL            string
	NATSSubject        string
	NATSQueueGroup     string
}

// Info is a stable diagnostic snapshot for a queue implementation.
type Info struct {
	Driver        string `json:"driver"`
	Mode          string `json:"mode"`
	Endpoint      string `json:"endpoint,omitempty"`
	Name          string `json:"name,omitempty"`
	ConsumerGroup string `json:"consumer_group,omitempty"`
	Capacity      int    `json:"capacity"`
	Length        int    `json:"length"`
}

// ClusterQueue extends Queue with foundation cluster diagnostics.
type ClusterQueue interface {
	Queue
	Info() Info
}

// NewQueue creates the configured queue implementation without opening external connections.
func NewQueue(options Options) (ClusterQueue, error) {
	driver := NormalizeDriver(options.Driver)
	switch driver {
	case MemoryDriver, "":
		memory, err := NewMemoryQueue(options.MemoryCapacity)
		if err != nil {
			return nil, err
		}
		return &InstrumentedQueue{Queue: memory, info: Info{Driver: MemoryDriver, Mode: QueueModeLocal, Capacity: memory.Capacity()}}, nil
	case RedisStreamsDriver:
		return NewRedisStreamsQueue(RedisStreamsOptions{Address: options.RedisAddress, Stream: options.RedisStream, ConsumerGroup: options.RedisConsumerGroup, Capacity: options.MemoryCapacity})
	case RabbitMQDriver:
		return NewRabbitMQQueue(RabbitMQOptions{URL: options.RabbitMQURL, QueueName: options.RabbitMQQueue, Capacity: options.MemoryCapacity})
	case NATSDriver:
		return NewNATSQueue(NATSOptions{URL: options.NATSURL, Subject: options.NATSSubject, QueueGroup: options.NATSQueueGroup, Capacity: options.MemoryCapacity})
	default:
		return nil, fmt.Errorf("unsupported queue driver %q", options.Driver)
	}
}

// NormalizeDriver returns the canonical queue driver name.
func NormalizeDriver(driver string) string {
	return strings.ToLower(strings.TrimSpace(driver))
}

// SupportedDrivers returns a defensive list of queue drivers known by the foundation.
func SupportedDrivers() []string {
	return []string{MemoryDriver, RedisStreamsDriver, RabbitMQDriver, NATSDriver}
}

// InstrumentedQueue wraps any Queue with diagnostic Info.
type InstrumentedQueue struct {
	Queue
	info Info
}

// Info returns queue diagnostics with live length/capacity values.
func (queue *InstrumentedQueue) Info() Info {
	if queue == nil || queue.Queue == nil {
		return Info{}
	}
	info := queue.info
	info.Length = queue.Len()
	info.Capacity = queue.Capacity()
	return info
}

// RedisStreamsOptions configures the foundation Redis Streams adapter.
type RedisStreamsOptions struct {
	Address       string
	Stream        string
	ConsumerGroup string
	Capacity      int
}

// RedisStreamsQueue is an offline-compilable foundation adapter for Redis Streams.
type RedisStreamsQueue struct {
	*MemoryQueue
	info Info
}

// NewRedisStreamsQueue creates a Redis Streams queue contract backed by local memory in v0.1.x.
func NewRedisStreamsQueue(options RedisStreamsOptions) (*RedisStreamsQueue, error) {
	address := strings.TrimSpace(options.Address)
	if address == "" {
		return nil, fmt.Errorf("redis streams address is required")
	}
	stream := strings.TrimSpace(options.Stream)
	if stream == "" {
		return nil, fmt.Errorf("redis stream is required")
	}
	group := strings.TrimSpace(options.ConsumerGroup)
	if group == "" {
		return nil, fmt.Errorf("redis consumer group is required")
	}
	memory, err := NewMemoryQueue(options.Capacity)
	if err != nil {
		return nil, err
	}
	return &RedisStreamsQueue{MemoryQueue: memory, info: Info{Driver: RedisStreamsDriver, Mode: QueueModeFoundationAdapter, Endpoint: address, Name: stream, ConsumerGroup: group, Capacity: memory.Capacity()}}, nil
}

// Info returns Redis Streams adapter diagnostics.
func (queue *RedisStreamsQueue) Info() Info {
	if queue == nil || queue.MemoryQueue == nil {
		return Info{}
	}
	info := queue.info
	info.Length = queue.Len()
	info.Capacity = queue.Capacity()
	return info
}

// RabbitMQOptions configures the foundation RabbitMQ adapter.
type RabbitMQOptions struct {
	URL       string
	QueueName string
	Capacity  int
}

// RabbitMQQueue is an offline-compilable foundation adapter for RabbitMQ.
type RabbitMQQueue struct {
	*MemoryQueue
	info Info
}

// NewRabbitMQQueue creates a RabbitMQ queue contract backed by local memory in v0.1.x.
func NewRabbitMQQueue(options RabbitMQOptions) (*RabbitMQQueue, error) {
	endpoint := strings.TrimSpace(options.URL)
	if endpoint == "" {
		return nil, fmt.Errorf("rabbitmq url is required")
	}
	name := strings.TrimSpace(options.QueueName)
	if name == "" {
		return nil, fmt.Errorf("rabbitmq queue is required")
	}
	memory, err := NewMemoryQueue(options.Capacity)
	if err != nil {
		return nil, err
	}
	return &RabbitMQQueue{MemoryQueue: memory, info: Info{Driver: RabbitMQDriver, Mode: QueueModeFoundationAdapter, Endpoint: endpoint, Name: name, Capacity: memory.Capacity()}}, nil
}

// Info returns RabbitMQ adapter diagnostics.
func (queue *RabbitMQQueue) Info() Info {
	if queue == nil || queue.MemoryQueue == nil {
		return Info{}
	}
	info := queue.info
	info.Length = queue.Len()
	info.Capacity = queue.Capacity()
	return info
}

// NATSOptions configures the foundation NATS adapter.
type NATSOptions struct {
	URL        string
	Subject    string
	QueueGroup string
	Capacity   int
}

// NATSQueue is an offline-compilable foundation adapter for NATS.
type NATSQueue struct {
	*MemoryQueue
	info Info
}

// NewNATSQueue creates a NATS queue contract backed by local memory in v0.1.x.
func NewNATSQueue(options NATSOptions) (*NATSQueue, error) {
	endpoint := strings.TrimSpace(options.URL)
	if endpoint == "" {
		return nil, fmt.Errorf("nats url is required")
	}
	subject := strings.TrimSpace(options.Subject)
	if subject == "" {
		return nil, fmt.Errorf("nats subject is required")
	}
	group := strings.TrimSpace(options.QueueGroup)
	if group == "" {
		return nil, fmt.Errorf("nats queue group is required")
	}
	memory, err := NewMemoryQueue(options.Capacity)
	if err != nil {
		return nil, err
	}
	return &NATSQueue{MemoryQueue: memory, info: Info{Driver: NATSDriver, Mode: QueueModeFoundationAdapter, Endpoint: endpoint, Name: subject, ConsumerGroup: group, Capacity: memory.Capacity()}}, nil
}

// Info returns NATS adapter diagnostics.
func (queue *NATSQueue) Info() Info {
	if queue == nil || queue.MemoryQueue == nil {
		return Info{}
	}
	info := queue.info
	info.Length = queue.Len()
	info.Capacity = queue.Capacity()
	return info
}

var _ Queue = (*InstrumentedQueue)(nil)
var _ Queue = (*RedisStreamsQueue)(nil)
var _ Queue = (*RabbitMQQueue)(nil)
var _ Queue = (*NATSQueue)(nil)
var _ ClusterQueue = (*InstrumentedQueue)(nil)
var _ ClusterQueue = (*RedisStreamsQueue)(nil)
var _ ClusterQueue = (*RabbitMQQueue)(nil)
var _ ClusterQueue = (*NATSQueue)(nil)

func enqueueSnapshot(ctx context.Context, target Queue, jobs []worker.Job) error {
	for _, job := range jobs {
		if err := target.Enqueue(ctx, job); err != nil {
			return err
		}
	}
	return nil
}
