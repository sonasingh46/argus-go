// Package kafka provides Kafka-based implementations of the queue interfaces.
package kafka

import (
	"context"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"

	"argus-go/internal/config"
	"argus-go/internal/queue"
)

// Producer implements queue.Producer using Kafka.
type Producer struct {
	writer *kafka.Writer
}

// NewProducer creates a new Kafka producer.
func NewProducer(cfg *config.KafkaConfig) *Producer {
	writer := &kafka.Writer{
		Addr:         kafka.TCP(cfg.Brokers...),
		Topic:        cfg.Topic,
		Balancer:     &kafka.Hash{}, // Use key-based partitioning
		BatchTimeout: 10 * time.Millisecond,
		RequiredAcks: kafka.RequireOne,
	}

	return &Producer{
		writer: writer,
	}
}

// Publish sends a message to Kafka.
func (p *Producer) Publish(ctx context.Context, msg *queue.Message) error {
	kafkaMsg := kafka.Message{
		Key:   msg.Key,
		Value: msg.Value,
	}

	// Convert headers
	if len(msg.Headers) > 0 {
		kafkaMsg.Headers = make([]kafka.Header, 0, len(msg.Headers))
		for k, v := range msg.Headers {
			kafkaMsg.Headers = append(kafkaMsg.Headers, kafka.Header{
				Key:   k,
				Value: []byte(v),
			})
		}
	}

	if err := p.writer.WriteMessages(ctx, kafkaMsg); err != nil {
		return fmt.Errorf("failed to write message to kafka: %w", err)
	}

	return nil
}

// Close closes the Kafka writer.
func (p *Producer) Close() error {
	if p.writer != nil {
		return p.writer.Close()
	}
	return nil
}
