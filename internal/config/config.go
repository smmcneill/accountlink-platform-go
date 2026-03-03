package config

import (
	"fmt"
	"net/url"
	"time"

	"github.com/kelseyhightower/envconfig"
)

type (
	Config struct {
		Port             string
		DBDSN            string
		DBStartupMaxWait time.Duration
		DBStartupRetry   time.Duration
		EventTarget      string
		SNSTopicARN      string
		SNSEndpoint      string
		SNSRegion        string
		OutboxPollDelay  time.Duration
		OutboxPollBatch  int
	}

	envConfig struct {
		Port               string `envconfig:"PORT" default:"8080"`
		DBDSN              string `envconfig:"DB_DSN"`
		DBHost             string `envconfig:"DB_HOST" default:"localhost"`
		DBPort             int    `envconfig:"DB_PORT" default:"5444"`
		DBUser             string `envconfig:"DB_USER" default:"accountlink"`
		DBPassword         string `envconfig:"DB_PASSWORD" default:"accountlink"`
		DBName             string `envconfig:"DB_NAME" default:"accountlink"`
		DBSSLMode          string `envconfig:"DB_SSL_MODE" default:"disable"`
		DBStartupMaxWaitMS int    `envconfig:"DB_STARTUP_MAX_WAIT_MS" default:"300000"`
		DBStartupRetryMS   int    `envconfig:"DB_STARTUP_RETRY_MS" default:"5000"`
		EventTarget        string `envconfig:"EVENT_TARGET" default:"logging"`
		SNSTopicARN        string `envconfig:"ACCOUNTLINK_SNS_TOPIC_ARN"`
		SNSEndpoint        string `envconfig:"ACCOUNTLINK_SNS_ENDPOINT"`
		SNSRegion          string `envconfig:"ACCOUNTLINK_SNS_REGION" default:"us-east-1"`
		OutboxPollDelayMS  int    `envconfig:"OUTBOX_POLL_DELAY_MS" default:"10000"`
		OutboxPollBatch    int    `envconfig:"OUTBOX_POLL_BATCH_SIZE" default:"100"`
	}
)

func Load() (Config, error) {
	var parsed envConfig
	if err := envconfig.Process("", &parsed); err != nil {
		return Config{}, err
	}

	dbDSN := parsed.DBDSN
	if dbDSN == "" {
		dbDSN = buildPostgresDSN(parsed)
	}

	return Config{
		Port:             parsed.Port,
		DBDSN:            dbDSN,
		DBStartupMaxWait: time.Duration(parsed.DBStartupMaxWaitMS) * time.Millisecond,
		DBStartupRetry:   time.Duration(parsed.DBStartupRetryMS) * time.Millisecond,
		EventTarget:      parsed.EventTarget,
		SNSTopicARN:      parsed.SNSTopicARN,
		SNSEndpoint:      parsed.SNSEndpoint,
		SNSRegion:        parsed.SNSRegion,
		OutboxPollDelay:  time.Duration(parsed.OutboxPollDelayMS) * time.Millisecond,
		OutboxPollBatch:  parsed.OutboxPollBatch,
	}, nil
}

func buildPostgresDSN(cfg envConfig) string {
	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(cfg.DBUser, cfg.DBPassword),
		Host:   fmt.Sprintf("%s:%d", cfg.DBHost, cfg.DBPort),
		Path:   "/" + cfg.DBName,
	}

	q := u.Query()
	q.Set("sslmode", cfg.DBSSLMode)
	u.RawQuery = q.Encode()

	return u.String()
}
