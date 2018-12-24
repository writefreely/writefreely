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
	"fmt"
	"net/http"
)

func handleViewHostMeta(app *app, w http.ResponseWriter, r *http.Request) error {
	w.Header().Set("Server", serverSoftware)
	w.Header().Set("Content-Type", "application/xrd+xml; charset=utf-8")

	meta := `<?xml version="1.0" encoding="UTF-8"?>
<XRD xmlns="http://docs.oasis-open.org/ns/xri/xrd-1.0">
  <Link rel="lrdd" type="application/xrd+xml" template="https://` + r.Host + `/.well-known/webfinger?resource={uri}"/>
</XRD>`
	fmt.Fprintf(w, meta)

	return nil
}
