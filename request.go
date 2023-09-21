/*
 * Copyright © 2018 Musing Studio LLC.
 *
 * This file is part of WriteFreely.
 *
 * WriteFreely is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License, included
 * in the LICENSE file in this source code package.
 */

package writefreely

import (
	"mime"
	"net/http"
	"strings"
)

func IsJSON(r *http.Request) bool {
	ct, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	accept := r.Header.Get("Accept")
	return ct == "application/json" || accept == "application/json"
}

func IsActivityPubRequest(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	return strings.Contains(accept, "application/activity+json") ||
		accept == "application/ld+json; profile=\"https://www.w3.org/ns/activitystreams\""
}
