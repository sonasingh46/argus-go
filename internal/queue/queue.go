// Package queue defines interfaces for message queue operations.
// This abstraction allows swapping implementations (Kafka, NATS, in-memory)
// without changing business logic.
package queue

import (
	"context"
)

// Message represents a message in the queue.
type Message struct {
	// Key is the partition key for ordering guarantees.
	Key []byte

	// Value is the message payload.
	Value []byte

	// Headers contains optional metadata.
	Headers map[string]string
}

// Producer defines the interface for publishing messages to a queue.
// Implementations must be safe for concurrent use.
type Producer interface {
	// Publish sends a message to the queue.
	// The key is used for partitioning - messages with the same key
	// are guaranteed to be processed in order.
	Publish(ctx context.Context, msg *Message) error

	// Close releases any resources held by the producer.
	Close() error
}

// MessageHandler is a callback function for processing consumed messages.
// Return an error to indicate processing failure (implementation may retry).
type MessageHandler func(ctx context.Context, msg *Message) error

// Consumer defines the interface for consuming messages from a queue.
type Consumer interface {
	// Start begins consuming messages and calls the handler for each one.
	// This is a blocking call that runs until the context is canceled
	// or an unrecoverable error occurs.
	Start(ctx context.Context, handler MessageHandler) error

	// Close stops consuming and releases any resources.
	Close() error
}
