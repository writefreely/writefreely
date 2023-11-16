/*
 * Copyright © 2023 Musing Studio LLC.
 *
 * This file is part of WriteFreely.
 *
 * WriteFreely is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License, included
 * in the LICENSE file in this source code package.
 */

package spam

import (
	"net/http"
	"strings"
)

func GetIP(r *http.Request) string {
	h := r.Header.Get("X-Forwarded-For")
	if h == "" {
		return ""
	}
	ips := strings.Split(h, ",")
	return strings.TrimSpace(ips[0])
}
