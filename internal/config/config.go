package config

import (
	"time"

	"github.com/kelseyhightower/envconfig"
)

type (
	Config struct {
		Port            string
		DBDSN           string
		EventTarget     string
		SNSTopicARN     string
		SNSEndpoint     string
		SNSRegion       string
		OutboxPollDelay time.Duration
		OutboxPollBatch int
	}

	envConfig struct {
		Port              string `envconfig:"PORT" default:"8080"`
		DBDSN             string `envconfig:"DB_DSN" default:"postgres://accountlink:accountlink@localhost:5444/accountlink?sslmode=disable"`
		EventTarget       string `envconfig:"EVENT_TARGET" default:"logging"`
		SNSTopicARN       string `envconfig:"ACCOUNTLINK_SNS_TOPIC_ARN"`
		SNSEndpoint       string `envconfig:"ACCOUNTLINK_SNS_ENDPOINT"`
		SNSRegion         string `envconfig:"ACCOUNTLINK_SNS_REGION" default:"us-east-1"`
		OutboxPollDelayMS int    `envconfig:"OUTBOX_POLL_DELAY_MS" default:"10000"`
		OutboxPollBatch   int    `envconfig:"OUTBOX_POLL_BATCH_SIZE" default:"100"`
	}
)

func Load() (Config, error) {
	var parsed envConfig
	if err := envconfig.Process("", &parsed); err != nil {
		return Config{}, err
	}

	return Config{
		Port:            parsed.Port,
		DBDSN:           parsed.DBDSN,
		EventTarget:     parsed.EventTarget,
		SNSTopicARN:     parsed.SNSTopicARN,
		SNSEndpoint:     parsed.SNSEndpoint,
		SNSRegion:       parsed.SNSRegion,
		OutboxPollDelay: time.Duration(parsed.OutboxPollDelayMS) * time.Millisecond,
		OutboxPollBatch: parsed.OutboxPollBatch,
	}, nil
}
