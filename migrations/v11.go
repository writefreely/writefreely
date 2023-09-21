/*
 * Copyright © 2020 Musing Studio LLC.
 *
 * This file is part of WriteFreely.
 *
 * WriteFreely is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License, included
 * in the LICENSE file in this source code package.
 */

package migrations

/**
 * Widen `oauth_users.access_token`, necessary only for mysql
 */
func widenOauthAcceesToken(db *datastore) error {
	if db.driverName == driverMySQL {
		t, err := db.Begin()
		if err != nil {
			t.Rollback()
			return err
		}

		_, err = t.Exec(`ALTER TABLE oauth_users MODIFY COLUMN access_token ` + db.typeText() + db.collateMultiByte() + ` NULL`)
		if err != nil {
			t.Rollback()
			return err
		}

		err = t.Commit()
		if err != nil {
			t.Rollback()
			return err
		}
	}

	return nil
}
