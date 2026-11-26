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

func supportEmailConfirmation(db *datastore) error {
	t, err := db.Begin()
	if err != nil {
		t.Rollback()
		return err
	}

	// Tokens aren't tied to a stored email address, so we never keep a cleartext copy of it. Instead, a user's pending
	// tokens are replaced whenever they change their address.
	_, err = t.Exec(`CREATE TABLE email_confirmations (
    user_id   ` + db.typeInt() + ` not null,
    token     ` + db.typeChar(32) + ` not null primary key,
    confirmed ` + db.typeBool() + ` default 0 not null,
    created   ` + db.typeDateTime() + ` not null
)` + db.engine())
	if err != nil {
		t.Rollback()
		return err
	}

	_, err = t.Exec("CREATE INDEX email_confirmations_user_index ON email_confirmations (`user_id`)")
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
