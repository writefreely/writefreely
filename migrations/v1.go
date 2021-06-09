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

func supportUserInvites(db *datastore) error {
	t, err := db.Begin()
	if err != nil {
		return err
	}
	_, err = t.Exec(`CREATE TABLE userinvites (
		  id ` + db.typeChar(6) + ` NOT NULL ,
		  owner_id ` + db.typeInt() + ` NOT NULL ,
		  max_uses ` + db.typeSmallInt() + ` NULL ,
		  created ` + db.typeDateTime() + ` NOT NULL ,
		  expires ` + db.typeDateTime() + ` NULL ,
		  inactive ` + db.typeBool() + ` NOT NULL ,
		  PRIMARY KEY (id)
		) ` + db.engine() + `;`)
	if err != nil {
		t.Rollback()
		return err
	}

	_, err = t.Exec(`CREATE TABLE usersinvited (
		  invite_id ` + db.typeChar(6) + ` NOT NULL ,
		  user_id ` + db.typeInt() + ` NOT NULL ,
		  PRIMARY KEY (invite_id, user_id)
		) ` + db.engine() + `;`)
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
