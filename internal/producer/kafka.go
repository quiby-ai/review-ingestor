package producer

import (
	"context"

	"github.com/quiby-ai/common/pkg/events"
	"github.com/quiby-ai/review-ingestor/config"
)

type Producer struct {
	producer *events.KafkaProducer
}

func NewProducer(cfg config.KafkaConfig) *Producer {
	producer := events.NewKafkaProducer(cfg.Brokers)
	return &Producer{producer: producer}
}

func (p *Producer) Close() error {
	return p.producer.Close()
}

func (p *Producer) PublishEvent(ctx context.Context, key []byte, envelope events.Envelope[any]) error {
	return p.producer.PublishEvent(ctx, key, envelope)
}

func (p *Producer) BuildEnvelope(event events.ExtractCompleted, sagaID string) events.Envelope[any] {
	envelope := events.BuildEnvelope(event, events.PipelineExtractCompleted, sagaID)
	envelope.Meta.AppID = event.AppID

	return envelope
}
