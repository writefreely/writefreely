/*
 * Copyright © 2018-2019 A Bunch Tell LLC.
 *
 * This file is part of WriteFreely.
 *
 * WriteFreely is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License, included
 * in the LICENSE file in this source code package.
 */

// Package config holds and assists in the configuration of a writefreely instance.
package config

import (
	"gopkg.in/ini.v1"
	"strings"
)

const (
	// FileName is the default configuration file name
	FileName = "config.ini"

	UserNormal UserType = "user"
	UserAdmin           = "admin"
)

type (
	UserType string

	// ServerCfg holds values that affect how the HTTP server runs
	ServerCfg struct {
		HiddenHost string `ini:"hidden_host"`
		Port       int    `ini:"port"`
		Bind       string `ini:"bind"`

		TLSCertPath string `ini:"tls_cert_path"`
		TLSKeyPath  string `ini:"tls_key_path"`
		Autocert    bool   `ini:"autocert"`

		TemplatesParentDir string `ini:"templates_parent_dir"`
		StaticParentDir    string `ini:"static_parent_dir"`
		PagesParentDir     string `ini:"pages_parent_dir"`
		KeysParentDir      string `ini:"keys_parent_dir"`

		HashSeed string `ini:"hash_seed"`

		Dev bool `ini:"-"`
	}

	// DatabaseCfg holds values that determine how the application connects to a datastore
	DatabaseCfg struct {
		Type     string `ini:"type"`
		FileName string `ini:"filename"`
		User     string `ini:"username"`
		Password string `ini:"password"`
		Database string `ini:"database"`
		Host     string `ini:"host"`
		Port     int    `ini:"port"`
	}

	WriteAsOauthCfg struct {
		ClientID              string `ini:"client_id"`
		ClientSecret          string `ini:"client_secret"`
		AuthLocation          string `ini:"auth_location"`
		TokenLocation         string `ini:"token_location"`
		InspectLocation       string `ini:"inspect_location"`
		StateRegisterLocation string `ini:"state_register_location"`
	}

	SlackOauthCfg struct {
		ClientID              string `ini:"client_id"`
		ClientSecret          string `ini:"client_secret"`
		TeamID                string `ini:"team_id"`
		StateRegisterLocation string `ini:"state_register_location"`
	}

	// AppCfg holds values that affect how the application functions
	AppCfg struct {
		SiteName string `ini:"site_name"`
		SiteDesc string `ini:"site_description"`
		Host     string `ini:"host"`

		// Site appearance
		Theme      string `ini:"theme"`
		Editor     string `ini:"editor"`
		JSDisabled bool   `ini:"disable_js"`
		WebFonts   bool   `ini:"webfonts"`
		Landing    string `ini:"landing"`
		SimpleNav  bool   `ini:"simple_nav"`
		WFModesty  bool   `ini:"wf_modesty"`

		// Site functionality
		Chorus        bool `ini:"chorus"`
		DisableDrafts bool `ini:"disable_drafts"`

		// Users
		SingleUser       bool `ini:"single_user"`
		OpenRegistration bool `ini:"open_registration"`
		MinUsernameLen   int  `ini:"min_username_len"`
		MaxBlogs         int  `ini:"max_blogs"`

		// Federation
		Federation  bool `ini:"federation"`
		PublicStats bool `ini:"public_stats"`

		// Access
		Private bool `ini:"private"`

		// Additional functions
		LocalTimeline bool   `ini:"local_timeline"`
		UserInvites   string `ini:"user_invites"`

		// Defaults
		DefaultVisibility string `ini:"default_visibility"`
	}

	// Config holds the complete configuration for running a writefreely instance
	Config struct {
		Server       ServerCfg       `ini:"server"`
		Database     DatabaseCfg     `ini:"database"`
		App          AppCfg          `ini:"app"`
		SlackOauth   SlackOauthCfg   `ini:"oauth.slack"`
		WriteAsOauth WriteAsOauthCfg `ini:"oauth.writeas"`
	}
)

// New creates a new Config with sane defaults
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

// IsSecureStandalone returns whether or not the application is running as a
// standalone server with TLS enabled.
func (cfg *Config) IsSecureStandalone() bool {
	return cfg.Server.Port == 443 && cfg.Server.TLSCertPath != "" && cfg.Server.TLSKeyPath != ""
}

func (ac *AppCfg) LandingPath() string {
	if !strings.HasPrefix(ac.Landing, "/") {
		return "/" + ac.Landing
	}
	return ac.Landing
}

// Load reads the given configuration file, then parses and returns it as a Config.
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

// Save writes the given Config to the given file.
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
