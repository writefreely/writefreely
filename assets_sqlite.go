//go:build sqlite && !wflib
// +build sqlite,!wflib

package writefreely

import _ "embed"

//go:embed sqlite.sql
var sqliteSQL []byte
