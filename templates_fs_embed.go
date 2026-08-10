//go:build embed

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
	"os"
)

//go:embed templates pages
var embeddedContentFiles embed.FS

// templatesFileSystem serves templates embedded in the binary at build
// time, falling back to disk only for paths an admin has explicitly
// overridden -- e.g. by pointing templates_parent_dir at a custom theme, or
// dropping a replacement file next to the binary.
func templatesFileSystem(parentDir string) fs.FS {
	return overlayContentFS(parentDir, templatesDir)
}

// pagesFileSystem serves pages embedded in the binary, with the same
// disk-overrides-embedded behavior as templatesFileSystem.
func pagesFileSystem(parentDir string) fs.FS {
	return overlayContentFS(parentDir, pagesDir)
}

func overlayContentFS(parentDir, sub string) fs.FS {
	root := parentDir
	if root == "" {
		root = "."
	}
	disk, err := fs.Sub(os.DirFS(root), sub)
	if err != nil {
		panic(err)
	}
	embedded, err := fs.Sub(embeddedContentFiles, sub)
	if err != nil {
		panic(err)
	}
	return &diskEmbedOverlayFS{disk: disk, embedded: embedded}
}

type diskEmbedOverlayFS struct {
	disk     fs.FS
	embedded fs.FS
}

func (o *diskEmbedOverlayFS) Open(name string) (fs.File, error) {
	if f, err := o.disk.Open(name); err == nil {
		return f, nil
	}
	return o.embedded.Open(name)
}
