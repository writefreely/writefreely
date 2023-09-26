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

func addPostRetrievalIndex(db *datastore) error {
	t, err := db.Begin()
	if err != nil {
		t.Rollback()
		return err
	}

	_, err = t.Exec("CREATE INDEX posts_get_collection_index ON posts (`collection_id`, `pinned_position`, `created`)")
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
