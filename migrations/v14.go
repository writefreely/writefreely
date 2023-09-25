/*
 * Copyright © 2023 Musing Studio LLC.
 *
 * This file is part of WriteFreely.
 *
 * WriteFreely is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License, included
 * in the LICENSE file in this source code package.
 */

package migrations

func supportPassReset(db *datastore) error {
	t, err := db.Begin()
	if err != nil {
		t.Rollback()
		return err
	}

	_, err = t.Exec(`CREATE TABLE password_resets (
    user_id ` + db.typeInt() + ` not null,
    token   ` + db.typeChar(32) + ` not null primary key,
    used    ` + db.typeBool() + ` default 0 not null,
    created ` + db.typeDateTime() + ` not null
)`)
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
