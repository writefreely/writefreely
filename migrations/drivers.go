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
	switch db.driverName {
	case driverSQLite:
		return "strftime('%Y-%m-%d %H:%M:%S','now')"
	case driverMySQL:
		return "NOW()"
	case driverPostgres:
		return "NOW()"
	}

	return "" // placeholder
}

func (db *datastore) typeInt() string {
	switch db.driverName {
	case driverSQLite:
		return "INTEGER"
	case driverMySQL:
		return "INT"
	case driverPostgres:
		return "INT"
	}

	return "" // placeholder
}

func (db *datastore) typeSmallInt() string {
	switch db.driverName {
	case driverSQLite:
		return "INTEGER"
	case driverMySQL:
		return "SMALLINT"
	case driverPostgres:
		return "SMALLINT"
	}

	return "" // placeholder
}

func (db *datastore) typeTinyInt() string {
	switch db.driverName {
	case driverSQLite:
		return "INTEGER"
	case driverMySQL:
		return "TINYINT"
	case driverPostgres:
		return "SMALLINT"
	}

	return "" // placeholder
}

func (db *datastore) typeText() string {
	return "TEXT"
}

func (db *datastore) typeChar(l int) string {
	switch db.driverName {
	case driverSQLite:
		return "TEXT"
	case driverMySQL:
		return fmt.Sprintf("CHAR(%d)", l)
	case driverPostgres:
		return fmt.Sprintf("CHAR(%d)", l)
	}

	return "" // placeholder
}

func (db *datastore) typeVarChar(l int) string {
	switch db.driverName {
	case driverSQLite:
		return "TEXT"
	case driverMySQL:
		return fmt.Sprintf("VARCHAR(%d)", l)
	case driverPostgres:
		return fmt.Sprintf("VARCHAR(%d)", l)
	}

	return "" // placeholder
}

func (db *datastore) typeVarBinary(l int) string {
	switch db.driverName {
	case driverSQLite:
		return "BLOB"
	case driverMySQL:
		return fmt.Sprintf("VARBINARY(%d)", l)
	case driverPostgres:
		return "BYTEA"
	}

	return "" // placeholder
}

func (db *datastore) typeBool() string {
	switch db.driverName {
	case driverSQLite:
		return "INTEGER"
	case driverMySQL:
		return "TINYINT(1)"
	case driverPostgres:
		return "BOOLEAN"
	}

	return "" // placeholder
}

func (db *datastore) typeDateTime() string {
	switch db.driverName {
	case driverSQLite:
		return "DATETIME"
	case driverMySQL:
		return "DATETIME"
	case driverPostgres:
		return "TIMESTAMP"
	}

	return "" // placeholder
}

func (db *datastore) typeIntPrimaryKey() string {
	switch db.driverName {
	case driverSQLite:
		// From docs: "In SQLite, a column with type INTEGER PRIMARY KEY is an alias for the ROWID (except in WITHOUT
		// ROWID tables) which is always a 64-bit signed integer."
		return "INTEGER PRIMARY KEY"
	case driverMySQL:
		return "INT AUTO_INCREMENT PRIMARY KEY"
	case driverPostgres:
		return "SERIAL PRIMARY KEY"
	}

	return "" // placeholder
}

func (db *datastore) collateMultiByte() string {
	switch db.driverName {
	case driverSQLite:
		return ""
	case driverMySQL:
		return " COLLATE utf8_bin"
	case driverPostgres:
		return ""
	}

	return "" // placeholder
}

func (db *datastore) engine() string {
	switch db.driverName {
	case driverSQLite:
		return ""
	case driverMySQL:
		return " ENGINE = InnoDB"
	case driverPostgres:
		return ""
	}

	return "" // placeholder
}

func (db *datastore) after(colName string) string {
	switch db.driverName {
	case driverSQLite:
		return ""
	case driverMySQL:
		return fmt.Sprintf(" AFTER %s", colName)
	case driverPostgres:
		return ""
	}

	return "" // placeholder
}

func (db *datastore) boolTrue() string {
	switch db.driverName {
	case driverSQLite:
		return "1"
	case driverMySQL:
		return "1"
	case driverPostgres:
		return "TRUE"
	}

	return "" // placeholder
}

func (db *datastore) boolFalse() string {
	switch db.driverName {
	case driverSQLite:
		return "0"
	case driverMySQL:
		return "0"
	case driverPostgres:
		return "FALSE"
	}

	return "" // placeholder
}

func (db *datastore) limit(offset int, size int) string {
	switch db.driverName {
	case driverSQLite, driverMySQL:
		return fmt.Sprintf(" LIMIT %d, %d", offset, size)
	case driverPostgres:
		return fmt.Sprintf(" LIMIT %d OFFSET %d", size, offset)
	}

	return "" // placeholder
}

func (db *datastore) QueryWrap(q string) string {
	if db.driverName != driverPostgres {
		return q
	}

	output := ""
	escape := false
	ctr := 0

	for i := range len(q) {
		if q[i] == '\'' || q[i] == '`' {
			escape = !escape
		}

		if q[i] == '?' && !escape {
			ctr += 1
			output += fmt.Sprintf("$%d", ctr)
		} else {
			output += string(q[i])
		}
	}

	return output
}