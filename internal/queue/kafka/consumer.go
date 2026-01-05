package kafka

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/segmentio/kafka-go"

	"argus-go/internal/config"
	"argus-go/internal/queue"
)

// Consumer implements queue.Consumer using Kafka.
type Consumer struct {
	reader *kafka.Reader
	logger *slog.Logger
}

// NewConsumer creates a new Kafka consumer.
func NewConsumer(cfg *config.KafkaConfig, logger *slog.Logger) *Consumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  cfg.Brokers,
		Topic:    cfg.Topic,
		GroupID:  cfg.ConsumerGroup,
		MinBytes: 1,
		MaxBytes: 10e6, // 10MB
	})

	return &Consumer{
		reader: reader,
		logger: logger,
	}
}

// Start begins consuming messages and calls the handler for each one.
func (c *Consumer) Start(ctx context.Context, handler queue.MessageHandler) error {
	c.logger.Info("starting kafka consumer",
		"topic", c.reader.Config().Topic,
		"group", c.reader.Config().GroupID,
	)

	for {
		select {
		case <-ctx.Done():
			c.logger.Info("kafka consumer stopping due to context cancellation")
			return ctx.Err()
		default:
		}

		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			c.logger.Error("failed to fetch message", "error", err)
			continue
		}

		// Convert Kafka message to queue.Message
		queueMsg := &queue.Message{
			Key:     msg.Key,
			Value:   msg.Value,
			Headers: make(map[string]string),
		}

		for _, h := range msg.Headers {
			queueMsg.Headers[h.Key] = string(h.Value)
		}

		// Process the message
		if err := handler(ctx, queueMsg); err != nil {
			c.logger.Error("failed to process message",
				"error", err,
				"partition", msg.Partition,
				"offset", msg.Offset,
			)
			// Continue processing other messages even if one fails
			continue
		}

		// Commit the message after successful processing
		if err := c.reader.CommitMessages(ctx, msg); err != nil {
			c.logger.Error("failed to commit message",
				"error", err,
				"partition", msg.Partition,
				"offset", msg.Offset,
			)
			return fmt.Errorf("failed to commit message: %w", err)
		}
	}
}

// Close closes the Kafka reader.
func (c *Consumer) Close() error {
	if c.reader != nil {
		return c.reader.Close()
	}
	return nil
}
