/*
 * Copyright © 2018-2021 Musing Studio LLC.
 *
 * This file is part of WriteFreely.
 *
 * WriteFreely is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License, included
 * in the LICENSE file in this source code package.
 */

package author

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/writefreely/writefreely/config"
)

// newCfg creates a config pointing at a temp pages dir with the given min username length.
func newCfg(t *testing.T, minLen int) *config.Config {
	t.Helper()
	dir := t.TempDir()
	pagesDir := filepath.Join(dir, "pages")
	if err := os.MkdirAll(pagesDir, 0o700); err != nil {
		t.Fatalf("mkdir pages: %v", err)
	}
	cfg := &config.Config{}
	cfg.App.MinUsernameLen = minLen
	cfg.Server.PagesParentDir = dir
	return cfg
}

func TestIsValidUsername_TooShort(t *testing.T) {
	cfg := newCfg(t, 3)
	if IsValidUsername(cfg, "ab") {
		t.Error("expected 'ab' (len 2) to be invalid when MinUsernameLen=3")
	}
}

func TestIsValidUsername_AtMinLength(t *testing.T) {
	cfg := newCfg(t, 3)
	if !IsValidUsername(cfg, "abc") {
		t.Error("expected 'abc' (len 3) to be valid when MinUsernameLen=3")
	}
}

func TestIsValidUsername_Reserved(t *testing.T) {
	cfg := newCfg(t, 1)
	reserved := []string{"admin", "about", "login", "logout", "signup", "api", "read"}
	for _, name := range reserved {
		t.Run(name, func(t *testing.T) {
			if IsValidUsername(cfg, name) {
				t.Errorf("expected %q to be reserved/invalid", name)
			}
		})
	}
}

func TestIsValidUsername_ValidNames(t *testing.T) {
	cfg := newCfg(t, 1)
	valid := []string{"alice", "bob123", "My-Blog", "A", "user-name"}
	for _, name := range valid {
		t.Run(name, func(t *testing.T) {
			if !IsValidUsername(cfg, name) {
				t.Errorf("expected %q to be a valid username", name)
			}
		})
	}
}

func TestIsValidUsername_InvalidChars(t *testing.T) {
	cfg := newCfg(t, 1)
	invalid := []string{"user name", "user@name", "user.name", "user/name", "-leading-dash"}
	for _, name := range invalid {
		t.Run(name, func(t *testing.T) {
			if IsValidUsername(cfg, name) {
				t.Errorf("expected %q to be invalid due to bad characters", name)
			}
		})
	}
}

func TestIsValidUsername_PageNameIsReserved(t *testing.T) {
	cfg := newCfg(t, 1)
	// Create a custom page file so that filename becomes reserved
	pagesDir := filepath.Join(cfg.Server.PagesParentDir, "pages")
	pageFile := filepath.Join(pagesDir, "mypage")
	if err := os.WriteFile(pageFile, []byte("content"), 0o600); err != nil {
		t.Fatalf("create page file: %v", err)
	}
	if IsValidUsername(cfg, "mypage") {
		t.Error("expected 'mypage' to be reserved because a page with that name exists")
	}
}
