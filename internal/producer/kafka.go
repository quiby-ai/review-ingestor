package producer

import (
	"context"

	"github.com/quiby-ai/common/pkg/events"
	"github.com/quiby-ai/review-ingestor/config"
	"github.com/segmentio/kafka-go"
)

type KafkaProducer struct {
	writer *kafka.Writer
}

func NewKafkaProducer(cfg config.KafkaConfig) *KafkaProducer {
	return &KafkaProducer{
		writer: &kafka.Writer{
			Addr:     kafka.TCP(cfg.Brokers...),
			Topic:    events.PipelineExtractCompleted,
			Balancer: &kafka.LeastBytes{},
		},
	}
}

func (p *KafkaProducer) Publish(ctx context.Context, payload []byte) error {
	return p.writer.WriteMessages(ctx, kafka.Message{
		Value: payload,
	})
}

func (p *KafkaProducer) Close() error {
	if p.writer != nil {
		return p.writer.Close()
	}
	return nil
}
