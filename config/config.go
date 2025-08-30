package config

import (
	"fmt"
	"time"

	"github.com/quiby-ai/review-ingestor/internal/logger"
	"github.com/spf13/viper"
)

type Config struct {
	AppStore AppStoreConfig
	HTTP     HTTPConfig
	Kafka    KafkaConfig
	Postgres PostgresConfig
	Logging  logger.Config
}

type AppStoreConfig struct {
	Referrer string
	APIHost  string
	APIPath  string
	Limit    int
}

type HTTPConfig struct {
	Timeout        time.Duration
	MaxRetries     int
	BackoffInitial time.Duration
	BackoffMax     time.Duration
	UserAgents     []string
}

type KafkaConfig struct {
	Brokers []string
	GroupID string
}

type PostgresConfig struct {
	DSN string
}

func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("toml")
	viper.AddConfigPath("/")

	viper.AutomaticEnv()

	viper.BindEnv("appstore.referrer", "APP_STORE_REFERRER")
	viper.BindEnv("appstore.api_path", "APP_STORE_API_PATH")
	viper.BindEnv("appstore.limit", "APP_STORE_LIMIT")

	viper.BindEnv("http.timeout_seconds", "HTTP_TIMEOUT_SECONDS")
	viper.BindEnv("http.max_retries", "HTTP_MAX_RETRIES")
	viper.BindEnv("http.backoff_initial_sec", "HTTP_BACKOFF_INITIAL_SEC")
	viper.BindEnv("http.backoff_max_sec", "HTTP_BACKOFF_MAX_SEC")
	viper.BindEnv("http.user_agents", "HTTP_USER_AGENTS")

	viper.BindEnv("kafka.brokers", "KAFKA_BROKERS")
	viper.BindEnv("kafka.group_id", "KAFKA_GROUP_ID")

	viper.BindEnv("PG_DSN")
	viper.BindEnv("APP_STORE_API_HOST")

	viper.BindEnv("logging.level", "LOG_LEVEL")
	viper.BindEnv("logging.format", "LOG_FORMAT")

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	config := &Config{
		AppStore: AppStoreConfig{
			Referrer: viper.GetString("appstore.referrer"),
			APIHost:  viper.GetString("APP_STORE_API_HOST"),
			APIPath:  viper.GetString("appstore.api_path"),
			Limit:    viper.GetInt("appstore.limit"),
		},
		Kafka: KafkaConfig{
			Brokers: viper.GetStringSlice("kafka.brokers"),
			GroupID: viper.GetString("kafka.group_id"),
		},
		Postgres: PostgresConfig{
			DSN: viper.GetString("PG_DSN"),
		},
		HTTP: HTTPConfig{
			Timeout:        viper.GetDuration("http.timeout_seconds"),
			MaxRetries:     viper.GetInt("http.max_retries"),
			BackoffInitial: viper.GetDuration("http.backoff_initial_sec"),
			BackoffMax:     viper.GetDuration("http.backoff_max_sec"),
			UserAgents:     viper.GetStringSlice("http.user_agents"),
		},
		Logging: logger.Config{
			Level:  getStringWithDefault("logging.level", "info"),
			Format: getStringWithDefault("logging.format", "json"),
		},
	}

	return config, nil
}

func getStringWithDefault(key, defaultValue string) string {
	if value := viper.GetString(key); value != "" {
		return value
	}
	return defaultValue
}
