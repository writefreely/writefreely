package config

import (
	"gopkg.in/ini.v1"
)

const (
	configFile = "config.ini"
)

type (
	ServerCfg struct {
		Host string `ini:"host"`
		Port int    `ini:"port"`
	}

	DatabaseCfg struct {
		Type     string `ini:"type"`
		User     string `ini:"username"`
		Password string `ini:"password"`
		Database string `ini:"database"`
		Host     string `ini:"host"`
		Port     int    `ini:"port"`
	}

	AppCfg struct {
		MultiUser   bool `ini:"multiuser"`
		OpenSignups bool `ini:"open_signups"`
		Federation  bool `ini:"federation"`

		Name string `ini:"site_name"`

		JSDisabled bool `ini:"disable_js"`

		// User registration
		MinUsernameLen int `ini:"min_username_len"`
	}

	Config struct {
		Server   ServerCfg   `ini:"server"`
		Database DatabaseCfg `ini:"database"`
		App      AppCfg      `ini:"app"`
	}
)

func New() *Config {
	return &Config{
		Server: ServerCfg{
			Host: "http://localhost:8080",
			Port: 8080,
		},
		Database: DatabaseCfg{
			Type: "mysql",
			Host: "localhost",
			Port: 3306,
		},
		App: AppCfg{
			Federation:     true,
			MinUsernameLen: 3,
		},
	}
}

func Load() (*Config, error) {
	cfg, err := ini.Load(configFile)
	if err != nil {
		return nil, err
	}

	// Parse INI file
	uc := &Config{}
	err = cfg.MapTo(uc)
	if err != nil {
		return nil, err
	}
	return uc, nil
}

func Save(uc *Config) error {
	cfg := ini.Empty()
	err := ini.ReflectFrom(cfg, uc)
	if err != nil {
		return err
	}

	return cfg.SaveTo(configFile)
}
