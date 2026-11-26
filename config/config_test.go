/*
 * Copyright © 2026 Musing Studio LLC.
 *
 * This file is part of WriteFreely.
 *
 * WriteFreely is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License, included
 * in the LICENSE file in this source code package.
 */

package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestRequireEmailDefault ensures newly generated config files require an email address on signup, while existing
// config files that don't mention the setting keep it off.
func TestRequireEmailDefault(t *testing.T) {
	dir := t.TempDir()

	// A newly generated config file
	newCfg := filepath.Join(dir, "new.ini")
	if err := Save(New(), newCfg); err != nil {
		t.Fatalf("save new config: %v", err)
	}
	cfg, err := Load(newCfg)
	if err != nil {
		t.Fatalf("load new config: %v", err)
	}
	if !cfg.App.RequireEmail {
		t.Error("new config files should set require_email = true")
	}

	// An existing config file, written before the setting existed
	oldCfg := filepath.Join(dir, "old.ini")
	err = os.WriteFile(oldCfg, []byte("[app]\nhost = http://localhost:8080\nopen_registration = true\n"), 0600)
	if err != nil {
		t.Fatalf("write old config: %v", err)
	}
	cfg, err = Load(oldCfg)
	if err != nil {
		t.Fatalf("load old config: %v", err)
	}
	if cfg.App.RequireEmail {
		t.Error("existing config files without require_email should keep it off")
	}
}
