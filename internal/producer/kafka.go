package producer

import (
	"context"

	"github.com/segmentio/kafka-go"
)

type KafkaProducer struct {
	writer *kafka.Writer
}

type KafkaConfig struct {
	Brokers             []string
	TopicPrepareReviews string
}

func NewKafkaProducer(cfg KafkaConfig) *KafkaProducer {
	return &KafkaProducer{
		writer: &kafka.Writer{
			Addr:     kafka.TCP(cfg.Brokers...),
			Topic:    cfg.TopicPrepareReviews,
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
