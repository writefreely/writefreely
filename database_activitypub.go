/*
 * Copyright © 2024 Musing Studio LLC.
 *
 * This file is part of WriteFreely.
 *
 * WriteFreely is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License, included
 * in the LICENSE file in this source code package.
 */

package writefreely

import (
	"database/sql"
	"fmt"
	"github.com/writeas/web-core/activitystreams"
	"github.com/writeas/web-core/log"
)

func apAddRemoteUser(app *App, t *sql.Tx, fullActor *activitystreams.Person) (int64, error) {
	// Add remote user locally, since it wasn't found before
	res, err := t.Exec("INSERT INTO remoteusers (actor_id, inbox, shared_inbox, url) VALUES (?, ?, ?, ?)", fullActor.ID, fullActor.Inbox, fullActor.Endpoints.SharedInbox, fullActor.URL)
	if err != nil {
		t.Rollback()
		return -1, fmt.Errorf("couldn't add new remoteuser in DB: %v", err)
	}

	remoteUserID, err := res.LastInsertId()
	if err != nil {
		t.Rollback()
		return -1, fmt.Errorf("no lastinsertid for followers, rolling back: %v", err)
	}

	// Add in key
	_, err = t.Exec("INSERT INTO remoteuserkeys (id, remote_user_id, public_key) VALUES (?, ?, ?)", fullActor.PublicKey.ID, remoteUserID, fullActor.PublicKey.PublicKeyPEM)
	if err != nil {
		if !app.db.isDuplicateKeyErr(err) {
			t.Rollback()
			log.Error("Couldn't add follower keys in DB: %v\n", err)
			return -1, fmt.Errorf("couldn't add follower keys in DB: %v", err)
		} else {
			t.Rollback()
			log.Error("Couldn't add follower keys in DB: %v\n", err)
			return -1, fmt.Errorf("couldn't add follower keys in DB: %v", err)
		}
	}

	return remoteUserID, nil
}
