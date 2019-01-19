/*
 * Copyright © 2018 A Bunch Tell LLC.
 *
 * This file is part of WriteFreely.
 *
 * WriteFreely is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License, included
 * in the LICENSE file in this source code package.
 */

package writefreely

import (
	"crypto/rand"
	"github.com/writeas/web-core/log"
	"io/ioutil"
	"os"
	"path/filepath"
)

const (
	keysDir = "keys"

	encKeysBytes = 32
)

var (
	emailKeyPath      = filepath.Join(keysDir, "email.aes256")
	cookieAuthKeyPath = filepath.Join(keysDir, "cookies_auth.aes256")
	cookieKeyPath     = filepath.Join(keysDir, "cookies_enc.aes256")
)

type keychain struct {
	emailKey, cookieAuthKey, cookieKey []byte
}

func initKeys(app *app) error {
	var err error
	app.keys = &keychain{}

	emailKeyPath = filepath.Join(app.cfg.Server.KeysParentDir, emailKeyPath)
	if debugging {
		log.Info("  %s", emailKeyPath)
	}
	app.keys.emailKey, err = ioutil.ReadFile(emailKeyPath)
	if err != nil {
		return err
	}

	cookieAuthKeyPath = filepath.Join(app.cfg.Server.KeysParentDir, cookieAuthKeyPath)
	if debugging {
		log.Info("  %s", cookieAuthKeyPath)
	}
	app.keys.cookieAuthKey, err = ioutil.ReadFile(cookieAuthKeyPath)
	if err != nil {
		return err
	}

	cookieKeyPath = filepath.Join(app.cfg.Server.KeysParentDir, cookieKeyPath)
	if debugging {
		log.Info("  %s", cookieKeyPath)
	}
	app.keys.cookieKey, err = ioutil.ReadFile(cookieKeyPath)
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
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		log.Info("%s already exists. rm the file if you understand the consquences.", path)
		return nil
	}

	log.Info("Generating %s.", path)
	b, err := generateBytes(encKeysBytes)
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

// generateBytes returns securely generated random bytes.
func generateBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return nil, err
	}

	return b, nil
}
