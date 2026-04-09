package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ruhuang2001/memobird-go/memobird"
	"github.com/spf13/viper"
)

const (
	defaultBaseURL    = "https://open.memobird.cn"
	defaultTimeoutSec = 30
	defaultDBPath     = "./memobird.db"
)

// Config is the top-level optional configuration helper for library users.
type Config struct {
	Memobird memobird.Config `mapstructure:"memobird"`
	Storage  StorageConfig   `mapstructure:"storage"`
}

// StorageConfig contains configuration for the storage layer.
type StorageConfig struct {
	DBPath string `mapstructure:"db_path"`
}

// TimeoutSec returns the configured timeout in seconds.
func (c *Config) TimeoutSec() int {
	if c.Memobird.Timeout <= 0 {
		return defaultTimeoutSec
	}
	return int((c.Memobird.Timeout + time.Second - 1) / time.Second)
}

// Load reads configuration from a file and environment variables.
// If configPath is empty, it looks for config.yaml in the current directory.
func Load(configPath string) (*Config, error) {
	v := viper.New()

	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
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
		{key: "memobird.timeout", env: "MEMOBIRD_TIMEOUT"},
		{key: "memobird.timeout_sec", env: "MEMOBIRD_TIMEOUT_SEC"},
		{key: "storage.db_path", env: "MEMOBIRD_STORAGE_DB_PATH"},
	} {
		if err := v.BindEnv(binding.key, binding.env); err != nil {
			return nil, fmt.Errorf("failed to bind env %s: %w", binding.env, err)
		}
	}

	v.SetDefault("memobird.base_url", defaultBaseURL)
	v.SetDefault("memobird.timeout_sec", defaultTimeoutSec)
	v.SetDefault("storage.db_path", defaultDBPath)

	if err := v.ReadInConfig(); err != nil {
		var configNotFound viper.ConfigFileNotFoundError
		missingConfig := errors.As(err, &configNotFound) || errors.Is(err, os.ErrNotExist)
		if configPath != "" && missingConfig {
			return nil, fmt.Errorf("config file %q not found: %w", configPath, err)
		}
		if !missingConfig {
			return nil, fmt.Errorf("failed to read config: %w", err)
		}
	}

	var raw struct {
		Memobird struct {
			AccessKey  string `mapstructure:"access_key"`
			DeviceID   string `mapstructure:"device_id"`
			UserID     int    `mapstructure:"user_id"`
			BaseURL    string `mapstructure:"base_url"`
			Timeout    string `mapstructure:"timeout"`
			TimeoutSec int    `mapstructure:"timeout_sec"`
		} `mapstructure:"memobird"`
		Storage StorageConfig `mapstructure:"storage"`
	}
	if err := v.Unmarshal(&raw); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	cfg := &Config{Storage: raw.Storage}
	cfg.Memobird.AccessKey = raw.Memobird.AccessKey
	cfg.Memobird.DeviceID = raw.Memobird.DeviceID
	cfg.Memobird.UserID = raw.Memobird.UserID
	cfg.Memobird.BaseURL = raw.Memobird.BaseURL

	switch {
	case raw.Memobird.Timeout != "":
		timeout, err := time.ParseDuration(raw.Memobird.Timeout)
		if err != nil {
			return nil, fmt.Errorf("invalid memobird.timeout: %w", err)
		}
		cfg.Memobird.Timeout = timeout
	case raw.Memobird.TimeoutSec > 0:
		cfg.Memobird.Timeout = time.Duration(raw.Memobird.TimeoutSec) * time.Second
	default:
		cfg.Memobird.Timeout = defaultTimeoutSec * time.Second
	}

	if cfg.Storage.DBPath == "" {
		cfg.Storage.DBPath = defaultDBPath
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return cfg, nil
}

// Validate checks that all required configuration fields are set.
func (c *Config) Validate() error {
	return c.Memobird.Validate()
}
