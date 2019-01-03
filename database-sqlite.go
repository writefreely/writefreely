// +build sqlite

/*
 * Copyright © 2019 A Bunch Tell LLC.
 *
 * This file is part of WriteFreely.
 *
 * WriteFreely is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License, included
 * in the LICENSE file in this source code package.
 */

package writefreely

import (
	"github.com/go-sql-driver/mysql"
	"github.com/mattn/go-sqlite3"
	"github.com/writeas/web-core/log"
)

func init() {
	SQLiteEnabled = true
}

func (db *datastore) isDuplicateKeyErr(err error) bool {
	if db.driverName == driverSQLite {
		if err, ok := err.(sqlite3.Error); ok {
			return err.Code == sqlite3.ErrConstraint
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
