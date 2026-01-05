package memory

import "errors"

// ErrQueueClosed is returned when attempting to publish to a closed queue.
var ErrQueueClosed = errors.New("queue is closed")
