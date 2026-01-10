package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Memobird MemobirdConfig `mapstructure:"memobird"`
	Storage  StorageConfig  `mapstructure:"storage"`
}

type MemobirdConfig struct {
	AccessKey  string `mapstructure:"access_key"`
	DeviceID   string `mapstructure:"device_id"`
	UserID     int    `mapstructure:"user_id"`
	BaseURL    string `mapstructure:"base_url"`
	TimeoutSec int    `mapstructure:"timeout_sec"`
}

type StorageConfig struct {
	DBPath string `mapstructure:"db_path"`
}

func (m *MemobirdConfig) Timeout() time.Duration {
	if m.TimeoutSec <= 0 {
		return 30 * time.Second
	}
	return time.Duration(m.TimeoutSec) * time.Second
}

func (m *MemobirdConfig) GetBaseURL() string {
	if m.BaseURL == "" {
		return "http://open.memobird.cn"
	}
	return m.BaseURL
}

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
	v.AutomaticEnv()

	v.SetDefault("memobird.base_url", "http://open.memobird.cn")
	v.SetDefault("memobird.timeout_sec", 30)
	v.SetDefault("storage.db_path", "./memobird.db")

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
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

func (c *Config) Validate() error {
	if c.Memobird.AccessKey == "" {
		return fmt.Errorf("memobird.access_key is required")
	}
	if c.Memobird.DeviceID == "" {
		return fmt.Errorf("memobird.device_id is required")
	}
	return nil
}
