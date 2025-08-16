package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	AppStore AppStoreConfig `mapstructure:"appstore"`
	HTTP     HTTPConfig     `mapstructure:"http"`
	Kafka    KafkaConfig    `mapstructure:"kafka"`
	Postgres PostgresConfig `mapstructure:"postgres"`
}

type AppStoreConfig struct {
	Referrer string `mapstructure:"referrer"`
	APIHost  string `mapstructure:"api_host"`
	APIPath  string `mapstructure:"api_path"`
	Limit    int    `mapstructure:"limit"`
}

type HTTPConfig struct {
	Timeout        time.Duration `mapstructure:"timeout"`
	MaxRetries     int           `mapstructure:"max_retries"`
	BackoffInitial time.Duration `mapstructure:"backoff_initial"`
	BackoffMax     time.Duration `mapstructure:"backoff_max"`
	UserAgents     []string      `mapstructure:"user_agents"`
}

type KafkaConfig struct {
	Brokers []string `mapstructure:"brokers"`
	GroupID string   `mapstructure:"group_id"`
}

type PostgresConfig struct {
	DSN string `mapstructure:"dsn"`
}

func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("toml")
	viper.AddConfigPath(".")

	setDefaults()

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	cfg.HTTP.Timeout = time.Duration(viper.GetInt("http.timeout_seconds")) * time.Second
	cfg.HTTP.BackoffInitial = time.Duration(viper.GetInt("http.backoff_initial_sec")) * time.Second
	cfg.HTTP.BackoffMax = time.Duration(viper.GetInt("http.backoff_max_sec")) * time.Second

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &cfg, nil
}

func setDefaults() {
	viper.SetDefault("appstore.referrer", "https://apps.apple.com/")
	viper.SetDefault("appstore.api_host", "https://amp-api-edge.apps.apple.com")
	viper.SetDefault("appstore.api_path", "v1/catalog/{country}/apps/{app_id}/reviews")
	viper.SetDefault("appstore.limit", 20)

	viper.SetDefault("http.timeout_seconds", 10)
	viper.SetDefault("http.max_retries", 3)
	viper.SetDefault("http.backoff_initial_sec", 1)
	viper.SetDefault("http.backoff_max_sec", 60)
	viper.SetDefault("http.user_agents", []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
		"Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1",
	})

	viper.SetDefault("kafka.brokers", []string{"localhost:9092"})
	viper.SetDefault("kafka.group_id", "ingestor")

	viper.SetDefault("postgres.dsn", "postgres://user:pass@localhost:5432/dbname?sslmode=disable")
}

func (c *Config) Validate() error {
	if err := c.AppStore.Validate(); err != nil {
		return fmt.Errorf("appstore config: %w", err)
	}
	if err := c.HTTP.Validate(); err != nil {
		return fmt.Errorf("http config: %w", err)
	}
	if err := c.Kafka.Validate(); err != nil {
		return fmt.Errorf("kafka config: %w", err)
	}
	if err := c.Postgres.Validate(); err != nil {
		return fmt.Errorf("postgres config: %w", err)
	}
	return nil
}

func (a *AppStoreConfig) Validate() error {
	if a.Referrer == "" {
		return fmt.Errorf("referrer is required")
	}
	if a.APIHost == "" {
		return fmt.Errorf("api_host is required")
	}
	if a.APIPath == "" {
		return fmt.Errorf("api_path is required")
	}
	if a.Limit <= 0 {
		return fmt.Errorf("limit must be positive")
	}
	return nil
}

func (h *HTTPConfig) Validate() error {
	if h.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}
	if h.MaxRetries < 0 {
		return fmt.Errorf("max_retries cannot be negative")
	}
	if h.BackoffInitial <= 0 {
		return fmt.Errorf("backoff_initial must be positive")
	}
	if h.BackoffMax <= 0 {
		return fmt.Errorf("backoff_max must be positive")
	}
	if h.BackoffMax < h.BackoffInitial {
		return fmt.Errorf("backoff_max must be greater than or equal to backoff_initial")
	}
	if len(h.UserAgents) == 0 {
		return fmt.Errorf("at least one user agent is required")
	}
	return nil
}

func (k *KafkaConfig) Validate() error {
	if len(k.Brokers) == 0 {
		return fmt.Errorf("at least one broker is required")
	}
	if k.GroupID == "" {
		return fmt.Errorf("group_id is required")
	}
	return nil
}

func (p *PostgresConfig) Validate() error {
	if p.DSN == "" {
		return fmt.Errorf("dsn is required")
	}
	return nil
}
