/*
 * Copyright © 2019 Musing Studio LLC.
 *
 * This file is part of WriteFreely.
 *
 * WriteFreely is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License, included
 * in the LICENSE file in this source code package.
 */

package migrations

import (
	"fmt"
)

// TODO: use now() from writefreely pkg
func (db *datastore) now() string {
	if db.driverName == driverSQLite {
		return "strftime('%Y-%m-%d %H:%M:%S','now')"
	}
	return "NOW()"
}

func (db *datastore) typeInt() string {
	if db.driverName == driverSQLite {
		return "INTEGER"
	}
	return "INT"
}

func (db *datastore) typeSmallInt() string {
	if db.driverName == driverSQLite {
		return "INTEGER"
	}
	return "SMALLINT"
}

func (db *datastore) typeTinyInt() string {
	if db.driverName == driverSQLite {
		return "INTEGER"
	}
	return "TINYINT"
}

func (db *datastore) typeText() string {
	return "TEXT"
}

func (db *datastore) typeChar(l int) string {
	if db.driverName == driverSQLite {
		return "TEXT"
	}
	return fmt.Sprintf("CHAR(%d)", l)
}

func (db *datastore) typeVarChar(l int) string {
	if db.driverName == driverSQLite {
		return "TEXT"
	}
	return fmt.Sprintf("VARCHAR(%d)", l)
}

func (db *datastore) typeVarBinary(l int) string {
	if db.driverName == driverSQLite {
		return "BLOB"
	}
	return fmt.Sprintf("VARBINARY(%d)", l)
}

func (db *datastore) typeBool() string {
	if db.driverName == driverSQLite {
		return "INTEGER"
	}
	return "TINYINT(1)"
}

func (db *datastore) typeDateTime() string {
	return "DATETIME"
}

func (db *datastore) typeIntPrimaryKey() string {
	if db.driverName == driverSQLite {
		// From docs: "In SQLite, a column with type INTEGER PRIMARY KEY is an alias for the ROWID (except in WITHOUT
		// ROWID tables) which is always a 64-bit signed integer."
		return "INTEGER PRIMARY KEY"
	}
	return "INT AUTO_INCREMENT PRIMARY KEY"
}

func (db *datastore) collateMultiByte() string {
	if db.driverName == driverSQLite {
		return ""
	}
	return " COLLATE utf8_bin"
}

func (db *datastore) engine() string {
	if db.driverName == driverSQLite {
		return ""
	}
	return " ENGINE = InnoDB"
}

func (db *datastore) after(colName string) string {
	if db.driverName == driverSQLite {
		return ""
	}
	return " AFTER " + colName
}
