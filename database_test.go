package writefreely

import (
	"context"
	"database/sql"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestOAuthDatastore(t *testing.T) {
	if !runMySQLTests() {
		t.Skip("skipping mysql tests")
	}
	withTestDB(t, func(db *sql.DB) {
		ctx := context.Background()
		ds := &datastore{
			DB:         db,
			driverName: "",
		}

		state, err := ds.GenerateOAuthState(ctx)
		assert.NoError(t, err)
		assert.Len(t, state, 24)

		countRows(t, ctx, db, 1, "SELECT COUNT(*) FROM `oauth_client_states` WHERE `state` = ? AND `used` = false", state)

		err = ds.ValidateOAuthState(ctx, state)
		assert.NoError(t, err)

		countRows(t, ctx, db, 1, "SELECT COUNT(*) FROM `oauth_client_states` WHERE `state` = ? AND `used` = true", state)

		var localUserID int64 = 99
		var remoteUserID int64 = 100
		err = ds.RecordRemoteUserID(ctx, localUserID, remoteUserID)
		assert.NoError(t, err)

		countRows(t, ctx, db, 1, "SELECT COUNT(*) FROM `oauth_users` WHERE `user_id` = ? AND `remote_user_id` = ?", localUserID, remoteUserID)

		foundUserID, err := ds.GetIDForRemoteUser(ctx, remoteUserID)
		assert.NoError(t, err)
		assert.Equal(t, localUserID, foundUserID)
	})
}
