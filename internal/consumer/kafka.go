package consumer

import (
	"context"
	"fmt"

	"github.com/quiby-ai/common/pkg/events"
	"github.com/quiby-ai/review-ingestor/config"
	"github.com/quiby-ai/review-ingestor/internal/service"
)

type IngestServiceProcessor struct {
	svc *service.IngestService
}

func (p *IngestServiceProcessor) Handle(ctx context.Context, payload any, sagaID string) error {
	if evt, ok := payload.(events.ExtractRequest); ok {
		return p.svc.Handle(ctx, evt, sagaID)
	}
	return fmt.Errorf("invalid payload type for preprocess service")
}

type KafkaConsumer struct {
	consumer *events.KafkaConsumer
}

func NewKafkaConsumer(cfg config.KafkaConfig, svc *service.IngestService) *KafkaConsumer {
	consumer := events.NewKafkaConsumer(cfg.Brokers, events.PipelineExtractRequest, cfg.GroupID)
	processor := &IngestServiceProcessor{svc: svc}
	consumer.SetProcessor(processor)
	return &KafkaConsumer{consumer: consumer}
}

func (kc *KafkaConsumer) Run(ctx context.Context) error {
	return kc.consumer.Run(ctx)
}

func (kc *KafkaConsumer) Close() error {
	return kc.consumer.Close()
}
