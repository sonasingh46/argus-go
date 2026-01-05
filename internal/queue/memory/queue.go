// Package memory provides an in-memory implementation of the queue interfaces.
// This is useful for testing and development without external dependencies.
package memory

import (
	"context"
	"sync"

	"argus-go/internal/queue"
)

// Queue is an in-memory implementation of both Producer and Consumer interfaces.
// Messages are stored in a channel, allowing for simple pub/sub within a process.
// This implementation is safe for concurrent use.
type Queue struct {
	messages chan *queue.Message
	closed   bool
	mu       sync.RWMutex
	wg       sync.WaitGroup
}

// NewQueue creates a new in-memory queue with the specified buffer size.
// The buffer size determines how many messages can be queued before
// Publish blocks (or fails if the context is canceled).
func NewQueue(bufferSize int) *Queue {
	return &Queue{
		messages: make(chan *queue.Message, bufferSize),
	}
}

// Publish sends a message to the in-memory queue.
// This method blocks if the queue is full until space is available
// or the context is canceled.
func (q *Queue) Publish(ctx context.Context, msg *queue.Message) error {
	q.mu.RLock()
	if q.closed {
		q.mu.RUnlock()
		return ErrQueueClosed
	}
	q.mu.RUnlock()

	select {
	case q.messages <- msg:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Start begins consuming messages and calls the handler for each one.
// This blocks until the context is canceled or the queue is closed.
func (q *Queue) Start(ctx context.Context, handler queue.MessageHandler) error {
	q.wg.Add(1)
	defer q.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg, ok := <-q.messages:
			if !ok {
				// Channel closed
				return nil
			}
			// Process the message
			if err := handler(ctx, msg); err != nil {
				// In a real implementation, you might want to handle errors differently
				// (retry, dead letter queue, etc.). For the mock, we just log and continue.
				continue
			}
		}
	}
}

// Close shuts down the queue, stopping all consumers.
func (q *Queue) Close() error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return nil
	}

	q.closed = true
	close(q.messages)
	q.wg.Wait()
	return nil
}

// Len returns the current number of messages in the queue.
// Useful for testing to verify queue state.
func (q *Queue) Len() int {
	return len(q.messages)
}
