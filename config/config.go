/*
 * Copyright © 2018 A Bunch Tell LLC.
 *
 * This file is part of WriteFreely.
 *
 * WriteFreely is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License, included
 * in the LICENSE file in this source code package.
 */

package config

import (
	"gopkg.in/ini.v1"
)

const (
	FileName = "config.ini"
)

type (
	ServerCfg struct {
		HiddenHost string `ini:"hidden_host"`
		Port       int    `ini:"port"`
		Bind       string `ini:"bind"`

		TLSCertPath string `ini:"tls_cert_path"`
		TLSKeyPath  string `ini:"tls_key_path"`

		Dev bool `ini:"-"`
	}

	DatabaseCfg struct {
		Type     string `ini:"type"`
		FileName string `ini:"filename"`
		User     string `ini:"username"`
		Password string `ini:"password"`
		Database string `ini:"database"`
		Host     string `ini:"host"`
		Port     int    `ini:"port"`
	}

	AppCfg struct {
		SiteName string `ini:"site_name"`
		SiteDesc string `ini:"site_description"`
		Host     string `ini:"host"`

		// Site appearance
		Theme      string `ini:"theme"`
		JSDisabled bool   `ini:"disable_js"`
		WebFonts   bool   `ini:"webfonts"`

		// Users
		SingleUser       bool `ini:"single_user"`
		OpenRegistration bool `ini:"open_registration"`
		MinUsernameLen   int  `ini:"min_username_len"`
		MaxBlogs         int  `ini:"max_blogs"`

		// Federation
		Federation  bool `ini:"federation"`
		PublicStats bool `ini:"public_stats"`
		Private     bool `ini:"private"`

		// Additional functions
		LocalTimeline bool `ini:"local_timeline"`
	}

	Config struct {
		Server   ServerCfg   `ini:"server"`
		Database DatabaseCfg `ini:"database"`
		App      AppCfg      `ini:"app"`
	}
)

func New() *Config {
	c := &Config{
		Server: ServerCfg{
			Port: 8080,
			Bind: "localhost", /* IPV6 support when not using localhost? */
		},
		App: AppCfg{
			Host:           "http://localhost:8080",
			Theme:          "write",
			WebFonts:       true,
			SingleUser:     true,
			MinUsernameLen: 3,
			MaxBlogs:       1,
			Federation:     true,
			PublicStats:    true,
		},
	}
	c.UseMySQL(true)
	return c
}

// UseMySQL resets the Config's Database to use default values for a MySQL setup.
func (cfg *Config) UseMySQL(fresh bool) {
	cfg.Database.Type = "mysql"
	if fresh {
		cfg.Database.Host = "localhost"
		cfg.Database.Port = 3306
	}
}

// UseSQLite resets the Config's Database to use default values for a SQLite setup.
func (cfg *Config) UseSQLite(fresh bool) {
	cfg.Database.Type = "sqlite3"
	if fresh {
		cfg.Database.FileName = "writefreely.db"
	}
}

func (cfg *Config) IsSecureStandalone() bool {
	return cfg.Server.Port == 443 && cfg.Server.TLSCertPath != "" && cfg.Server.TLSKeyPath != ""
}

func Load(fname string) (*Config, error) {
	if fname == "" {
		fname = FileName
	}
	cfg, err := ini.Load(fname)
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

func Save(uc *Config, fname string) error {
	cfg := ini.Empty()
	err := ini.ReflectFrom(cfg, uc)
	if err != nil {
		return err
	}

	if fname == "" {
		fname = FileName
	}
	return cfg.SaveTo(fname)
}
