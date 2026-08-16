//go:build embed
// +build embed

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
	"embed"
	"io/fs"
	"net/http"
	"path/filepath"
)

//go:embed static
var embeddedStaticFiles embed.FS

// staticFileSystem serves static assets embedded in the binary at build
// time, falling back to disk-based lookups so file-based overrides -- like
// static/local/custom.css, which is created after deployment and is never
// embedded -- still take precedence over the embedded copy.
func staticFileSystem(parentDir string) http.FileSystem {
	sub, err := fs.Sub(embeddedStaticFiles, staticDir)
	if err != nil {
		panic(err)
	}
	return &overlayFileSystem{
		disk:     http.Dir(filepath.Join(parentDir, staticDir)),
		embedded: http.FS(sub),
	}
}

type overlayFileSystem struct {
	disk     http.FileSystem
	embedded http.FileSystem
}

func (o *overlayFileSystem) Open(name string) (http.File, error) {
	if f, err := o.disk.Open(name); err == nil {
		return f, nil
	}
	return o.embedded.Open(name)
}
