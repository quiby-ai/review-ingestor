package consumer

import (
	"context"
	"fmt"

	"github.com/quiby-ai/common/pkg/events"
	"github.com/quiby-ai/review-ingestor/config"
	"github.com/quiby-ai/review-ingestor/internal/logger"
	"github.com/quiby-ai/review-ingestor/internal/service"
)

type IngestServiceProcessor struct {
	svc *service.IngestService
}

func (p *IngestServiceProcessor) Handle(ctx context.Context, payload any, sagaID string) error {
	ctx = logger.WithSagaID(ctx, sagaID)

	logger.Debug(ctx, "Kafka message received", "saga_id", sagaID)

	if evt, ok := payload.(events.ExtractRequest); ok {
		ctx = logger.WithAppID(ctx, evt.AppID)
		logger.LogEvent(ctx, "kafka.message.decoded", "success", "app_id", evt.AppID)

		err := p.svc.Handle(ctx, evt, sagaID)
		if err != nil {
			logger.LogEvent(ctx, "kafka.message.processed", "failed")
			return err
		}

		logger.LogEvent(ctx, "kafka.message.processed", "success")
		return nil
	}

	logger.LogEvent(ctx, "kafka.message.decoded", "failed", "reason", "invalid_payload_type")
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
