/*
 * Copyright © 2024 Musing Studio LLC.
 *
 * This file is part of WriteFreely.
 *
 * WriteFreely is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License, included
 * in the LICENSE file in this source code package.
 */

package migrations

func supportRemoteLikes(db *datastore) error {
	t, err := db.Begin()
	if err != nil {
		t.Rollback()
		return err
	}

	_, err = t.Exec(`CREATE TABLE remote_likes (
	post_id        ` + db.typeChar(16) + ` NOT NULL,
	remote_user_id ` + db.typeInt() + ` NOT NULL,
	created        ` + db.typeDateTime() + ` NOT NULL,
	PRIMARY KEY (post_id,remote_user_id)
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
