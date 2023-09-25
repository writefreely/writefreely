/*
 * Copyright © 2021 A Bunch Tell LLC.
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
    id ` + db.typeInt() + ` auto_increment,
    post_id ` + db.typeVarChar(16) + ` not null,
    action  ` + db.typeVarChar(16) + ` not null,
    delay   ` + db.typeTinyInt() + ` not null,
	PRIMARY KEY (id)
)`)
	if err != nil {
		t.Rollback()
		return err
	}

	// TODO: fix for SQLite database
	_, err = t.Exec(`CREATE TABLE emailsubscribers (
    id            char(8)              not null,
    collection_id int                  not null,
    user_id       int                  null,
    email         varchar(255)         null,
    subscribed    datetime             not null,
    token         char(16)             not null,
    confirmed     tinyint(1) default 0 not null,
    allow_export  tinyint(1) default 0 not null,
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
