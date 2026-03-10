package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server     ServerConfig     `mapstructure:"server"`
	Admin      AdminConfig      `mapstructure:"admin"`
	Database   DatabaseConfig   `mapstructure:"database"`
	Upstream   UpstreamConfig   `mapstructure:"upstream"`
	Decisions  DecisionsConfig  `mapstructure:"decisions"`
	Allowlists AllowlistsConfig `mapstructure:"allowlists"`
	Log        LogConfig        `mapstructure:"log"`
}

type ServerConfig struct {
	Listen string        `mapstructure:"listen"`
	JWTTTL time.Duration `mapstructure:"jwt_ttl"`
}

type AdminConfig struct {
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	APIKey   string `mapstructure:"api_key"`
}

type AllowlistsConfig struct {
	File string `mapstructure:"file"`
}

type DatabaseConfig struct {
	DSN string `mapstructure:"dsn"`
}

type UpstreamConfig struct {
	Enabled      bool          `mapstructure:"enabled"`
	BaseURL      string        `mapstructure:"base_url"`
	MachineID    string        `mapstructure:"machine_id"`
	Password     string        `mapstructure:"password"`
	SyncInterval time.Duration `mapstructure:"sync_interval"`
	PushSignals  bool          `mapstructure:"push_signals"`
}

type DecisionsConfig struct {
	DefaultDuration time.Duration `mapstructure:"default_duration"`
	Sources         SourcesConfig `mapstructure:"sources"`
}

type SourcesConfig struct {
	LocalSignals bool `mapstructure:"local_signals"`
	UpstreamCAPI bool `mapstructure:"upstream_capi"`
	Manual       bool `mapstructure:"manual"`
}

type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

func Load(cfgFile string) (*Config, error) {
	v := viper.New()

	// Defaults
	v.SetDefault("server.listen", "0.0.0.0:8080")
	v.SetDefault("server.jwt_ttl", "24h")
	v.SetDefault("admin.username", "admin")
	v.SetDefault("upstream.base_url", "https://api.crowdsec.net")
	v.SetDefault("upstream.sync_interval", "1h")
	v.SetDefault("decisions.default_duration", "24h")
	v.SetDefault("decisions.sources.local_signals", true)
	v.SetDefault("decisions.sources.upstream_capi", true)
	v.SetDefault("decisions.sources.manual", true)
	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "json")

	if cfgFile != "" {
		v.SetConfigFile(cfgFile)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
		v.AddConfigPath("/etc/crowdsec-capi/")
	}

	v.SetEnvPrefix("CAPI")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("reading config: %w", err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}
	return &cfg, nil
}
