/*
 * Copyright © 2021 Musing Studio LLC.
 *
 * This file is part of WriteFreely.
 *
 * WriteFreely is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License, included
 * in the LICENSE file in this source code package.
 */

package migrations

func supportLetters(db *datastore) error {
	t, err := db.Begin()
	if err != nil {
		t.Rollback()
		return err
	}

	_, err = t.Exec(`CREATE TABLE publishjobs (
    id      ` + db.typeIntPrimaryKey() + `,
    post_id ` + db.typeVarChar(16) + ` not null,
    action  ` + db.typeVarChar(16) + ` not null,
    delay   ` + db.typeTinyInt() + ` not null
)`)
	if err != nil {
		t.Rollback()
		return err
	}

	_, err = t.Exec(`CREATE TABLE emailsubscribers (
    id            ` + db.typeChar(8) + ` not null,
    collection_id ` + db.typeInt() + ` not null,
    user_id       ` + db.typeInt() + ` null,
    email         ` + db.typeVarChar(255) + ` null,
    subscribed    ` + db.typeDateTime() + ` not null,
    token         ` + db.typeChar(16) + ` not null,
    confirmed     ` + db.typeBool() + ` default 0 not null,
    allow_export  ` + db.typeBool() + ` default 0 not null,
    constraint eu_coll_email
        unique (collection_id, email),
    constraint eu_coll_user
        unique (collection_id, user_id),
	PRIMARY KEY (id)
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
