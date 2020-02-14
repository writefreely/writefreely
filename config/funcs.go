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
	"net/http"
	"strings"
	"time"
)

// FriendlyHost returns the app's Host sans any schema
func (ac AppCfg) FriendlyHost() string {
	return ac.Host[strings.Index(ac.Host, "://")+len("://"):]
}

func (ac AppCfg) CanCreateBlogs(currentlyUsed uint64) bool {
	if ac.MaxBlogs <= 0 {
		return true
	}
	return int(currentlyUsed) < ac.MaxBlogs
}

// OrDefaultString returns input or a default value if input is empty.
func OrDefaultString(input, defaultValue string) string {
	if len(input) == 0 {
		return defaultValue
	}
	return input
}

// DefaultHTTPClient returns a sane default HTTP client.
func DefaultHTTPClient() *http.Client {
	return &http.Client{Timeout: 10 * time.Second}
}
