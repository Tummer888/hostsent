package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	App  AppConfig  `mapstructure:"app"`
	Auth AuthConfig `mapstructure:"auth"`
}

type AppConfig struct {
	Name         string `mapstructure:"name"`
	Env          string `mapstructure:"env"`
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	ReadTimeout  int    `mapstructure:"read_timeout"`
	WriteTimeout int    `mapstructure:"write_timeout"`
}

type AuthConfig struct {
	BearerPrefix string `mapstructure:"bearer_prefix"`
	MockToken    string `mapstructure:"mock_token"`
}

func Load() (*Config, error) {
	v := viper.New()
	v.SetConfigType("yaml")
	v.SetConfigName("config")
	v.AddConfigPath("./configs")
	v.AddConfigPath("../configs")
	v.AddConfigPath("../../configs")

	v.SetEnvPrefix("HOSTSENT")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	setDefaults(v)

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("read config: %w", err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("app.name", "hostsent-backend")
	v.SetDefault("app.env", "development")
	v.SetDefault("app.host", "0.0.0.0")
	v.SetDefault("app.port", 8080)
	v.SetDefault("app.read_timeout", 10)
	v.SetDefault("app.write_timeout", 10)
	v.SetDefault("auth.bearer_prefix", "Bearer")
	v.SetDefault("auth.mock_token", "mock-token")
}
