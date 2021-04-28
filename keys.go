/*
 * Copyright © 2018-2019, 2021 A Bunch Tell LLC.
 *
 * This file is part of WriteFreely.
 *
 * WriteFreely is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License, included
 * in the LICENSE file in this source code package.
 */

package writefreely

import (
	"github.com/writeas/web-core/log"
	"github.com/writefreely/writefreely/key"
	"io/ioutil"
	"os"
	"path/filepath"
)

const (
	keysDir = "keys"
)

var (
	emailKeyPath      = filepath.Join(keysDir, "email.aes256")
	cookieAuthKeyPath = filepath.Join(keysDir, "cookies_auth.aes256")
	cookieKeyPath     = filepath.Join(keysDir, "cookies_enc.aes256")
	csrfKeyPath       = filepath.Join(keysDir, "csrf.aes256")
)

// InitKeys loads encryption keys into memory via the given Apper interface
func InitKeys(apper Apper) error {
	log.Info("Loading encryption keys...")
	err := apper.LoadKeys()
	if err != nil {
		return err
	}
	return nil
}

func initKeyPaths(app *App) {
	emailKeyPath = filepath.Join(app.cfg.Server.KeysParentDir, emailKeyPath)
	cookieAuthKeyPath = filepath.Join(app.cfg.Server.KeysParentDir, cookieAuthKeyPath)
	cookieKeyPath = filepath.Join(app.cfg.Server.KeysParentDir, cookieKeyPath)
	csrfKeyPath = filepath.Join(app.cfg.Server.KeysParentDir, csrfKeyPath)
}

// generateKey generates a key at the given path used for the encryption of
// certain user data. Because user data becomes unrecoverable without these
// keys, this won't overwrite any existing key, and instead outputs a message.
func generateKey(path string) error {
	// Check if key file exists
	if _, err := os.Stat(path); err == nil {
		log.Info("%s already exists. rm the file if you understand the consquences.", path)
		return nil
	} else if !os.IsNotExist(err) {
		log.Error("%s", err)
		return err
	}

	log.Info("Generating %s.", path)
	b, err := key.GenerateBytes(key.EncKeysBytes)
	if err != nil {
		log.Error("FAILED. %s. Run writefreely --gen-keys again.", err)
		return err
	}
	err = ioutil.WriteFile(path, b, 0600)
	if err != nil {
		log.Error("FAILED writing file: %s", err)
		return err
	}
	log.Info("Success.")
	return nil
}
