package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config is the top-level application configuration.
type Config struct {
	Memobird MemobirdConfig `mapstructure:"memobird"`
	Storage  StorageConfig  `mapstructure:"storage"`
}

// MemobirdConfig contains configuration for the Memobird API client.
type MemobirdConfig struct {
	AccessKey  string `mapstructure:"access_key"`
	DeviceID   string `mapstructure:"device_id"`
	UserID     int    `mapstructure:"user_id"`
	BaseURL    string `mapstructure:"base_url"`
	TimeoutSec int    `mapstructure:"timeout_sec"`
}

// StorageConfig contains configuration for the storage layer.
type StorageConfig struct {
	DBPath string `mapstructure:"db_path"`
}

// Timeout returns the HTTP request timeout duration, defaulting to 30 seconds.
func (m *MemobirdConfig) Timeout() time.Duration {
	if m.TimeoutSec <= 0 {
		return 30 * time.Second
	}
	return time.Duration(m.TimeoutSec) * time.Second
}

// GetBaseURL returns the Memobird API base URL, defaulting to the official API endpoint.
func (m *MemobirdConfig) GetBaseURL() string {
	if m.BaseURL == "" {
		return "http://open.memobird.cn"
	}
	return m.BaseURL
}

// Load reads the configuration from a file or environment variables.
// If configPath is empty, it looks for config.yaml in default locations.
func Load(configPath string) (*Config, error) {
	v := viper.New()

	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
		v.AddConfigPath("/etc/memobird/")
	}

	v.SetEnvPrefix("MEMOBIRD")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	for _, binding := range []struct {
		key string
		env string
	}{
		{key: "memobird.access_key", env: "MEMOBIRD_ACCESS_KEY"},
		{key: "memobird.device_id", env: "MEMOBIRD_DEVICE_ID"},
		{key: "memobird.user_id", env: "MEMOBIRD_USER_ID"},
		{key: "memobird.base_url", env: "MEMOBIRD_BASE_URL"},
		{key: "memobird.timeout_sec", env: "MEMOBIRD_TIMEOUT_SEC"},
		{key: "storage.db_path", env: "MEMOBIRD_STORAGE_DB_PATH"},
	} {
		if err := v.BindEnv(binding.key, binding.env); err != nil {
			return nil, fmt.Errorf("failed to bind env %s: %w", binding.env, err)
		}
	}

	v.SetDefault("memobird.base_url", "http://open.memobird.cn")
	v.SetDefault("memobird.timeout_sec", 30)
	v.SetDefault("storage.db_path", "./memobird.db")

	if err := v.ReadInConfig(); err != nil {
		var configNotFound viper.ConfigFileNotFoundError
		if !errors.As(err, &configNotFound) && !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("failed to read config: %w", err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &cfg, nil
}

// Validate checks that all required configuration fields are set.
func (c *Config) Validate() error {
	if c.Memobird.AccessKey == "" {
		return fmt.Errorf("memobird.access_key is required")
	}
	if c.Memobird.DeviceID == "" {
		return fmt.Errorf("memobird.device_id is required")
	}
	return nil
}
