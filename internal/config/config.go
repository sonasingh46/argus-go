// Package config provides configuration loading and management for ArgusGo.
// It supports loading configuration from YAML files with environment variable overrides.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// StorageMode represents the storage backend mode.
type StorageMode string

const (
	// StorageModeMemory uses in-memory implementations for all storage.
	StorageModeMemory StorageMode = "memory"
	// StorageModeStorage uses real storage backends (Kafka, Redis, PostgreSQL).
	StorageModeStorage StorageMode = "storage"
)

// IsValid returns true if the storage mode is valid.
func (m StorageMode) IsValid() bool {
	return m == StorageModeMemory || m == StorageModeStorage
}

// Config represents the complete application configuration.
type Config struct {
	Storage  StorageConfig  `yaml:"storage"`
	Server   ServerConfig   `yaml:"server"`
	Kafka    KafkaConfig    `yaml:"kafka"`
	Redis    RedisConfig    `yaml:"redis"`
	Postgres PostgresConfig `yaml:"postgres"`
	Logger   LoggerConfig   `yaml:"logger"`
}

// StorageConfig holds the storage mode configuration.
type StorageConfig struct {
	Mode StorageMode `yaml:"mode"`
}

// UseMemory returns true if in-memory storage should be used.
func (c *StorageConfig) UseMemory() bool {
	return c.Mode == StorageModeMemory
}

// UseStorage returns true if real storage backends should be used.
func (c *StorageConfig) UseStorage() bool {
	return c.Mode == StorageModeStorage
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Host         string        `yaml:"host"`
	Port         int           `yaml:"port"`
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
	IdleTimeout  time.Duration `yaml:"idle_timeout"`
}

// KafkaConfig holds Kafka connection and topic settings.
type KafkaConfig struct {
	Brokers        []string `yaml:"brokers"`
	Topic          string   `yaml:"topic"`
	ConsumerGroup  string   `yaml:"consumer_group"`
	PartitionCount int      `yaml:"partition_count"`
}

// RedisConfig holds Redis connection settings.
type RedisConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

// PostgresConfig holds PostgreSQL connection settings.
type PostgresConfig struct {
	Host         string `yaml:"host"`
	Port         int    `yaml:"port"`
	User         string `yaml:"user"`
	Password     string `yaml:"password"`
	Database     string `yaml:"database"`
	SSLMode      string `yaml:"ssl_mode"`
	MaxOpenConns int32  `yaml:"max_open_conns"`
	MaxIdleConns int32  `yaml:"max_idle_conns"`
}

// LoggerConfig holds logging settings.
type LoggerConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"` // "json" or "text"
}

// Load reads configuration from the specified YAML file path.
// Returns an error if the file cannot be read or parsed.
func Load(path string) (*Config, error) {
	// Clean the path to prevent path traversal attacks
	cleanPath := filepath.Clean(path)
	data, err := os.ReadFile(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Apply defaults for any unset values
	applyDefaults(cfg)

	return cfg, nil
}

// applyDefaults sets sensible default values for configuration fields
// that are not explicitly set in the config file.
func applyDefaults(cfg *Config) {
	// Storage defaults
	if cfg.Storage.Mode == "" {
		cfg.Storage.Mode = StorageModeMemory
	}

	// Server defaults
	if cfg.Server.Host == "" {
		cfg.Server.Host = "0.0.0.0"
	}
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8080
	}
	if cfg.Server.ReadTimeout == 0 {
		cfg.Server.ReadTimeout = 10 * time.Second
	}
	if cfg.Server.WriteTimeout == 0 {
		cfg.Server.WriteTimeout = 10 * time.Second
	}
	if cfg.Server.IdleTimeout == 0 {
		cfg.Server.IdleTimeout = 120 * time.Second
	}

	// Kafka defaults
	if len(cfg.Kafka.Brokers) == 0 {
		cfg.Kafka.Brokers = []string{"localhost:9092"}
	}
	if cfg.Kafka.Topic == "" {
		cfg.Kafka.Topic = "argus-events"
	}
	if cfg.Kafka.ConsumerGroup == "" {
		cfg.Kafka.ConsumerGroup = "argus-processor"
	}
	if cfg.Kafka.PartitionCount == 0 {
		cfg.Kafka.PartitionCount = 32
	}

	// Redis defaults
	if cfg.Redis.Host == "" {
		cfg.Redis.Host = "localhost"
	}
	if cfg.Redis.Port == 0 {
		cfg.Redis.Port = 6379
	}

	// Postgres defaults
	if cfg.Postgres.Host == "" {
		cfg.Postgres.Host = "localhost"
	}
	if cfg.Postgres.Port == 0 {
		cfg.Postgres.Port = 5432
	}
	if cfg.Postgres.SSLMode == "" {
		cfg.Postgres.SSLMode = "disable"
	}
	if cfg.Postgres.MaxOpenConns == 0 {
		cfg.Postgres.MaxOpenConns = 25
	}
	if cfg.Postgres.MaxIdleConns == 0 {
		cfg.Postgres.MaxIdleConns = 5
	}

	// Logger defaults
	if cfg.Logger.Level == "" {
		cfg.Logger.Level = "info"
	}
	if cfg.Logger.Format == "" {
		cfg.Logger.Format = "json"
	}
}

// Address returns the full server address in host:port format.
func (c *ServerConfig) Address() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// DSN returns the PostgreSQL connection string.
func (c *PostgresConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Database, c.SSLMode,
	)
}

// RedisAddr returns the Redis address in host:port format.
func (c *RedisConfig) RedisAddr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}
