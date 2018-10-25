package config

import (
	"gopkg.in/ini.v1"
)

const (
	FileName = "config.ini"
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
		SiteName string `ini:"site_name"`

		// Site appearance
		Theme      string `ini:"theme"`
		JSDisabled bool   `ini:"disable_js"`
		WebFonts   bool   `ini:"webfonts"`

		// Users
		SingleUser       bool `ini:"single_user"`
		OpenRegistration bool `ini:"open_registration"`
		MinUsernameLen   int  `ini:"min_username_len"`

		// Federation
		Federation  bool `ini:"federation"`
		PublicStats bool `ini:"public_stats"`
		Private     bool `ini:"private"`
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
			Theme:          "write",
			WebFonts:       true,
			SingleUser:     true,
			MinUsernameLen: 3,
			Federation:     true,
			PublicStats:    true,
		},
	}
}

func Load() (*Config, error) {
	cfg, err := ini.Load(FileName)
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

	return cfg.SaveTo(FileName)
}
