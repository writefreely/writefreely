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
	"io/fs"
	"os"
)

// templatesFileSystem serves templates straight off disk. This is the
// default build, so editing files under templates/ is picked up without a
// rebuild.
func templatesFileSystem(parentDir string) fs.FS {
	return diskContentFS(parentDir, templatesDir)
}

// pagesFileSystem serves pages straight off disk.
func pagesFileSystem(parentDir string) fs.FS {
	return diskContentFS(parentDir, pagesDir)
}

func diskContentFS(parentDir, sub string) fs.FS {
	root := parentDir
	if root == "" {
		root = "."
	}
	subFS, err := fs.Sub(os.DirFS(root), sub)
	if err != nil {
		panic(err)
	}
	return subFS
}
