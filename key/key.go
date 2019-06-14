/*
 * Copyright © 2019 A Bunch Tell LLC.
 *
 * This file is part of WriteFreely.
 *
 * WriteFreely is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License, included
 * in the LICENSE file in this source code package.
 */

// Package key holds application keys and utilities around generating them.
package key

import (
	"crypto/rand"
)

const (
	EncKeysBytes = 32
)

type Keychain struct {
	EmailKey, CookieAuthKey, CookieKey []byte
}

// GenerateKeys generates necessary keys for the app on the given Keychain,
// skipping any that already exist.
func (keys *Keychain) GenerateKeys() error {
	// Generate keys only if they don't already exist
	// TODO: use something like https://github.com/hashicorp/go-multierror to return errors
	var err, keyErrs error
	if len(keys.EmailKey) == 0 {
		keys.EmailKey, err = GenerateBytes(EncKeysBytes)
		if err != nil {
			keyErrs = err
		}
	}
	if len(keys.CookieAuthKey) == 0 {
		keys.CookieAuthKey, err = GenerateBytes(EncKeysBytes)
		if err != nil {
			keyErrs = err
		}
	}
	if len(keys.CookieKey) == 0 {
		keys.CookieKey, err = GenerateBytes(EncKeysBytes)
		if err != nil {
			keyErrs = err
		}
	}

	return keyErrs
}

// GenerateBytes returns securely generated random bytes.
func GenerateBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return nil, err
	}

	return b, nil
}
