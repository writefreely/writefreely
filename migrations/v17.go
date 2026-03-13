/*
 * Copyright © 2026 Musing Studio LLC.
 *
 * This file is part of WriteFreely.
 *
 * WriteFreely is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License, included
 * in the LICENSE file in this source code package.
 */

package migrations

func fixPostSignatureCharset(db *datastore) error {
	// Only run this migration on MySQL databases
	if db.driverName != driverMySQL {
		return nil
	}

	t, err := db.Begin()
	if err != nil {
		t.Rollback()
		return err
	}

	_, err = t.Exec(`ALTER TABLE collections MODIFY post_signature TEXT CHARACTER SET utf8mb4 COLLATE utf8mb4_bin;`)
	if err != nil {
		t.Rollback()
		return err
	}

	err = t.Commit()
	if err != nil {
		t.Rollback()
		return err
	}

	return nil
}
