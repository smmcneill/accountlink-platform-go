package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port            string
	DBDSN           string
	EventTarget     string
	SNSTopicARN     string
	SNSEndpoint     string
	SNSRegion       string
	OutboxPollDelay time.Duration
	OutboxPollBatch int
}

func Load() Config {
	return Config{
		Port:            getOrDefault("PORT", "8080"),
		DBDSN:           getOrDefault("DB_DSN", "postgres://accountlink:accountlink@localhost:5444/accountlink?sslmode=disable"),
		EventTarget:     getOrDefault("EVENT_TARGET", "logging"),
		SNSTopicARN:     os.Getenv("ACCOUNTLINK_SNS_TOPIC_ARN"),
		SNSEndpoint:     os.Getenv("ACCOUNTLINK_SNS_ENDPOINT"),
		SNSRegion:       getOrDefault("ACCOUNTLINK_SNS_REGION", "us-east-1"),
		OutboxPollDelay: time.Duration(getIntOrDefault("OUTBOX_POLL_DELAY_MS", 10000)) * time.Millisecond,
		OutboxPollBatch: getIntOrDefault("OUTBOX_POLL_BATCH_SIZE", 100),
	}
}

func getOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getIntOrDefault(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return parsed
}
