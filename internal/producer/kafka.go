package producer

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/quiby-ai/common/pkg/events"
	"github.com/quiby-ai/review-ingestor/config"
	"github.com/segmentio/kafka-go"
)

type Producer struct {
	w *kafka.Writer
}

func NewProducer(cfg config.KafkaConfig) *Producer {
	w := kafka.NewWriter(kafka.WriterConfig{
		Brokers:      cfg.Brokers,
		Balancer:     &kafka.Hash{},
		RequiredAcks: int(kafka.RequireAll),
		Async:        false,
	})
	return &Producer{w: w}
}

func (p *Producer) Close() error { return p.w.Close() }

func (p *Producer) PublishEvent(ctx context.Context, key []byte, envelope events.Envelope[any]) error {
	value, err := events.MarshalEnvelope(envelope)
	if err != nil {
		return fmt.Errorf("marshal envelope: %w", err)
	}

	// Convert envelope headers to Kafka headers
	kafkaHeaders := make([]kafka.Header, 0, len(envelope.KafkaHeaders()))
	for _, h := range envelope.KafkaHeaders() {
		kafkaHeaders = append(kafkaHeaders, kafka.Header{
			Key:   h.Key,
			Value: h.Value,
		})
	}

	msg := kafka.Message{
		Topic:   events.PipelineExtractCompleted,
		Key:     key,
		Value:   value,
		Headers: kafkaHeaders,
		Time:    time.Now(),
	}
	return p.w.WriteMessages(ctx, msg)
}

func (p *Producer) BuildEnvelope(event events.ExtractCompleted, sagaID string) events.Envelope[any] {
	return events.Envelope[any]{
		MessageID:  uuid.NewString(),
		SagaID:     sagaID,
		Type:       events.PipelineExtractCompleted,
		OccurredAt: time.Now().UTC(),
		Payload:    event,
		Meta: events.Meta{
			AppID:         event.AppID,
			SchemaVersion: events.SchemaVersionV1,
		},
	}
}
