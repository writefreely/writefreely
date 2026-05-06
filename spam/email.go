/*
 * Copyright © 2020-2021 Musing Studio LLC.
 *
 * This file is part of WriteFreely.
 *
 * WriteFreely is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License, included
 * in the LICENSE file in this source code package.
 */

package spam

import (
	"github.com/writeas/web-core/id"
	"strings"
)

var honeypotField string

func HoneypotFieldName() string {
	if honeypotField == "" {
		honeypotField = id.Generate62RandomString(39)
	}
	return honeypotField
}

// CleanEmail takes an email address and strips it down to a unique address that can be blocked.
func CleanEmail(email string) string {
	emailParts := strings.Split(strings.ToLower(email), "@")
	if len(emailParts) < 2 {
		return ""
	}
	u := emailParts[0]
	d := emailParts[1]
	// Ignore anything after '+'
	plusIdx := strings.IndexRune(u, '+')
	if plusIdx > -1 {
		u = u[:plusIdx]
	}
	// Strip dots in email address
	u = strings.ReplaceAll(u, ".", "")
	return u + "@" + d
}
