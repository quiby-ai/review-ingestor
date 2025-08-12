package consumer

import (
	"context"
	"log"

	"github.com/quiby-ai/review-ingestor/config"
	"github.com/quiby-ai/review-ingestor/internal/service"
	"github.com/segmentio/kafka-go"
)

type KafkaConsumer struct {
	reader *kafka.Reader
	svc    *service.IngestService
}

func NewKafkaConsumer(cfg config.KafkaConfig, svc *service.IngestService) *KafkaConsumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: cfg.Brokers,
		Topic:   cfg.TopicExtractReviews,
		GroupID: cfg.GroupID,
	})
	return &KafkaConsumer{reader: reader, svc: svc}
}

func (kc *KafkaConsumer) Run(ctx context.Context) error {
	for {
		m, err := kc.reader.ReadMessage(ctx)
		if err != nil {
			return err
		}
		log.Printf("received message: %s", string(m.Value))
		if err := kc.svc.Process(ctx, m.Value); err != nil {
			log.Printf("processing error: %v", err)
		}
	}
}

func (kc *KafkaConsumer) Close() error {
	if kc.reader != nil {
		return kc.reader.Close()
	}
	return nil
}
