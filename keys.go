/*
 * Copyright © 2018-2019 A Bunch Tell LLC.
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
	"github.com/writeas/writefreely/key"
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
)


func initKeyPaths(app *App) {
	emailKeyPath = filepath.Join(app.cfg.Server.KeysParentDir, emailKeyPath)
	cookieAuthKeyPath = filepath.Join(app.cfg.Server.KeysParentDir, cookieAuthKeyPath)
	cookieKeyPath = filepath.Join(app.cfg.Server.KeysParentDir, cookieKeyPath)
}

func initKeys(app *App) error {
	var err error
	app.keys = &key.Keychain{}

	if debugging {
		log.Info("  %s", emailKeyPath)
	}
	app.keys.EmailKey, err = ioutil.ReadFile(emailKeyPath)
	if err != nil {
		return err
	}

	if debugging {
		log.Info("  %s", cookieAuthKeyPath)
	}
	app.keys.CookieAuthKey, err = ioutil.ReadFile(cookieAuthKeyPath)
	if err != nil {
		return err
	}

	if debugging {
		log.Info("  %s", cookieKeyPath)
	}
	app.keys.CookieKey, err = ioutil.ReadFile(cookieKeyPath)
	if err != nil {
		return err
	}

	return nil
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
