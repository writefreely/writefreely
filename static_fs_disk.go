//go:build !embed

/*
 * Copyright © 2026 Musing Studio LLC.
 *
 * This file is part of WriteFreely.
 *
 * WriteFreely is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License, included
 * in the LICENSE file in this source code package.
 */

package writefreely

import (
	"net/http"
	"path/filepath"
)

// staticFileSystem serves static assets straight off disk. This is the
// default build, so editing files under static/ is picked up without a rebuild.
func staticFileSystem(parentDir string) http.FileSystem {
	return http.Dir(filepath.Join(parentDir, staticDir))
}
