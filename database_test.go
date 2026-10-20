package writefreely

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
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

		state, err := ds.GenerateOAuthState(ctx, "test", "development", 0, "")
		assert.NoError(t, err)
		assert.Len(t, state, 24)

		countRows(t, ctx, db, 1, "SELECT COUNT(*) FROM `oauth_client_states` WHERE `state` = ? AND `used` = false", state)

		_, _, _, _, err = ds.ValidateOAuthState(ctx, state)
		assert.NoError(t, err)

		countRows(t, ctx, db, 1, "SELECT COUNT(*) FROM `oauth_client_states` WHERE `state` = ? AND `used` = true", state)

		var localUserID int64 = 99
		var remoteUserID = "100"
		err = ds.RecordRemoteUserID(ctx, localUserID, remoteUserID, "test", "test", "access_token_a")
		assert.NoError(t, err)

		countRows(t, ctx, db, 1, "SELECT COUNT(*) FROM `oauth_users` WHERE `user_id` = ? AND `remote_user_id` = ? AND access_token = 'access_token_a'", localUserID, remoteUserID)

		err = ds.RecordRemoteUserID(ctx, localUserID, remoteUserID, "test", "test", "access_token_b")
		assert.NoError(t, err)

		countRows(t, ctx, db, 1, "SELECT COUNT(*) FROM `oauth_users` WHERE `user_id` = ? AND `remote_user_id` = ? AND access_token = 'access_token_b'", localUserID, remoteUserID)

		countRows(t, ctx, db, 1, "SELECT COUNT(*) FROM `oauth_users`")

		foundUserID, err := ds.GetIDForRemoteUser(ctx, remoteUserID, "test", "test")
		assert.NoError(t, err)
		assert.Equal(t, localUserID, foundUserID)
	})
}

func TestQueryWrap(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain placeholders", "SELECT a FROM t WHERE b = ? AND c = ?", "SELECT a FROM t WHERE b = $1 AND c = $2"},
		{"placeholder inside single quotes", "SELECT '?' FROM t WHERE b = ?", "SELECT '?' FROM t WHERE b = $1"},
		{"backtick inside single quotes", "SELECT '`' FROM t WHERE b = ?", "SELECT '`' FROM t WHERE b = $1"},
		{"single quote inside backticks", "SELECT `it's` FROM t WHERE b = ?", "SELECT `it's` FROM t WHERE b = $1"},
		{"placeholder inside double quotes", "SELECT \"a?b\" FROM t WHERE b = ?", "SELECT \"a?b\" FROM t WHERE b = $1"},
		{"doubled single quote", "SELECT 'it''s ?' FROM t WHERE b = ?", "SELECT 'it''s ?' FROM t WHERE b = $1"},
	}

	pg := &datastore{driverName: driverPostgres}
	other := &datastore{driverName: "sqlite3"}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pg.QueryWrap(tt.in); got != tt.want {
				t.Errorf("postgres QueryWrap(%q) = %q, want %q", tt.in, got, tt.want)
			}
			if got := other.QueryWrap(tt.in); got != tt.in {
				t.Errorf("non-postgres QueryWrap(%q) = %q, want unchanged", tt.in, got)
			}
		})
	}
}
