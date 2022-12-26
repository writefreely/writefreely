//go:build sqlite && !wflib
// +build sqlite,!wflib

/*
 * Copyright © 2019-2020 Musing Studio LLC.
 *
 * This file is part of WriteFreely.
 *
 * WriteFreely is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License, included
 * in the LICENSE file in this source code package.
 */

package writefreely

import (
	"database/sql/driver"
	"errors"
	"regexp"

	"github.com/go-sql-driver/mysql"
	"github.com/writeas/web-core/log"
	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

func init() {
	SQLiteEnabled = true

	sqlite_regex := func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
		if len(args) < 2 {
			return nil, errors.New("wrong number of arguments to SQLite regexp function")
		}
		res, ok := args[0].(string)
		if !ok {
			return nil, errors.New("bad argument 1 to SQLite regexp function, expected string")
		}
		val, ok := args[0].(string)
		if !ok {
			return nil, errors.New("bad argument 2 to SQLite regexp function, expected string")
		}
		return regexp.MatchString(res, val)
	}
	sqlite.MustRegisterDeterministicScalarFunction("regexp", 2, sqlite_regex)
}

func (db *datastore) isDuplicateKeyErr(err error) bool {
	if db.driverName == driverSQLite {
		if err, ok := err.(*sqlite.Error); ok {
			return err.Code() == sqlite3.SQLITE_CONSTRAINT
		}
	} else if db.driverName == driverMySQL {
		if mysqlErr, ok := err.(*mysql.MySQLError); ok {
			return mysqlErr.Number == mySQLErrDuplicateKey
		}
	} else {
		log.Error("isDuplicateKeyErr: failed check for unrecognized driver '%s'", db.driverName)
	}

	return false
}

func (db *datastore) isIgnorableError(err error) bool {
	if db.driverName == driverMySQL {
		if mysqlErr, ok := err.(*mysql.MySQLError); ok {
			return mysqlErr.Number == mySQLErrCollationMix
		}
	} else {
		log.Error("isIgnorableError: failed check for unrecognized driver '%s'", db.driverName)
	}

	return false
}

func (db *datastore) isHighLoadError(err error) bool {
	if db.driverName == driverMySQL {
		if mysqlErr, ok := err.(*mysql.MySQLError); ok {
			return mysqlErr.Number == mySQLErrMaxUserConns || mysqlErr.Number == mySQLErrTooManyConns
		}
	}

	return false
}
