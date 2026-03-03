package events

import (
	"context"
	"log/slog"

	"accountlink-platform-go/internal/domain"

	awsv2 "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sns/types"
)

type SNSPublisher struct {
	client   *sns.Client
	topicARN string
	logger   *slog.Logger
}

func NewSNSPublisher(client *sns.Client, topicARN string, logger *slog.Logger) *SNSPublisher {
	return &SNSPublisher{client: client, topicARN: topicARN, logger: logger}
}

func (p *SNSPublisher) Publish(ctx context.Context, event domain.PublishedEvent) error {
	attrs := map[string]types.MessageAttributeValue{
		"event_type": {
			DataType:    awsv2.String("String"),
			StringValue: awsv2.String(event.EventType),
		},
		"aggregate_type": {
			DataType:    awsv2.String("String"),
			StringValue: awsv2.String(event.AggregateType),
		},
		"aggregate_id": {
			DataType:    awsv2.String("String"),
			StringValue: awsv2.String(event.AggregateID),
		},
		"outbox_id": {
			DataType:    awsv2.String("String"),
			StringValue: awsv2.String(event.OutboxID.String()),
		},
	}

	resp, err := p.client.Publish(ctx, &sns.PublishInput{
		TopicArn:          awsv2.String(p.topicARN),
		Message:           awsv2.String(event.Payload),
		MessageAttributes: attrs,
	})
	if err != nil {
		return err
	}

	p.logger.Info("event_published",
		slog.String("target", "sns"),
		slog.String("sns.topic_arn", p.topicARN),
		slog.String("sns.message_id", awsv2.ToString(resp.MessageId)),
		slog.String("event.type", event.EventType),
		slog.String("event.outbox_id", event.OutboxID.String()),
		slog.String("aggregate.type", event.AggregateType),
		slog.String("aggregate.id", event.AggregateID),
		slog.Int("payload.size", len(event.Payload)),
	)

	return nil
}
