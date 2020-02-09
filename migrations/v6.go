/*
 * Copyright © 2019 A Bunch Tell LLC.
 *
 * This file is part of WriteFreely.
 *
 * WriteFreely is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License, included
 * in the LICENSE file in this source code package.
 */

package migrations

func supportActivityPubMentions(db *datastore) error {
	t, err := db.Begin()

	_, err = t.Exec(`ALTER TABLE remoteusers ADD COLUMN handle ` + db.typeVarChar(255) + ` DEFAULT '' NOT NULL`)
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
