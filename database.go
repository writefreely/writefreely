/*
 * Copyright © 2018-2021 Musing Studio LLC.
 *
 * This file is part of WriteFreely.
 *
 * WriteFreely is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License, included
 * in the LICENSE file in this source code package.
 */

package writefreely

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/writeas/web-core/silobridge"
	wf_db "github.com/writefreely/writefreely/db"
	"github.com/writefreely/writefreely/parse"

	"github.com/guregu/null"
	"github.com/guregu/null/zero"
	uuid "github.com/nu7hatch/gouuid"
	"github.com/writeas/activityserve"
	"github.com/writeas/impart"
	"github.com/writeas/web-core/activitypub"
	"github.com/writeas/web-core/auth"
	"github.com/writeas/web-core/data"
	"github.com/writeas/web-core/id"
	"github.com/writeas/web-core/log"
	"github.com/writeas/web-core/query"
	"github.com/writefreely/writefreely/author"
	"github.com/writefreely/writefreely/config"
	"github.com/writefreely/writefreely/key"
)

const (
	mySQLErrDuplicateKey = 1062
	mySQLErrCollationMix = 1267
	mySQLErrTooManyConns = 1040
	mySQLErrMaxUserConns = 1203

	driverMySQL  = "mysql"
	driverSQLite = "sqlite3"
)

var (
	SQLiteEnabled bool
)

type writestore interface {
	CreateUser(*config.Config, *User, string, string) error
	UpdateUserEmail(keys *key.Keychain, userID int64, email string) error
	UpdateEncryptedUserEmail(int64, []byte) error
	GetUserByID(int64) (*User, error)
	GetUserForAuth(string) (*User, error)
	GetUserForAuthByID(int64) (*User, error)
	GetUserNameFromToken(string) (string, error)
	GetUserDataFromToken(string) (int64, string, error)
	GetAPIUser(header string) (*User, error)
	GetUserID(accessToken string) int64
	GetUserIDPrivilege(accessToken string) (userID int64, sudo bool)
	DeleteToken(accessToken []byte) error
	FetchLastAccessToken(userID int64) string
	GetAccessToken(userID int64) (string, error)
	GetTemporaryAccessToken(userID int64, validSecs int) (string, error)
	GetTemporaryOneTimeAccessToken(userID int64, validSecs int, oneTime bool) (string, error)
	DeleteAccount(userID int64) error
	ChangeSettings(app *App, u *User, s *userSettings) error
	ChangePassphrase(userID int64, sudo bool, curPass string, hashedPass []byte) error

	GetCollections(u *User, hostName string) (*[]Collection, error)
	GetPublishableCollections(u *User, hostName string) (*[]Collection, error)
	GetMeStats(u *User) userMeStats
	GetTotalCollections() (int64, error)
	GetTotalPosts() (int64, error)
	GetTopPosts(u *User, alias string, hostName string) (*[]PublicPost, error)
	GetAnonymousPosts(u *User, page int) (*[]PublicPost, error)
	GetUserPosts(u *User) (*[]PublicPost, error)

	CreateOwnedPost(post *SubmittedPost, accessToken, collAlias, hostName string) (*PublicPost, error)
	CreatePost(userID, collID int64, post *SubmittedPost) (*Post, error)
	UpdateOwnedPost(post *AuthenticatedPost, userID int64) error
	GetEditablePost(id, editToken string) (*PublicPost, error)
	PostIDExists(id string) bool
	GetPost(id string, collectionID int64) (*PublicPost, error)
	GetOwnedPost(id string, ownerID int64) (*PublicPost, error)
	GetPostProperty(id string, collectionID int64, property string) (interface{}, error)

	CreateCollectionFromToken(*config.Config, string, string, string) (*Collection, error)
	CreateCollection(*config.Config, string, string, int64) (*Collection, error)
	GetCollectionBy(condition string, value interface{}) (*Collection, error)
	GetCollection(alias string) (*Collection, error)
	GetCollectionForPad(alias string) (*Collection, error)
	GetCollectionByID(id int64) (*Collection, error)
	UpdateCollection(app *App, c *SubmittedCollection, alias string) error
	DeleteCollection(alias string, userID int64) error

	UpdatePostPinState(pinned bool, postID string, collID, ownerID, pos int64) error
	GetLastPinnedPostPos(collID int64) int64
	GetPinnedPosts(coll *CollectionObj, includeFuture bool) (*[]PublicPost, error)
	RemoveCollectionRedirect(t *sql.Tx, alias string) error
	GetCollectionRedirect(alias string) (new string)
	IsCollectionAttributeOn(id int64, attr string) bool
	CollectionHasAttribute(id int64, attr string) bool

	CanCollect(cpr *ClaimPostRequest, userID int64) bool
	AttemptClaim(p *ClaimPostRequest, query string, params []interface{}, slugIdx int) (sql.Result, error)
	DispersePosts(userID int64, postIDs []string) (*[]ClaimPostResult, error)
	ClaimPosts(cfg *config.Config, userID int64, collAlias string, posts *[]ClaimPostRequest) (*[]ClaimPostResult, error)

	GetPostLikeCounts(postID string) (int64, error)
	GetPostsCount(c *CollectionObj, includeFuture bool)
	GetPosts(cfg *config.Config, c *Collection, page int, includeFuture, forceRecentFirst, includePinned bool) (*[]PublicPost, error)
	GetAllPostsTaggedIDs(c *Collection, tag string, includeFuture bool) ([]string, error)
	GetPostsTagged(cfg *config.Config, c *Collection, tag string, page int, includeFuture bool) (*[]PublicPost, error)

	GetAPFollowers(c *Collection) (*[]RemoteUser, error)
	GetAPActorKeys(collectionID int64) ([]byte, []byte)
	CreateUserInvite(id string, userID int64, maxUses int, expires *time.Time) error
	GetUserInvites(userID int64) (*[]Invite, error)
	GetUserInvite(id string) (*Invite, error)
	GetUsersInvitedCount(id string) int64
	CreateInvitedUser(inviteID string, userID int64) error

	GetDynamicContent(id string) (*instanceContent, error)
	UpdateDynamicContent(id, title, content, contentType string) error
	GetAllUsers(page uint) (*[]User, error)
	GetAllUsersCount() int64
	GetUserLastPostTime(id int64) (*time.Time, error)
	GetCollectionLastPostTime(id int64) (*time.Time, error)

	GetIDForRemoteUser(context.Context, string, string, string) (int64, error)
	RecordRemoteUserID(context.Context, int64, string, string, string, string) error
	ValidateOAuthState(context.Context, string) (string, string, int64, string, error)
	GenerateOAuthState(context.Context, string, string, int64, string) (string, error)
	GetOauthAccounts(ctx context.Context, userID int64) ([]oauthAccountInfo, error)
	RemoveOauth(ctx context.Context, userID int64, provider string, clientID string, remoteUserID string) error

	DatabaseInitialized() bool
}

type datastore struct {
	*sql.DB
	driverName string
}

var _ writestore = &datastore{}

func (db *datastore) now() string {
	if db.driverName == driverSQLite {
		return "strftime('%Y-%m-%d %H:%M:%S','now')"
	}
	return "NOW()"
}

func (db *datastore) clip(field string, l int) string {
	if db.driverName == driverSQLite {
		return fmt.Sprintf("SUBSTR(%s, 0, %d)", field, l)
	}
	return fmt.Sprintf("LEFT(%s, %d)", field, l)
}

func (db *datastore) upsert(indexedCols ...string) string {
	if db.driverName == driverSQLite {
		// NOTE: SQLite UPSERT syntax only works in v3.24.0 (2018-06-04) or later
		// Leaving this for whenever we can upgrade and include it in our binary
		cc := strings.Join(indexedCols, ", ")
		return "ON CONFLICT(" + cc + ") DO UPDATE SET"
	}
	return "ON DUPLICATE KEY UPDATE"
}

func (db *datastore) dateAdd(l int, unit string) string {
	if db.driverName == driverSQLite {
		return fmt.Sprintf("DATETIME('now', '%d %s')", l, unit)
	}
	return fmt.Sprintf("DATE_ADD(NOW(), INTERVAL %d %s)", l, unit)
}

func (db *datastore) dateSub(l int, unit string) string {
	if db.driverName == driverSQLite {
		return fmt.Sprintf("DATETIME('now', '-%d %s')", l, unit)
	}
	return fmt.Sprintf("DATE_SUB(NOW(), INTERVAL %d %s)", l, unit)
}

// CreateUser creates a new user in the database from the given User, UPDATING it in the process with the user's ID.
func (db *datastore) CreateUser(cfg *config.Config, u *User, collectionTitle string, collectionDesc string) error {
	if db.PostIDExists(u.Username) {
		return impart.HTTPError{http.StatusConflict, "Invalid collection name."}
	}

	// New users get a `users` and `collections` row.
	t, err := db.Begin()
	if err != nil {
		return err
	}

	// 1. Add to `users` table
	// NOTE: Assumes User's Password is already hashed!
	res, err := t.Exec("INSERT INTO users (username, password, email) VALUES (?, ?, ?)", u.Username, u.HashedPass, u.Email)
	if err != nil {
		t.Rollback()
		if db.isDuplicateKeyErr(err) {
			return impart.HTTPError{http.StatusConflict, "Username is already taken."}
		}

		log.Error("Rolling back users INSERT: %v\n", err)
		return err
	}
	u.ID, err = res.LastInsertId()
	if err != nil {
		t.Rollback()
		log.Error("Rolling back after LastInsertId: %v\n", err)
		return err
	}

	// 2. Create user's Collection
	if collectionTitle == "" {
		collectionTitle = u.Username
	}
	res, err = t.Exec("INSERT INTO collections (alias, title, description, privacy, owner_id, view_count) VALUES (?, ?, ?, ?, ?, ?)", u.Username, collectionTitle, collectionDesc, defaultVisibility(cfg), u.ID, 0)
	if err != nil {
		t.Rollback()
		if db.isDuplicateKeyErr(err) {
			return impart.HTTPError{http.StatusConflict, "Username is already taken."}
		}
		log.Error("Rolling back collections INSERT: %v\n", err)
		return err
	}

	db.RemoveCollectionRedirect(t, u.Username)

	err = t.Commit()
	if err != nil {
		t.Rollback()
		log.Error("Rolling back after Commit(): %v\n", err)
		return err
	}

	return nil
}

// FIXME: We're returning errors inconsistently in this file. Do we use Errorf
// for returned value, or impart?
func (db *datastore) UpdateUserEmail(keys *key.Keychain, userID int64, email string) error {
	encEmail, err := data.Encrypt(keys.EmailKey, email)
	if err != nil {
		return fmt.Errorf("Couldn't encrypt email %s: %s\n", email, err)
	}

	return db.UpdateEncryptedUserEmail(userID, encEmail)
}

func (db *datastore) UpdateEncryptedUserEmail(userID int64, encEmail []byte) error {
	_, err := db.Exec("UPDATE users SET email = ? WHERE id = ?", encEmail, userID)
	if err != nil {
		return fmt.Errorf("Unable to update user email: %s", err)
	}

	return nil
}

func (db *datastore) CreateCollectionFromToken(cfg *config.Config, alias, title, accessToken string) (*Collection, error) {
	userID := db.GetUserID(accessToken)
	if userID == -1 {
		return nil, ErrBadAccessToken
	}

	return db.CreateCollection(cfg, alias, title, userID)
}

func (db *datastore) GetUserCollectionCount(userID int64) (uint64, error) {
	var collCount uint64
	err := db.QueryRow("SELECT COUNT(*) FROM collections WHERE owner_id = ?", userID).Scan(&collCount)
	switch {
	case err == sql.ErrNoRows:
		return 0, impart.HTTPError{http.StatusInternalServerError, "Couldn't retrieve user from database."}
	case err != nil:
		log.Error("Couldn't get collections count for user %d: %v", userID, err)
		return 0, err
	}

	return collCount, nil
}

func (db *datastore) CreateCollection(cfg *config.Config, alias, title string, userID int64) (*Collection, error) {
	if db.PostIDExists(alias) {
		return nil, impart.HTTPError{http.StatusConflict, "Invalid collection name."}
	}

	// All good, so create new collection
	res, err := db.Exec("INSERT INTO collections (alias, title, description, privacy, owner_id, view_count) VALUES (?, ?, ?, ?, ?, ?)", alias, title, "", defaultVisibility(cfg), userID, 0)
	if err != nil {
		if db.isDuplicateKeyErr(err) {
			return nil, impart.HTTPError{http.StatusConflict, "Collection already exists."}
		}
		log.Error("Couldn't add to collections: %v\n", err)
		return nil, err
	}

	c := &Collection{
		Alias:       alias,
		Title:       title,
		OwnerID:     userID,
		PublicOwner: false,
		Public:      defaultVisibility(cfg) == CollPublic,
	}

	c.ID, err = res.LastInsertId()
	if err != nil {
		log.Error("Couldn't get collection LastInsertId: %v\n", err)
	}

	return c, nil
}

func (db *datastore) GetUserByID(id int64) (*User, error) {
	u := &User{ID: id}

	err := db.QueryRow("SELECT username, password, email, created, status FROM users WHERE id = ?", id).Scan(&u.Username, &u.HashedPass, &u.Email, &u.Created, &u.Status)
	switch {
	case err == sql.ErrNoRows:
		return nil, ErrUserNotFound
	case err != nil:
		log.Error("Couldn't SELECT user password: %v", err)
		return nil, err
	}

	return u, nil
}

// IsUserSilenced returns true if the user account associated with id is
// currently silenced.
func (db *datastore) IsUserSilenced(id int64) (bool, error) {
	u := &User{ID: id}

	err := db.QueryRow("SELECT status FROM users WHERE id = ?", id).Scan(&u.Status)
	switch {
	case err == sql.ErrNoRows:
		return false, ErrUserNotFound
	case err != nil:
		log.Error("Couldn't SELECT user status: %v", err)
		return false, fmt.Errorf("is user silenced: %v", err)
	}

	return u.IsSilenced(), nil
}

// DoesUserNeedAuth returns true if the user hasn't provided any methods for
// authenticating with the account, such a passphrase or email address.
// Any errors are reported to admin and silently quashed, returning false as the
// result.
func (db *datastore) DoesUserNeedAuth(id int64) bool {
	var pass, email []byte

	// Find out if user has an email set first
	err := db.QueryRow("SELECT password, email FROM users WHERE id = ?", id).Scan(&pass, &email)
	switch {
	case err == sql.ErrNoRows:
		// ERROR. Don't give false positives on needing auth methods
		return false
	case err != nil:
		// ERROR. Don't give false positives on needing auth methods
		log.Error("Couldn't SELECT user %d from users: %v", id, err)
		return false
	}
	// User doesn't need auth if there's an email
	return len(email) == 0 && len(pass) == 0
}

func (db *datastore) IsUserPassSet(id int64) (bool, error) {
	var pass []byte
	err := db.QueryRow("SELECT password FROM users WHERE id = ?", id).Scan(&pass)
	switch {
	case err == sql.ErrNoRows:
		return false, nil
	case err != nil:
		log.Error("Couldn't SELECT user %d from users: %v", id, err)
		return false, err
	}

	return len(pass) > 0, nil
}

func (db *datastore) GetUserForAuth(username string) (*User, error) {
	u := &User{Username: username}

	err := db.QueryRow("SELECT id, password, email, created, status FROM users WHERE username = ?", username).Scan(&u.ID, &u.HashedPass, &u.Email, &u.Created, &u.Status)
	switch {
	case err == sql.ErrNoRows:
		// Check if they've entered the wrong, unnormalized username
		username = getSlug(username, "")
		if username != u.Username {
			err = db.QueryRow("SELECT id FROM users WHERE username = ? LIMIT 1", username).Scan(&u.ID)
			if err == nil {
				return db.GetUserForAuth(username)
			}
		}
		return nil, ErrUserNotFound
	case err != nil:
		log.Error("Couldn't SELECT user password: %v", err)
		return nil, err
	}

	return u, nil
}

func (db *datastore) GetUserForAuthByID(userID int64) (*User, error) {
	u := &User{ID: userID}

	err := db.QueryRow("SELECT id, password, email, created, status FROM users WHERE id = ?", u.ID).Scan(&u.ID, &u.HashedPass, &u.Email, &u.Created, &u.Status)
	switch {
	case err == sql.ErrNoRows:
		return nil, ErrUserNotFound
	case err != nil:
		log.Error("Couldn't SELECT userForAuthByID: %v", err)
		return nil, err
	}

	return u, nil
}

func (db *datastore) GetUserNameFromToken(accessToken string) (string, error) {
	t := auth.GetToken(accessToken)
	if len(t) == 0 {
		return "", ErrNoAccessToken
	}

	var oneTime bool
	var username string
	err := db.QueryRow("SELECT username, one_time FROM accesstokens LEFT JOIN users ON user_id = id WHERE token LIKE ? AND (expires IS NULL OR expires > "+db.now()+")", t).Scan(&username, &oneTime)
	switch {
	case err == sql.ErrNoRows:
		return "", ErrBadAccessToken
	case err != nil:
		return "", ErrInternalGeneral
	}

	// Delete token if it was one-time
	if oneTime {
		db.DeleteToken(t[:])
	}

	return username, nil
}

func (db *datastore) GetUserDataFromToken(accessToken string) (int64, string, error) {
	t := auth.GetToken(accessToken)
	if len(t) == 0 {
		return 0, "", ErrNoAccessToken
	}

	var userID int64
	var oneTime bool
	var username string
	err := db.QueryRow("SELECT user_id, username, one_time FROM accesstokens LEFT JOIN users ON user_id = id WHERE token LIKE ? AND (expires IS NULL OR expires > "+db.now()+")", t).Scan(&userID, &username, &oneTime)
	switch {
	case err == sql.ErrNoRows:
		return 0, "", ErrBadAccessToken
	case err != nil:
		return 0, "", ErrInternalGeneral
	}

	// Delete token if it was one-time
	if oneTime {
		db.DeleteToken(t[:])
	}

	return userID, username, nil
}

func (db *datastore) GetAPIUser(header string) (*User, error) {
	uID := db.GetUserID(header)
	if uID == -1 {
		return nil, fmt.Errorf(ErrUserNotFound.Error())
	}
	return db.GetUserByID(uID)
}

// GetUserID takes a hexadecimal accessToken, parses it into its binary
// representation, and gets any user ID associated with the token. If no user
// is associated, -1 is returned.
func (db *datastore) GetUserID(accessToken string) int64 {
	i, _ := db.GetUserIDPrivilege(accessToken)
	return i
}

func (db *datastore) GetUserIDPrivilege(accessToken string) (userID int64, sudo bool) {
	t := auth.GetToken(accessToken)
	if len(t) == 0 {
		return -1, false
	}

	var oneTime bool
	err := db.QueryRow("SELECT user_id, sudo, one_time FROM accesstokens WHERE token LIKE ? AND (expires IS NULL OR expires > "+db.now()+")", t).Scan(&userID, &sudo, &oneTime)
	switch {
	case err == sql.ErrNoRows:
		return -1, false
	case err != nil:
		return -1, false
	}

	// Delete token if it was one-time
	if oneTime {
		db.DeleteToken(t[:])
	}

	return
}

func (db *datastore) DeleteToken(accessToken []byte) error {
	res, err := db.Exec("DELETE FROM accesstokens WHERE token LIKE ?", accessToken)
	if err != nil {
		return err
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return impart.HTTPError{http.StatusNotFound, "Token is invalid or doesn't exist"}
	}
	return nil
}

// FetchLastAccessToken creates a new non-expiring, valid access token for the given
// userID.
func (db *datastore) FetchLastAccessToken(userID int64) string {
	var t []byte
	err := db.QueryRow("SELECT token FROM accesstokens WHERE user_id = ? AND (expires IS NULL OR expires > "+db.now()+") ORDER BY created DESC LIMIT 1", userID).Scan(&t)
	switch {
	case err == sql.ErrNoRows:
		return ""
	case err != nil:
		log.Error("Failed selecting from accesstoken: %v", err)
		return ""
	}

	u, err := uuid.Parse(t)
	if err != nil {
		return ""
	}
	return u.String()
}

// GetAccessToken creates a new non-expiring, valid access token for the given
// userID.
func (db *datastore) GetAccessToken(userID int64) (string, error) {
	return db.GetTemporaryOneTimeAccessToken(userID, 0, false)
}

// GetTemporaryAccessToken creates a new valid access token for the given
// userID that remains valid for the given time in seconds. If validSecs is 0,
// the access token doesn't automatically expire.
func (db *datastore) GetTemporaryAccessToken(userID int64, validSecs int) (string, error) {
	return db.GetTemporaryOneTimeAccessToken(userID, validSecs, false)
}

// GetTemporaryOneTimeAccessToken creates a new valid access token for the given
// userID that remains valid for the given time in seconds and can only be used
// once if oneTime is true. If validSecs is 0, the access token doesn't
// automatically expire.
func (db *datastore) GetTemporaryOneTimeAccessToken(userID int64, validSecs int, oneTime bool) (string, error) {
	u, err := uuid.NewV4()
	if err != nil {
		log.Error("Unable to generate token: %v", err)
		return "", err
	}

	// Insert UUID to `accesstokens`
	binTok := u[:]

	expirationVal := "NULL"
	if validSecs > 0 {
		expirationVal = db.dateAdd(validSecs, "SECOND")
	}

	_, err = db.Exec("INSERT INTO accesstokens (token, user_id, one_time, expires) VALUES (?, ?, ?, "+expirationVal+")", string(binTok), userID, oneTime)
	if err != nil {
		log.Error("Couldn't INSERT accesstoken: %v", err)
		return "", err
	}

	return u.String(), nil
}

func (db *datastore) CreatePasswordResetToken(userID int64) (string, error) {
	t := id.Generate62RandomString(32)

	_, err := db.Exec("INSERT INTO password_resets (user_id, token, used, created) VALUES (?, ?, 0, "+db.now()+")", userID, t)
	if err != nil {
		log.Error("Couldn't INSERT password_resets: %v", err)
		return "", err
	}

	return t, nil
}

func (db *datastore) GetUserFromPasswordReset(token string) int64 {
	var userID int64
	err := db.QueryRow("SELECT user_id FROM password_resets WHERE token = ? AND used = 0 AND created > "+db.dateSub(3, "HOUR"), token).Scan(&userID)
	if err != nil {
		return 0
	}
	return userID
}

func (db *datastore) ConsumePasswordResetToken(t string) error {
	_, err := db.Exec("UPDATE password_resets SET used = 1 WHERE token = ?", t)
	if err != nil {
		log.Error("Couldn't UPDATE password_resets: %v", err)
		return err
	}

	return nil
}

func (db *datastore) CreateOwnedPost(post *SubmittedPost, accessToken, collAlias, hostName string) (*PublicPost, error) {
	var userID, collID int64 = -1, -1
	var coll *Collection
	var err error
	if accessToken != "" {
		userID = db.GetUserID(accessToken)
		if userID == -1 {
			return nil, ErrBadAccessToken
		}
		if collAlias != "" {
			coll, err = db.GetCollection(collAlias)
			if err != nil {
				return nil, err
			}
			coll.hostName = hostName
			if coll.OwnerID != userID {
				return nil, ErrForbiddenCollection
			}
			collID = coll.ID
		}
	}

	rp := &PublicPost{}
	rp.Post, err = db.CreatePost(userID, collID, post)
	if err != nil {
		return rp, err
	}
	if coll != nil {
		coll.ForPublic()
		rp.Collection = &CollectionObj{Collection: *coll}
	}
	return rp, nil
}

func (db *datastore) CreatePost(userID, collID int64, post *SubmittedPost) (*Post, error) {
	idLen := postIDLen
	friendlyID := id.GenerateFriendlyRandomString(idLen)

	// Handle appearance / font face
	appearance := post.Font
	if !post.isFontValid() {
		appearance = "norm"
	}

	var err error
	ownerID := sql.NullInt64{
		Valid: false,
	}
	ownerCollID := sql.NullInt64{
		Valid: false,
	}
	slug := sql.NullString{"", false}

	// If an alias was supplied, we'll add this to the collection as well.
	if userID > 0 {
		ownerID.Int64 = userID
		ownerID.Valid = true
		if collID > 0 {
			ownerCollID.Int64 = collID
			ownerCollID.Valid = true
			var slugVal string
			if post.Slug != nil && *post.Slug != "" {
				slugVal = *post.Slug
			} else {
				if post.Title != nil && *post.Title != "" {
					slugVal = getSlug(*post.Title, post.Language.String)
					if slugVal == "" {
						slugVal = getSlug(*post.Content, post.Language.String)
					}
				} else {
					slugVal = getSlug(*post.Content, post.Language.String)
				}
			}
			if slugVal == "" {
				slugVal = friendlyID
			}
			slug = sql.NullString{slugVal, true}
		}
	}

	created := time.Now()
	if db.driverName == driverSQLite {
		// SQLite stores datetimes in UTC, so convert time.Now() to it here
		created = created.UTC()
	}
	if post.Created != nil && *post.Created != "" {
		created, err = time.Parse("2006-01-02T15:04:05Z", *post.Created)
		if err != nil {
			log.Error("Unable to parse Created time '%s': %v", *post.Created, err)
			created = time.Now()
			if db.driverName == driverSQLite {
				// SQLite stores datetimes in UTC, so convert time.Now() to it here
				created = created.UTC()
			}
		}
	}

	stmt, err := db.Prepare("INSERT INTO posts (id, slug, title, content, text_appearance, language, rtl, privacy, owner_id, collection_id, created, updated, view_count) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, " + db.now() + ", ?)")
	if err != nil {
		return nil, err
	}
	defer stmt.Close()
	_, err = stmt.Exec(friendlyID, slug, post.Title, post.Content, appearance, post.Language, post.IsRTL, 0, ownerID, ownerCollID, created, 0)
	if err != nil {
		if db.isDuplicateKeyErr(err) {
			// Duplicate entry error; try a new slug
			// TODO: make this a little more robust
			slug = sql.NullString{id.GenSafeUniqueSlug(slug.String), true}
			_, err = stmt.Exec(friendlyID, slug, post.Title, post.Content, appearance, post.Language, post.IsRTL, 0, ownerID, ownerCollID, created, 0)
			if err != nil {
				return nil, handleFailedPostInsert(fmt.Errorf("Retried slug generation, still failed: %v", err))
			}
		} else {
			return nil, handleFailedPostInsert(err)
		}
	}

	// TODO: return Created field in proper format
	return &Post{
		ID:           friendlyID,
		Slug:         null.NewString(slug.String, slug.Valid),
		Font:         appearance,
		Language:     zero.NewString(post.Language.String, post.Language.Valid),
		RTL:          zero.NewBool(post.IsRTL.Bool, post.IsRTL.Valid),
		OwnerID:      null.NewInt(userID, true),
		CollectionID: null.NewInt(userID, true),
		Created:      created.Truncate(time.Second).UTC(),
		Updated:      time.Now().Truncate(time.Second).UTC(),
		Title:        zero.NewString(*(post.Title), true),
		Content:      *(post.Content),
	}, nil
}

// UpdateOwnedPost updates an existing post with only the given fields in the
// supplied AuthenticatedPost.
func (db *datastore) UpdateOwnedPost(post *AuthenticatedPost, userID int64) error {
	params := []interface{}{}
	var queryUpdates, sep, authCondition string
	if post.Slug != nil && *post.Slug != "" {
		queryUpdates += sep + "slug = ?"
		sep = ", "
		params = append(params, getSlug(*post.Slug, ""))
	}
	if post.Content != nil {
		queryUpdates += sep + "content = ?"
		sep = ", "
		params = append(params, post.Content)
	}
	if post.Title != nil {
		queryUpdates += sep + "title = ?"
		sep = ", "
		params = append(params, post.Title)
	}
	if post.Language.Valid {
		queryUpdates += sep + "language = ?"
		sep = ", "
		params = append(params, post.Language.String)
	}
	if post.IsRTL.Valid {
		queryUpdates += sep + "rtl = ?"
		sep = ", "
		params = append(params, post.IsRTL.Bool)
	}
	if post.Font != "" {
		queryUpdates += sep + "text_appearance = ?"
		sep = ", "
		params = append(params, post.Font)
	}
	if post.Created != nil {
		createTime, err := time.Parse(postMetaDateFormat, *post.Created)
		if err != nil {
			log.Error("Unable to parse Created date: %v", err)
			return fmt.Errorf("That's the incorrect format for Created date.")
		}
		queryUpdates += sep + "created = ?"
		sep = ", "
		params = append(params, createTime)
	}

	// WHERE parameters...
	// id = ?
	params = append(params, post.ID)
	// AND owner_id = ?
	authCondition = "(owner_id = ?)"
	params = append(params, userID)

	if queryUpdates == "" {
		return ErrPostNoUpdatableVals
	}

	queryUpdates += sep + "updated = " + db.now()

	res, err := db.Exec("UPDATE posts SET "+queryUpdates+" WHERE id = ? AND "+authCondition, params...)
	if err != nil {
		log.Error("Unable to update owned post: %v", err)
		return err
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		// Show the correct error message if nothing was updated
		var dummy int
		err := db.QueryRow("SELECT 1 FROM posts WHERE id = ? AND "+authCondition, post.ID, params[len(params)-1]).Scan(&dummy)
		switch {
		case err == sql.ErrNoRows:
			return ErrUnauthorizedEditPost
		case err != nil:
			log.Error("Failed selecting from posts: %v", err)
		}
		return nil
	}

	return nil
}

func (db *datastore) GetCollectionBy(condition string, value interface{}) (*Collection, error) {
	c := &Collection{}

	// FIXME: change Collection to reflect database values. Add helper functions to get actual values
	var styleSheet, script, signature, format zero.String
	row := db.QueryRow("SELECT id, alias, title, description, style_sheet, script, post_signature, format, owner_id, privacy, view_count FROM collections WHERE "+condition, value)

	err := row.Scan(&c.ID, &c.Alias, &c.Title, &c.Description, &styleSheet, &script, &signature, &format, &c.OwnerID, &c.Visibility, &c.Views)
	switch {
	case err == sql.ErrNoRows:
		return nil, impart.HTTPError{http.StatusNotFound, "Collection doesn't exist."}
	case db.isHighLoadError(err):
		return nil, ErrUnavailable
	case err != nil:
		log.Error("Failed selecting from collections: %v", err)
		return nil, err
	}
	c.StyleSheet = styleSheet.String
	c.Script = script.String
	c.Signature = signature.String
	c.Format = format.String
	c.Public = c.IsPublic()
	c.Monetization = db.GetCollectionAttribute(c.ID, "monetization_pointer")
	c.Verification = db.GetCollectionAttribute(c.ID, "verification_link")

	c.db = db

	return c, nil
}

func (db *datastore) GetCollection(alias string) (*Collection, error) {
	return db.GetCollectionBy("alias = ?", alias)
}

func (db *datastore) GetCollectionForPad(alias string) (*Collection, error) {
	c := &Collection{Alias: alias}

	row := db.QueryRow("SELECT id, alias, title, description, privacy FROM collections WHERE alias = ?", alias)

	err := row.Scan(&c.ID, &c.Alias, &c.Title, &c.Description, &c.Visibility)
	switch {
	case err == sql.ErrNoRows:
		return c, impart.HTTPError{http.StatusNotFound, "Collection doesn't exist."}
	case err != nil:
		log.Error("Failed selecting from collections: %v", err)
		return c, ErrInternalGeneral
	}
	c.Public = c.IsPublic()

	return c, nil
}

func (db *datastore) GetCollectionByID(id int64) (*Collection, error) {
	return db.GetCollectionBy("id = ?", id)
}

func (db *datastore) GetCollectionFromDomain(host string) (*Collection, error) {
	return db.GetCollectionBy("host = ?", host)
}

func (db *datastore) UpdateCollection(app *App, c *SubmittedCollection, alias string) error {
	// Truncate fields correctly, so we don't get "Data too long for column" errors in MySQL (writefreely#600)
	if c.Title != nil {
		*c.Title = parse.Truncate(*c.Title, collMaxLengthTitle)
	}
	if c.Description != nil {
		*c.Description = parse.Truncate(*c.Description, collMaxLengthDescription)
	}

	q := query.NewUpdate().
		SetStringPtr(c.Title, "title").
		SetStringPtr(c.Description, "description").
		SetStringPtr(c.StyleSheet, "style_sheet").
		SetStringPtr(c.Script, "script").
		SetStringPtr(c.Signature, "post_signature")

	if c.Format != nil {
		cf := &CollectionFormat{Format: c.Format.String}
		if cf.Valid() {
			q.SetNullString(c.Format, "format")
		}
	}

	var updatePass bool
	if c.Visibility != nil && (collVisibility(*c.Visibility)&CollProtected == 0 || c.Pass != "") {
		q.SetIntPtr(c.Visibility, "privacy")
		if c.Pass != "" {
			updatePass = true
		}
	}

	// WHERE values
	q.Where("alias = ? AND owner_id = ?", alias, c.OwnerID)

	if q.Updates == "" && c.Monetization == nil {
		return ErrPostNoUpdatableVals
	}

	// Find any current domain
	var collID int64
	var rowsAffected int64
	var changed bool
	var res sql.Result
	err := db.QueryRow("SELECT id FROM collections WHERE alias = ?", alias).Scan(&collID)
	if err != nil {
		log.Error("Failed selecting from collections: %v. Some things won't work.", err)
	}

	// Update MathJax value
	if c.MathJax {
		if db.driverName == driverSQLite {
			_, err = db.Exec("INSERT OR REPLACE INTO collectionattributes (collection_id, attribute, value) VALUES (?, ?, ?)", collID, "render_mathjax", "1")
		} else {
			_, err = db.Exec("INSERT INTO collectionattributes (collection_id, attribute, value) VALUES (?, ?, ?) "+db.upsert("collection_id", "attribute")+" value = ?", collID, "render_mathjax", "1", "1")
		}
		if err != nil {
			log.Error("Unable to insert render_mathjax value: %v", err)
			return err
		}
	} else {
		_, err = db.Exec("DELETE FROM collectionattributes WHERE collection_id = ? AND attribute = ?", collID, "render_mathjax")
		if err != nil {
			log.Error("Unable to delete render_mathjax value: %v", err)
			return err
		}
	}

	// Update Verification link value
	if c.Verification != nil {
		skipUpdate := false
		if *c.Verification != "" {
			// Strip away any excess spaces
			trimmed := strings.TrimSpace(*c.Verification)
			if strings.HasPrefix(trimmed, "@") && strings.Count(trimmed, "@") == 2 {
				// This looks like a fediverse handle, so resolve profile URL
				profileURL, err := GetProfileURLFromHandle(app, trimmed)
				if err != nil || profileURL == "" {
					log.Error("Couldn't find user %s: %v", trimmed, err)
					skipUpdate = true
				} else {
					c.Verification = &profileURL
				}
			} else {
				if !strings.HasPrefix(trimmed, "http") {
					trimmed = "https://" + trimmed
				}
				vu, err := url.Parse(trimmed)
				if err != nil {
					// Value appears invalid, so don't update
					skipUpdate = true
				} else {
					s := vu.String()
					c.Verification = &s
				}
			}
		}
		if !skipUpdate {
			err = db.SetCollectionAttribute(collID, "verification_link", *c.Verification)
			if err != nil {
				log.Error("Unable to insert verification_link value: %v", err)
				return err
			}
		}
	}

	// Update Monetization value
	if c.Monetization != nil {
		skipUpdate := false
		if *c.Monetization != "" {
			// Strip away any excess spaces
			trimmed := strings.TrimSpace(*c.Monetization)
			// Only update value when it starts with "$", per spec: https://paymentpointers.org
			if strings.HasPrefix(trimmed, "$") {
				c.Monetization = &trimmed
			} else {
				// Value appears invalid, so don't update
				skipUpdate = true
			}
		}
		if !skipUpdate {
			_, err = db.Exec("INSERT INTO collectionattributes (collection_id, attribute, value) VALUES (?, ?, ?) "+db.upsert("collection_id", "attribute")+" value = ?", collID, "monetization_pointer", *c.Monetization, *c.Monetization)
			if err != nil {
				log.Error("Unable to insert monetization_pointer value: %v", err)
				return err
			}
		}
	}

	// Update EmailSub value
	if c.EmailSubs {
		err = db.SetCollectionAttribute(collID, "email_subs", "1")
		if err != nil {
			log.Error("Unable to insert email_subs value: %v", err)
			return err
		}
		skipUpdate := false
		if c.LetterReply != nil {
			// Strip away any excess spaces
			trimmed := strings.TrimSpace(*c.LetterReply)
			// Only update value when it contains "@"
			if strings.IndexRune(trimmed, '@') > 0 {
				c.LetterReply = &trimmed
			} else {
				// Value appears invalid, so don't update
				skipUpdate = true
			}
			if !skipUpdate {
				err = db.SetCollectionAttribute(collID, collAttrLetterReplyTo, *c.LetterReply)
				if err != nil {
					log.Error("Unable to insert %s value: %v", collAttrLetterReplyTo, err)
					return err
				}
			}
		}
	} else {
		_, err = db.Exec("DELETE FROM collectionattributes WHERE collection_id = ? AND attribute = ?", collID, "email_subs")
		if err != nil {
			log.Error("Unable to delete email_subs value: %v", err)
			return err
		}
	}

	// Update rest of the collection data
	if q.Updates != "" {
		res, err = db.Exec("UPDATE collections SET "+q.Updates+" WHERE "+q.Conditions, q.Params...)
		if err != nil {
			log.Error("Unable to update collection: %v", err)
			return err
		}
	}

	rowsAffected, _ = res.RowsAffected()
	if !changed || rowsAffected == 0 {
		// Show the correct error message if nothing was updated
		var dummy int
		err := db.QueryRow("SELECT 1 FROM collections WHERE alias = ? AND owner_id = ?", alias, c.OwnerID).Scan(&dummy)
		switch {
		case err == sql.ErrNoRows:
			return ErrUnauthorizedEditPost
		case err != nil:
			log.Error("Failed selecting from collections: %v", err)
		}
		if !updatePass {
			return nil
		}
	}

	if updatePass {
		hashedPass, err := auth.HashPass([]byte(c.Pass))
		if err != nil {
			log.Error("Unable to create hash: %s", err)
			return impart.HTTPError{http.StatusInternalServerError, "Could not create password hash."}
		}
		if db.driverName == driverSQLite {
			_, err = db.Exec("INSERT OR REPLACE INTO collectionpasswords (collection_id, password) VALUES ((SELECT id FROM collections WHERE alias = ?), ?)", alias, hashedPass)
		} else {
			_, err = db.Exec("INSERT INTO collectionpasswords (collection_id, password) VALUES ((SELECT id FROM collections WHERE alias = ?), ?) "+db.upsert("collection_id")+" password = ?", alias, hashedPass, hashedPass)
		}
		if err != nil {
			return err
		}
	}

	return nil
}

const postCols = "id, slug, text_appearance, language, rtl, privacy, owner_id, collection_id, pinned_position, created, updated, view_count, title, content"

// getEditablePost returns a PublicPost with the given ID only if the given
// edit token is valid for the post.
func (db *datastore) GetEditablePost(id, editToken string) (*PublicPost, error) {
	// FIXME: code duplicated from getPost()
	// TODO: add slight logic difference to getPost / one func
	var ownerName sql.NullString
	p := &Post{}

	row := db.QueryRow("SELECT "+postCols+", (SELECT username FROM users WHERE users.id = posts.owner_id) AS username FROM posts WHERE id = ? LIMIT 1", id)
	err := row.Scan(&p.ID, &p.Slug, &p.Font, &p.Language, &p.RTL, &p.Privacy, &p.OwnerID, &p.CollectionID, &p.PinnedPosition, &p.Created, &p.Updated, &p.ViewCount, &p.Title, &p.Content, &ownerName)
	switch {
	case err == sql.ErrNoRows:
		return nil, ErrPostNotFound
	case err != nil:
		log.Error("Failed selecting from collections: %v", err)
		return nil, err
	}

	if p.Content == "" && p.Title.String == "" {
		return nil, ErrPostUnpublished
	}

	res := p.processPost()
	if ownerName.Valid {
		res.Owner = &PublicUser{Username: ownerName.String}
	}

	return &res, nil
}

func (db *datastore) PostIDExists(id string) bool {
	var dummy bool
	err := db.QueryRow("SELECT 1 FROM posts WHERE id = ?", id).Scan(&dummy)
	return err == nil && dummy
}

// GetPost gets a public-facing post object from the database. If collectionID
// is > 0, the post will be retrieved by slug and collection ID, rather than
// post ID.
// TODO: break this into two functions:
//   - GetPost(id string)
//   - GetCollectionPost(slug string, collectionID int64)
func (db *datastore) GetPost(id string, collectionID int64) (*PublicPost, error) {
	var ownerName sql.NullString
	p := &Post{}

	var row *sql.Row
	var where string
	params := []interface{}{id}
	if collectionID > 0 {
		where = "slug = ? AND collection_id = ?"
		params = append(params, collectionID)
	} else {
		where = "id = ?"
	}
	row = db.QueryRow("SELECT "+postCols+", (SELECT username FROM users WHERE users.id = posts.owner_id) AS username FROM posts WHERE "+where+" LIMIT 1", params...)
	err := row.Scan(&p.ID, &p.Slug, &p.Font, &p.Language, &p.RTL, &p.Privacy, &p.OwnerID, &p.CollectionID, &p.PinnedPosition, &p.Created, &p.Updated, &p.ViewCount, &p.Title, &p.Content, &ownerName)
	switch {
	case err == sql.ErrNoRows:
		if collectionID > 0 {
			return nil, ErrCollectionPageNotFound
		}
		return nil, ErrPostNotFound
	case err != nil:
		log.Error("Failed selecting from collections: %v", err)
		return nil, err
	}

	if p.Content == "" && p.Title.String == "" {
		return nil, ErrPostUnpublished
	}

	// Get additional information needed before processing post data
	p.LikeCount, err = db.GetPostLikeCounts(p.ID)
	if err != nil {
		return nil, err
	}

	res := p.processPost()
	if ownerName.Valid {
		res.Owner = &PublicUser{Username: ownerName.String}
	}

	return &res, nil
}

// TODO: don't duplicate getPost() functionality
func (db *datastore) GetOwnedPost(id string, ownerID int64) (*PublicPost, error) {
	p := &Post{}

	var row *sql.Row
	where := "id = ? AND owner_id = ?"
	params := []interface{}{id, ownerID}
	row = db.QueryRow("SELECT "+postCols+" FROM posts WHERE "+where+" LIMIT 1", params...)
	err := row.Scan(&p.ID, &p.Slug, &p.Font, &p.Language, &p.RTL, &p.Privacy, &p.OwnerID, &p.CollectionID, &p.PinnedPosition, &p.Created, &p.Updated, &p.ViewCount, &p.Title, &p.Content)
	switch {
	case err == sql.ErrNoRows:
		return nil, ErrPostNotFound
	case err != nil:
		log.Error("Failed selecting from collections: %v", err)
		return nil, err
	}

	if p.Content == "" && p.Title.String == "" {
		return nil, ErrPostUnpublished
	}

	res := p.processPost()

	return &res, nil
}

func (db *datastore) GetPostProperty(id string, collectionID int64, property string) (interface{}, error) {
	propSelects := map[string]string{
		"views": "view_count AS views",
	}
	selectQuery, ok := propSelects[property]
	if !ok {
		return nil, impart.HTTPError{http.StatusBadRequest, fmt.Sprintf("Invalid property: %s.", property)}
	}

	var res interface{}
	var row *sql.Row
	if collectionID != 0 {
		row = db.QueryRow("SELECT "+selectQuery+" FROM posts WHERE slug = ? AND collection_id = ? LIMIT 1", id, collectionID)
	} else {
		row = db.QueryRow("SELECT "+selectQuery+" FROM posts WHERE id = ? LIMIT 1", id)
	}
	err := row.Scan(&res)
	switch {
	case err == sql.ErrNoRows:
		return nil, impart.HTTPError{http.StatusNotFound, "Post not found."}
	case err != nil:
		log.Error("Failed selecting post: %v", err)
		return nil, err
	}

	return res, nil
}

func (db *datastore) GetPostLikeCounts(postID string) (int64, error) {
	var count int64
	err := db.QueryRow("SELECT COUNT(*) FROM remote_likes WHERE post_id = ?", postID).Scan(&count)
	switch {
	case err == sql.ErrNoRows:
		count = 0
	case err != nil:
		return 0, err
	}
	return count, nil
}

// GetPostsCount modifies the CollectionObj to include the correct number of
// standard (non-pinned) posts. It will return future posts if `includeFuture`
// is true.
func (db *datastore) GetPostsCount(c *CollectionObj, includeFuture bool) {
	var count int64
	timeCondition := ""
	if !includeFuture {
		timeCondition = "AND created <= " + db.now()
	}
	err := db.QueryRow("SELECT COUNT(*) FROM posts WHERE collection_id = ? AND pinned_position IS NULL "+timeCondition, c.ID).Scan(&count)
	switch {
	case err == sql.ErrNoRows:
		c.TotalPosts = 0
	case err != nil:
		log.Error("Failed selecting from collections: %v", err)
		c.TotalPosts = 0
	}

	c.TotalPosts = int(count)
}

// GetPosts retrieves all posts for the given Collection.
// It will return future posts if `includeFuture` is true.
// It will include only standard (non-pinned) posts unless `includePinned` is true.
// TODO: change includeFuture to isOwner, since that's how it's used
func (db *datastore) GetPosts(cfg *config.Config, c *Collection, page int, includeFuture, forceRecentFirst, includePinned bool) (*[]PublicPost, error) {
	collID := c.ID

	cf := c.NewFormat()
	order := "DESC"
	if cf.Ascending() && !forceRecentFirst {
		order = "ASC"
	}

	pagePosts := cf.PostsPerPage()
	start := page*pagePosts - pagePosts
	if page == 0 {
		start = 0
		pagePosts = 1000
	}

	limitStr := ""
	if page > 0 {
		limitStr = fmt.Sprintf(" LIMIT %d, %d", start, pagePosts)
	}
	timeCondition := ""
	if !includeFuture {
		timeCondition = "AND created <= " + db.now()
	}
	pinnedCondition := ""
	if !includePinned {
		pinnedCondition = "AND pinned_position IS NULL"
	}
	rows, err := db.Query("SELECT "+postCols+" FROM posts WHERE collection_id = ? "+pinnedCondition+" "+timeCondition+" ORDER BY created "+order+limitStr, collID)
	if err != nil {
		log.Error("Failed selecting from posts: %v", err)
		return nil, impart.HTTPError{http.StatusInternalServerError, "Couldn't retrieve collection posts."}
	}
	defer rows.Close()

	// TODO: extract this common row scanning logic for queries using `postCols`
	posts := []PublicPost{}
	for rows.Next() {
		p := &Post{}
		err = rows.Scan(&p.ID, &p.Slug, &p.Font, &p.Language, &p.RTL, &p.Privacy, &p.OwnerID, &p.CollectionID, &p.PinnedPosition, &p.Created, &p.Updated, &p.ViewCount, &p.Title, &p.Content)
		if err != nil {
			log.Error("Failed scanning row: %v", err)
			break
		}
		p.extractData()
		p.augmentContent(c)
		p.formatContent(cfg, c, includeFuture, false)

		posts = append(posts, p.processPost())
	}
	err = rows.Err()
	if err != nil {
		log.Error("Error after Next() on rows: %v", err)
	}

	return &posts, nil
}

func (db *datastore) GetAllPostsTaggedIDs(c *Collection, tag string, includeFuture bool) ([]string, error) {
	collID := c.ID

	cf := c.NewFormat()
	order := "DESC"
	if cf.Ascending() {
		order = "ASC"
	}

	timeCondition := ""
	if !includeFuture {
		timeCondition = "AND created <= " + db.now()
	}
	var rows *sql.Rows
	var err error
	if db.driverName == driverSQLite {
		rows, err = db.Query("SELECT id FROM posts WHERE collection_id = ? AND LOWER(content) regexp ? "+timeCondition+" ORDER BY created "+order, collID, `.*#`+strings.ToLower(tag)+`\b.*`)
	} else {
		rows, err = db.Query("SELECT id FROM posts WHERE collection_id = ? AND LOWER(content) RLIKE ? "+timeCondition+" ORDER BY created "+order, collID, "#"+strings.ToLower(tag)+"[[:>:]]")
	}
	if err != nil {
		log.Error("Failed selecting tagged posts: %v", err)
		return nil, impart.HTTPError{http.StatusInternalServerError, "Couldn't retrieve tagged collection posts."}
	}
	defer rows.Close()

	ids := []string{}
	for rows.Next() {
		var id string
		err = rows.Scan(&id)
		if err != nil {
			log.Error("Failed scanning row: %v", err)
			break
		}

		ids = append(ids, id)
	}
	err = rows.Err()
	if err != nil {
		log.Error("Error after Next() on rows: %v", err)
	}

	return ids, nil
}

// GetPostsTagged retrieves all posts on the given Collection that contain the
// given tag.
// It will return future posts if `includeFuture` is true.
// TODO: change includeFuture to isOwner, since that's how it's used
func (db *datastore) GetPostsTagged(cfg *config.Config, c *Collection, tag string, page int, includeFuture bool) (*[]PublicPost, error) {
	collID := c.ID

	cf := c.NewFormat()
	order := "DESC"
	if cf.Ascending() {
		order = "ASC"
	}

	pagePosts := cf.PostsPerPage()
	start := page*pagePosts - pagePosts
	if page == 0 {
		start = 0
		pagePosts = 1000
	}

	limitStr := ""
	if page > 0 {
		limitStr = fmt.Sprintf(" LIMIT %d, %d", start, pagePosts)
	}
	timeCondition := ""
	if !includeFuture {
		timeCondition = "AND created <= " + db.now()
	}

	var rows *sql.Rows
	var err error
	if db.driverName == driverSQLite {
		rows, err = db.Query("SELECT "+postCols+" FROM posts WHERE collection_id = ? AND LOWER(content) regexp ? "+timeCondition+" ORDER BY created "+order+limitStr, collID, `.*#`+strings.ToLower(tag)+`\b.*`)
	} else {
		rows, err = db.Query("SELECT "+postCols+" FROM posts WHERE collection_id = ? AND LOWER(content) RLIKE ? "+timeCondition+" ORDER BY created "+order+limitStr, collID, "#"+strings.ToLower(tag)+"[[:>:]]")
	}
	if err != nil {
		log.Error("Failed selecting from posts: %v", err)
		return nil, impart.HTTPError{http.StatusInternalServerError, "Couldn't retrieve collection posts."}
	}
	defer rows.Close()

	// TODO: extract this common row scanning logic for queries using `postCols`
	posts := []PublicPost{}
	for rows.Next() {
		p := &Post{}
		err = rows.Scan(&p.ID, &p.Slug, &p.Font, &p.Language, &p.RTL, &p.Privacy, &p.OwnerID, &p.CollectionID, &p.PinnedPosition, &p.Created, &p.Updated, &p.ViewCount, &p.Title, &p.Content)
		if err != nil {
			log.Error("Failed scanning row: %v", err)
			break
		}
		p.extractData()
		p.augmentContent(c)
		p.formatContent(cfg, c, includeFuture, false)

		posts = append(posts, p.processPost())
	}
	err = rows.Err()
	if err != nil {
		log.Error("Error after Next() on rows: %v", err)
	}

	return &posts, nil
}

func (db *datastore) GetCollLangTotalPosts(collID int64, lang string) (uint64, error) {
	var articles uint64
	err := db.QueryRow("SELECT COUNT(*) FROM posts WHERE collection_id = ? AND language = ? AND created <= "+db.now(), collID, lang).Scan(&articles)
	if err != nil && err != sql.ErrNoRows {
		log.Error("Couldn't get total lang posts count for collection %d: %v", collID, err)
		return 0, err
	}
	return articles, nil
}

func (db *datastore) GetLangPosts(cfg *config.Config, c *Collection, lang string, page int, includeFuture bool) (*[]PublicPost, error) {
	collID := c.ID

	cf := c.NewFormat()
	order := "DESC"
	if cf.Ascending() {
		order = "ASC"
	}

	pagePosts := cf.PostsPerPage()
	start := page*pagePosts - pagePosts
	if page == 0 {
		start = 0
		pagePosts = 1000
	}

	limitStr := ""
	if page > 0 {
		limitStr = fmt.Sprintf(" LIMIT %d, %d", start, pagePosts)
	}
	timeCondition := ""
	if !includeFuture {
		timeCondition = "AND created <= " + db.now()
	}

	rows, err := db.Query(`SELECT `+postCols+`
FROM posts
WHERE collection_id = ? AND language = ? `+timeCondition+`
ORDER BY created `+order+limitStr, collID, lang)
	if err != nil {
		log.Error("Failed selecting from posts: %v", err)
		return nil, impart.HTTPError{http.StatusInternalServerError, "Couldn't retrieve collection posts."}
	}
	defer rows.Close()

	// TODO: extract this common row scanning logic for queries using `postCols`
	posts := []PublicPost{}
	for rows.Next() {
		p := &Post{}
		err = rows.Scan(&p.ID, &p.Slug, &p.Font, &p.Language, &p.RTL, &p.Privacy, &p.OwnerID, &p.CollectionID, &p.PinnedPosition, &p.Created, &p.Updated, &p.ViewCount, &p.Title, &p.Content)
		if err != nil {
			log.Error("Failed scanning row: %v", err)
			break
		}
		p.extractData()
		p.augmentContent(c)
		p.formatContent(cfg, c, includeFuture, false)

		posts = append(posts, p.processPost())
	}
	err = rows.Err()
	if err != nil {
		log.Error("Error after Next() on rows: %v", err)
	}

	return &posts, nil
}

func (db *datastore) GetAPFollowers(c *Collection) (*[]RemoteUser, error) {
	rows, err := db.Query("SELECT actor_id, inbox, shared_inbox, f.created FROM remotefollows f INNER JOIN remoteusers u ON f.remote_user_id = u.id WHERE collection_id = ?", c.ID)
	if err != nil {
		log.Error("Failed selecting from followers: %v", err)
		return nil, impart.HTTPError{http.StatusInternalServerError, "Couldn't retrieve followers."}
	}
	defer rows.Close()

	followers := []RemoteUser{}
	for rows.Next() {
		f := RemoteUser{}
		err = rows.Scan(&f.ActorID, &f.Inbox, &f.SharedInbox, &f.Created)
		followers = append(followers, f)
	}
	return &followers, nil
}

// CanCollect returns whether or not the given user can add the given post to a
// collection. This is true when a post is already owned by the user.
// NOTE: this is currently only used to potentially add owned posts to a
// collection. This has the SIDE EFFECT of also generating a slug for the post.
// FIXME: make this side effect more explicit (or extract it)
func (db *datastore) CanCollect(cpr *ClaimPostRequest, userID int64) bool {
	var title, content string
	var lang sql.NullString
	err := db.QueryRow("SELECT title, content, language FROM posts WHERE id = ? AND owner_id = ?", cpr.ID, userID).Scan(&title, &content, &lang)
	switch {
	case err == sql.ErrNoRows:
		return false
	case err != nil:
		log.Error("Failed on post CanCollect(%s, %d): %v", cpr.ID, userID, err)
		return false
	}

	// Since we have the post content and the post is collectable, generate the
	// post's slug now.
	cpr.Slug = getSlugFromPost(title, content, lang.String)

	return true
}

func (db *datastore) AttemptClaim(p *ClaimPostRequest, query string, params []interface{}, slugIdx int) (sql.Result, error) {
	qRes, err := db.Exec(query, params...)
	if err != nil {
		if db.isDuplicateKeyErr(err) && slugIdx > -1 {
			s := id.GenSafeUniqueSlug(p.Slug)
			if s == p.Slug {
				// Sanity check to prevent infinite recursion
				return qRes, fmt.Errorf("GenSafeUniqueSlug generated nothing unique: %s", s)
			}
			p.Slug = s
			params[slugIdx] = p.Slug
			return db.AttemptClaim(p, query, params, slugIdx)
		}
		return qRes, fmt.Errorf("attemptClaim: %s", err)
	}
	return qRes, nil
}

func (db *datastore) DispersePosts(userID int64, postIDs []string) (*[]ClaimPostResult, error) {
	postClaimReqs := map[string]bool{}
	res := []ClaimPostResult{}
	for i := range postIDs {
		postID := postIDs[i]

		r := ClaimPostResult{Code: 0, ErrorMessage: ""}

		// Perform post validation
		if postID == "" {
			r.ErrorMessage = "Missing post ID. "
		}
		if _, ok := postClaimReqs[postID]; ok {
			r.Code = 429
			r.ErrorMessage = "You've already tried anonymizing this post."
			r.ID = postID
			res = append(res, r)
			continue
		}
		postClaimReqs[postID] = true

		var err error
		// Get full post information to return
		var fullPost *PublicPost
		fullPost, err = db.GetPost(postID, 0)
		if err != nil {
			if err, ok := err.(impart.HTTPError); ok {
				r.Code = err.Status
				r.ErrorMessage = err.Message
				r.ID = postID
				res = append(res, r)
				continue
			} else {
				log.Error("Error getting post in dispersePosts: %v", err)
			}
		}
		if fullPost.OwnerID.Int64 != userID {
			r.Code = http.StatusConflict
			r.ErrorMessage = "Post is already owned by someone else."
			r.ID = postID
			res = append(res, r)
			continue
		}

		var qRes sql.Result
		var query string
		var params []interface{}
		// Do AND owner_id = ? for sanity.
		// This should've been caught and returned with a good error message
		// just above.
		query = "UPDATE posts SET collection_id = NULL WHERE id = ? AND owner_id = ?"
		params = []interface{}{postID, userID}
		qRes, err = db.Exec(query, params...)
		if err != nil {
			r.Code = http.StatusInternalServerError
			r.ErrorMessage = "A glitch happened on our end."
			r.ID = postID
			res = append(res, r)
			log.Error("dispersePosts (post %s): %v", postID, err)
			continue
		}

		// Post was successfully dispersed
		r.Code = http.StatusOK
		r.Post = fullPost

		rowsAffected, _ := qRes.RowsAffected()
		if rowsAffected == 0 {
			// This was already claimed, but return 200
			r.Code = http.StatusOK
		}
		res = append(res, r)
	}

	return &res, nil
}

func (db *datastore) ClaimPosts(cfg *config.Config, userID int64, collAlias string, posts *[]ClaimPostRequest) (*[]ClaimPostResult, error) {
	postClaimReqs := map[string]bool{}
	res := []ClaimPostResult{}
	postCollAlias := collAlias
	for i := range *posts {
		p := (*posts)[i]
		if &p == nil {
			continue
		}

		r := ClaimPostResult{Code: 0, ErrorMessage: ""}

		// Perform post validation
		if p.ID == "" {
			r.ErrorMessage = "Missing post ID `id`. "
		}
		if _, ok := postClaimReqs[p.ID]; ok {
			r.Code = 429
			r.ErrorMessage = "You've already tried claiming this post."
			r.ID = p.ID
			res = append(res, r)
			continue
		}
		postClaimReqs[p.ID] = true

		canCollect := db.CanCollect(&p, userID)
		if !canCollect && p.Token == "" {
			// TODO: ensure post isn't owned by anyone else when a valid modify
			// token is given.
			r.ErrorMessage += "Missing post Edit Token `token`."
		}
		if r.ErrorMessage != "" {
			// Post validate failed
			r.Code = http.StatusBadRequest
			r.ID = p.ID
			res = append(res, r)
			continue
		}

		var err error
		var qRes sql.Result
		var query string
		var params []interface{}
		var slugIdx int = -1
		var coll *Collection
		if collAlias == "" {
			// Posts are being claimed at /posts/claim, not
			// /collections/{alias}/collect, so use given individual collection
			// to associate post with.
			postCollAlias = p.CollectionAlias
		}
		if postCollAlias != "" {
			// Associate this post with a collection
			if p.CreateCollection {
				// This is a new collection
				// TODO: consider removing this. This seriously complicates this
				// method and adds another (unnecessary?) logic path.
				coll, err = db.CreateCollection(cfg, postCollAlias, "", userID)
				if err != nil {
					if err, ok := err.(impart.HTTPError); ok {
						r.Code = err.Status
						r.ErrorMessage = err.Message
					} else {
						r.Code = http.StatusInternalServerError
						r.ErrorMessage = "Unknown error occurred creating collection"
					}
					r.ID = p.ID
					res = append(res, r)
					continue
				}
			} else {
				// Attempt to add to existing collection
				coll, err = db.GetCollection(postCollAlias)
				if err != nil {
					if err, ok := err.(impart.HTTPError); ok {
						if err.Status == http.StatusNotFound {
							// Show obfuscated "forbidden" response, as if attempting to add to an
							// unowned blog.
							r.Code = ErrForbiddenCollection.Status
							r.ErrorMessage = ErrForbiddenCollection.Message
						} else {
							r.Code = err.Status
							r.ErrorMessage = err.Message
						}
					} else {
						r.Code = http.StatusInternalServerError
						r.ErrorMessage = "Unknown error occurred claiming post with collection"
					}
					r.ID = p.ID
					res = append(res, r)
					continue
				}
				if coll.OwnerID != userID {
					r.Code = ErrForbiddenCollection.Status
					r.ErrorMessage = ErrForbiddenCollection.Message
					r.ID = p.ID
					res = append(res, r)
					continue
				}
			}
			if p.Slug == "" {
				p.Slug = p.ID
			}
			if canCollect {
				// User already owns this post, so just add it to the given
				// collection.
				query = "UPDATE posts SET collection_id = ?, slug = ? WHERE id = ? AND owner_id = ?"
				params = []interface{}{coll.ID, p.Slug, p.ID, userID}
				slugIdx = 1
			} else {
				query = "UPDATE posts SET owner_id = ?, collection_id = ?, slug = ? WHERE id = ? AND modify_token = ? AND owner_id IS NULL"
				params = []interface{}{userID, coll.ID, p.Slug, p.ID, p.Token}
				slugIdx = 2
			}
		} else {
			query = "UPDATE posts SET owner_id = ? WHERE id = ? AND modify_token = ? AND owner_id IS NULL"
			params = []interface{}{userID, p.ID, p.Token}
		}
		qRes, err = db.AttemptClaim(&p, query, params, slugIdx)
		if err != nil {
			r.Code = http.StatusInternalServerError
			r.ErrorMessage = "An unknown error occurred."
			r.ID = p.ID
			res = append(res, r)
			log.Error("claimPosts (post %s): %v", p.ID, err)
			continue
		}

		// Get full post information to return
		var fullPost *PublicPost
		if p.Token != "" {
			fullPost, err = db.GetEditablePost(p.ID, p.Token)
		} else {
			fullPost, err = db.GetPost(p.ID, 0)
		}
		if err != nil {
			if err, ok := err.(impart.HTTPError); ok {
				r.Code = err.Status
				r.ErrorMessage = err.Message
				r.ID = p.ID
				res = append(res, r)
				continue
			}
		}
		if fullPost.OwnerID.Int64 != userID {
			r.Code = http.StatusConflict
			r.ErrorMessage = "Post is already owned by someone else."
			r.ID = p.ID
			res = append(res, r)
			continue
		}

		// Post was successfully claimed
		r.Code = http.StatusOK
		r.Post = fullPost
		if coll != nil {
			r.Post.Collection = &CollectionObj{Collection: *coll}
		}

		rowsAffected, _ := qRes.RowsAffected()
		if rowsAffected == 0 {
			// This was already claimed, but return 200
			r.Code = http.StatusOK
		}
		res = append(res, r)
	}

	return &res, nil
}

func (db *datastore) UpdatePostPinState(pinned bool, postID string, collID, ownerID, pos int64) error {
	if pos <= 0 || pos > 20 {
		pos = db.GetLastPinnedPostPos(collID) + 1
		if pos == -1 {
			pos = 1
		}
	}
	var err error
	if pinned {
		_, err = db.Exec("UPDATE posts SET pinned_position = ? WHERE id = ?", pos, postID)
	} else {
		_, err = db.Exec("UPDATE posts SET pinned_position = NULL WHERE id = ?", postID)
	}
	if err != nil {
		log.Error("Unable to update pinned post: %v", err)
		return err
	}
	return nil
}

func (db *datastore) GetLastPinnedPostPos(collID int64) int64 {
	var lastPos sql.NullInt64
	err := db.QueryRow("SELECT MAX(pinned_position) FROM posts WHERE collection_id = ? AND pinned_position IS NOT NULL", collID).Scan(&lastPos)
	switch {
	case err == sql.ErrNoRows:
		return -1
	case err != nil:
		log.Error("Failed selecting from posts: %v", err)
		return -1
	}
	if !lastPos.Valid {
		return -1
	}
	return lastPos.Int64
}

func (db *datastore) GetPinnedPosts(coll *CollectionObj, includeFuture bool) (*[]PublicPost, error) {
	// FIXME: sqlite-backed instances don't include ellipsis on truncated titles
	timeCondition := ""
	if !includeFuture {
		timeCondition = "AND created <= " + db.now()
	}
	rows, err := db.Query("SELECT id, slug, title, "+db.clip("content", 80)+", pinned_position FROM posts WHERE collection_id = ? AND pinned_position IS NOT NULL "+timeCondition+" ORDER BY pinned_position ASC", coll.ID)
	if err != nil {
		log.Error("Failed selecting pinned posts: %v", err)
		return nil, impart.HTTPError{http.StatusInternalServerError, "Couldn't retrieve pinned posts."}
	}
	defer rows.Close()

	posts := []PublicPost{}
	for rows.Next() {
		p := &Post{}
		err = rows.Scan(&p.ID, &p.Slug, &p.Title, &p.Content, &p.PinnedPosition)
		if err != nil {
			log.Error("Failed scanning row: %v", err)
			break
		}
		p.extractData()
		p.augmentContent(&coll.Collection)

		pp := p.processPost()
		pp.Collection = coll
		posts = append(posts, pp)
	}
	return &posts, nil
}

func (db *datastore) GetCollections(u *User, hostName string) (*[]Collection, error) {
	rows, err := db.Query("SELECT id, alias, title, description, privacy, view_count FROM collections WHERE owner_id = ? ORDER BY id ASC", u.ID)
	if err != nil {
		log.Error("Failed selecting from collections: %v", err)
		return nil, impart.HTTPError{http.StatusInternalServerError, "Couldn't retrieve user collections."}
	}
	defer rows.Close()

	colls := []Collection{}
	for rows.Next() {
		c := Collection{}
		err = rows.Scan(&c.ID, &c.Alias, &c.Title, &c.Description, &c.Visibility, &c.Views)
		if err != nil {
			log.Error("Failed scanning row: %v", err)
			break
		}
		c.hostName = hostName
		c.URL = c.CanonicalURL()
		c.Public = c.IsPublic()

		/*
			// NOTE: future functionality
			if visibility != nil { // TODO: && visibility == CollPublic {
				// Add Monetization info when retrieving all public collections
				c.Monetization = db.GetCollectionAttribute(c.ID, "monetization_pointer")
			}
		*/

		colls = append(colls, c)
	}
	err = rows.Err()
	if err != nil {
		log.Error("Error after Next() on rows: %v", err)
	}

	return &colls, nil
}

func (db *datastore) GetPublishableCollections(u *User, hostName string) (*[]Collection, error) {
	c, err := db.GetCollections(u, hostName)
	if err != nil {
		return nil, err
	}

	if len(*c) == 0 {
		return nil, impart.HTTPError{http.StatusInternalServerError, "You don't seem to have any blogs; they might've moved to another account. Try logging out and logging into your other account."}
	}
	return c, nil
}

func (db *datastore) GetPublicCollections(hostName string) (*[]Collection, error) {
	rows, err := db.Query(`SELECT c.id, alias, title, description, privacy, view_count
	FROM collections c
	LEFT JOIN users u ON u.id = c.owner_id
	WHERE c.privacy = 1 AND u.status = 0
	ORDER BY title ASC`)
	if err != nil {
		log.Error("Failed selecting public collections: %v", err)
		return nil, impart.HTTPError{http.StatusInternalServerError, "Couldn't retrieve public collections."}
	}
	defer rows.Close()

	colls := []Collection{}
	for rows.Next() {
		c := Collection{}
		err = rows.Scan(&c.ID, &c.Alias, &c.Title, &c.Description, &c.Visibility, &c.Views)
		if err != nil {
			log.Error("Failed scanning row: %v", err)
			break
		}
		c.hostName = hostName
		c.URL = c.CanonicalURL()
		c.Public = c.IsPublic()

		// Add Monetization information
		c.Monetization = db.GetCollectionAttribute(c.ID, "monetization_pointer")

		colls = append(colls, c)
	}
	err = rows.Err()
	if err != nil {
		log.Error("Error after Next() on rows: %v", err)
	}

	return &colls, nil
}

func (db *datastore) GetMeStats(u *User) userMeStats {
	s := userMeStats{}

	// User counts
	colls, _ := db.GetUserCollectionCount(u.ID)
	s.TotalCollections = colls

	var articles, collPosts uint64
	err := db.QueryRow("SELECT COUNT(*) FROM posts WHERE owner_id = ? AND collection_id IS NULL", u.ID).Scan(&articles)
	if err != nil && err != sql.ErrNoRows {
		log.Error("Couldn't get articles count for user %d: %v", u.ID, err)
	}
	s.TotalArticles = articles

	err = db.QueryRow("SELECT COUNT(*) FROM posts WHERE owner_id = ? AND collection_id IS NOT NULL", u.ID).Scan(&collPosts)
	if err != nil && err != sql.ErrNoRows {
		log.Error("Couldn't get coll posts count for user %d: %v", u.ID, err)
	}
	s.CollectionPosts = collPosts

	return s
}

func (db *datastore) GetTotalCollections() (collCount int64, err error) {
	err = db.QueryRow(`
	SELECT COUNT(*) 
	FROM collections c
	LEFT JOIN users u ON u.id = c.owner_id
	WHERE u.status = 0`).Scan(&collCount)
	if err != nil {
		log.Error("Unable to fetch collections count: %v", err)
	}
	return
}

func (db *datastore) GetTotalPosts() (postCount int64, err error) {
	err = db.QueryRow(`
	SELECT COUNT(*)
	FROM posts p
	LEFT JOIN users u ON u.id = p.owner_id
	WHERE u.status = 0`).Scan(&postCount)
	if err != nil {
		log.Error("Unable to fetch posts count: %v", err)
	}
	return
}

func (db *datastore) GetTopPosts(u *User, alias string, hostName string) (*[]PublicPost, error) {
	params := []interface{}{u.ID}
	where := ""
	if alias != "" {
		where = " AND alias = ?"
		params = append(params, alias)
	}
	rows, err := db.Query("SELECT p.id, p.slug, p.view_count, p.title, p.content, c.alias, c.title, c.description, c.view_count FROM posts p LEFT JOIN collections c ON p.collection_id = c.id WHERE p.owner_id = ?"+where+" ORDER BY p.view_count DESC, created DESC LIMIT 25", params...)
	if err != nil {
		log.Error("Failed selecting from posts: %v", err)
		return nil, impart.HTTPError{http.StatusInternalServerError, "Couldn't retrieve user top posts."}
	}
	defer rows.Close()

	posts := []PublicPost{}
	var gotErr bool
	for rows.Next() {
		p := Post{}
		c := Collection{}
		var alias, title, description sql.NullString
		var views sql.NullInt64
		err = rows.Scan(&p.ID, &p.Slug, &p.ViewCount, &p.Title, &p.Content, &alias, &title, &description, &views)
		if err != nil {
			log.Error("Failed scanning User.getPosts() row: %v", err)
			gotErr = true
			break
		}
		p.extractData()
		pubPost := p.processPost()

		if alias.Valid && alias.String != "" {
			c.Alias = alias.String
			c.Title = title.String
			c.Description = description.String
			c.Views = views.Int64
			c.hostName = hostName
			pubPost.Collection = &CollectionObj{Collection: c}
		}

		posts = append(posts, pubPost)
	}
	err = rows.Err()
	if err != nil {
		log.Error("Error after Next() on rows: %v", err)
	}

	if gotErr && len(posts) == 0 {
		// There were a lot of errors
		return nil, impart.HTTPError{http.StatusInternalServerError, "Unable to get data."}
	}

	return &posts, nil
}

func (db *datastore) GetAnonymousPosts(u *User, page int) (*[]PublicPost, error) {
	pagePosts := 10
	start := page*pagePosts - pagePosts
	if page == 0 {
		start = 0
		pagePosts = 1000
	}

	limitStr := ""
	if page > 0 {
		limitStr = fmt.Sprintf(" LIMIT %d, %d", start, pagePosts)
	}
	rows, err := db.Query("SELECT id, view_count, title, language, created, updated, content FROM posts WHERE owner_id = ? AND collection_id IS NULL ORDER BY created DESC"+limitStr, u.ID)
	if err != nil {
		log.Error("Failed selecting from posts: %v", err)
		return nil, impart.HTTPError{http.StatusInternalServerError, "Couldn't retrieve user anonymous posts."}
	}
	defer rows.Close()

	posts := []PublicPost{}
	for rows.Next() {
		p := Post{}
		err = rows.Scan(&p.ID, &p.ViewCount, &p.Title, &p.Language, &p.Created, &p.Updated, &p.Content)
		if err != nil {
			log.Error("Failed scanning row: %v", err)
			break
		}
		p.extractData()

		posts = append(posts, p.processPost())
	}
	err = rows.Err()
	if err != nil {
		log.Error("Error after Next() on rows: %v", err)
	}

	return &posts, nil
}

func (db *datastore) GetUserPosts(u *User) (*[]PublicPost, error) {
	rows, err := db.Query("SELECT p.id, p.slug, p.view_count, p.title, p.created, p.updated, p.content, p.text_appearance, p.language, p.rtl, c.alias, c.title, c.description, c.view_count FROM posts p LEFT JOIN collections c ON collection_id = c.id WHERE p.owner_id = ? ORDER BY created ASC", u.ID)
	if err != nil {
		log.Error("Failed selecting from posts: %v", err)
		return nil, impart.HTTPError{http.StatusInternalServerError, "Couldn't retrieve user posts."}
	}
	defer rows.Close()

	posts := []PublicPost{}
	var gotErr bool
	for rows.Next() {
		p := Post{}
		c := Collection{}
		var alias, title, description sql.NullString
		var views sql.NullInt64
		err = rows.Scan(&p.ID, &p.Slug, &p.ViewCount, &p.Title, &p.Created, &p.Updated, &p.Content, &p.Font, &p.Language, &p.RTL, &alias, &title, &description, &views)
		if err != nil {
			log.Error("Failed scanning User.getPosts() row: %v", err)
			gotErr = true
			break
		}
		p.extractData()
		pubPost := p.processPost()

		if alias.Valid && alias.String != "" {
			c.Alias = alias.String
			c.Title = title.String
			c.Description = description.String
			c.Views = views.Int64
			pubPost.Collection = &CollectionObj{Collection: c}
		}

		posts = append(posts, pubPost)
	}
	err = rows.Err()
	if err != nil {
		log.Error("Error after Next() on rows: %v", err)
	}

	if gotErr && len(posts) == 0 {
		// There were a lot of errors
		return nil, impart.HTTPError{http.StatusInternalServerError, "Unable to get data."}
	}

	return &posts, nil
}

func (db *datastore) GetUserPostsCount(userID int64) int64 {
	var count int64
	err := db.QueryRow("SELECT COUNT(*) FROM posts WHERE owner_id = ?", userID).Scan(&count)
	switch {
	case err == sql.ErrNoRows:
		return 0
	case err != nil:
		log.Error("Failed selecting posts count for user %d: %v", userID, err)
		return 0
	}

	return count
}

// ChangeSettings takes a User and applies the changes in the given
// userSettings, MODIFYING THE USER with successful changes.
func (db *datastore) ChangeSettings(app *App, u *User, s *userSettings) error {
	var errPass error
	q := query.NewUpdate()

	// Update email if given
	if s.Email != "" {
		encEmail, err := data.Encrypt(app.keys.EmailKey, s.Email)
		if err != nil {
			log.Error("Couldn't encrypt email %s: %s\n", s.Email, err)
			return impart.HTTPError{http.StatusInternalServerError, "Unable to encrypt email address."}
		}
		q.SetBytes(encEmail, "email")

		// Update the email if something goes awry updating the password
		defer func() {
			if errPass != nil {
				db.UpdateEncryptedUserEmail(u.ID, encEmail)
			}
		}()
		u.Email = zero.StringFrom(s.Email)
	}

	// Update username if given
	var newUsername string
	if s.Username != "" {
		var ie *impart.HTTPError
		newUsername, ie = getValidUsername(app, s.Username, u.Username)
		if ie != nil {
			// Username is invalid
			return *ie
		}
		if !author.IsValidUsername(app.cfg, newUsername) {
			// Ensure the username is syntactically correct.
			return impart.HTTPError{http.StatusPreconditionFailed, "Username isn't valid."}
		}

		t, err := db.Begin()
		if err != nil {
			log.Error("Couldn't start username change transaction: %v", err)
			return err
		}

		_, err = t.Exec("UPDATE users SET username = ? WHERE id = ?", newUsername, u.ID)
		if err != nil {
			t.Rollback()
			if db.isDuplicateKeyErr(err) {
				return impart.HTTPError{http.StatusConflict, "Username is already taken."}
			}
			log.Error("Unable to update users table: %v", err)
			return ErrInternalGeneral
		}

		_, err = t.Exec("UPDATE collections SET alias = ? WHERE alias = ? AND owner_id = ?", newUsername, u.Username, u.ID)
		if err != nil {
			t.Rollback()
			if db.isDuplicateKeyErr(err) {
				return impart.HTTPError{http.StatusConflict, "Username is already taken."}
			}
			log.Error("Unable to update collection: %v", err)
			return ErrInternalGeneral
		}

		// Keep track of name changes for redirection
		db.RemoveCollectionRedirect(t, newUsername)
		_, err = t.Exec("UPDATE collectionredirects SET new_alias = ? WHERE new_alias = ?", newUsername, u.Username)
		if err != nil {
			log.Error("Unable to update collectionredirects: %v", err)
		}
		_, err = t.Exec("INSERT INTO collectionredirects (prev_alias, new_alias) VALUES (?, ?)", u.Username, newUsername)
		if err != nil {
			log.Error("Unable to add new collectionredirect: %v", err)
		}

		err = t.Commit()
		if err != nil {
			t.Rollback()
			log.Error("Rolling back after Commit(): %v\n", err)
			return err
		}

		u.Username = newUsername
	}

	// Update passphrase if given
	if s.NewPass != "" {
		// Check if user has already set a password
		var err error
		u.HasPass, err = db.IsUserPassSet(u.ID)
		if err != nil {
			errPass = impart.HTTPError{http.StatusInternalServerError, "Unable to retrieve user data."}
			return errPass
		}

		if u.HasPass {
			// Check if currently-set password is correct
			hashedPass := u.HashedPass
			if len(hashedPass) == 0 {
				authUser, err := db.GetUserForAuthByID(u.ID)
				if err != nil {
					errPass = err
					return errPass
				}
				hashedPass = authUser.HashedPass
			}
			if !auth.Authenticated(hashedPass, []byte(s.OldPass)) {
				errPass = impart.HTTPError{http.StatusUnauthorized, "Incorrect password."}
				return errPass
			}
		}
		hashedPass, err := auth.HashPass([]byte(s.NewPass))
		if err != nil {
			errPass = impart.HTTPError{http.StatusInternalServerError, "Could not create password hash."}
			return errPass
		}
		q.SetBytes(hashedPass, "password")
	}

	// WHERE values
	q.Append(u.ID)

	if q.Updates == "" {
		if s.Username == "" {
			return ErrPostNoUpdatableVals
		}

		// Nothing to update except username. That was successful, so return now.
		return nil
	}

	res, err := db.Exec("UPDATE users SET "+q.Updates+" WHERE id = ?", q.Params...)
	if err != nil {
		log.Error("Unable to update collection: %v", err)
		return err
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		// Show the correct error message if nothing was updated
		var dummy int
		err := db.QueryRow("SELECT 1 FROM users WHERE id = ?", u.ID).Scan(&dummy)
		switch {
		case err == sql.ErrNoRows:
			return ErrUnauthorizedGeneral
		case err != nil:
			log.Error("Failed selecting from users: %v", err)
		}
		return nil
	}

	if s.NewPass != "" && !u.HasPass {
		u.HasPass = true
	}

	return nil
}

func (db *datastore) ChangePassphrase(userID int64, sudo bool, curPass string, hashedPass []byte) error {
	var dbPass []byte
	err := db.QueryRow("SELECT password FROM users WHERE id = ?", userID).Scan(&dbPass)
	switch {
	case err == sql.ErrNoRows:
		return ErrUserNotFound
	case err != nil:
		log.Error("Couldn't SELECT user password for change: %v", err)
		return err
	}

	if !sudo && !auth.Authenticated(dbPass, []byte(curPass)) {
		return impart.HTTPError{http.StatusUnauthorized, "Incorrect password."}
	}

	_, err = db.Exec("UPDATE users SET password = ? WHERE id = ?", hashedPass, userID)
	if err != nil {
		log.Error("Could not update passphrase: %v", err)
		return err
	}

	return nil
}

func (db *datastore) RemoveCollectionRedirect(t *sql.Tx, alias string) error {
	_, err := t.Exec("DELETE FROM collectionredirects WHERE prev_alias = ?", alias)
	if err != nil {
		log.Error("Unable to delete from collectionredirects: %v", err)
		return err
	}
	return nil
}

func (db *datastore) GetCollectionRedirect(alias string) (new string) {
	row := db.QueryRow("SELECT new_alias FROM collectionredirects WHERE prev_alias = ?", alias)
	err := row.Scan(&new)
	if err != nil && err != sql.ErrNoRows && !db.isIgnorableError(err) {
		log.Error("Failed selecting from collectionredirects: %v", err)
	}
	return
}

func (db *datastore) DeleteCollection(alias string, userID int64) error {
	c := &Collection{Alias: alias}
	var username string

	row := db.QueryRow("SELECT username FROM users WHERE id = ?", userID)
	err := row.Scan(&username)
	if err != nil {
		return err
	}

	// Ensure user isn't deleting their main blog
	if alias == username {
		return impart.HTTPError{http.StatusForbidden, "You cannot currently delete your primary blog."}
	}

	row = db.QueryRow("SELECT id FROM collections WHERE alias = ? AND owner_id = ?", alias, userID)
	err = row.Scan(&c.ID)
	switch {
	case err == sql.ErrNoRows:
		return impart.HTTPError{http.StatusNotFound, "Collection doesn't exist or you're not allowed to delete it."}
	case err != nil:
		log.Error("Failed selecting from collections: %v", err)
		return ErrInternalGeneral
	}

	t, err := db.Begin()
	if err != nil {
		return err
	}

	// Float all collection's posts
	_, err = t.Exec("UPDATE posts SET collection_id = NULL WHERE collection_id = ? AND owner_id = ?", c.ID, userID)
	if err != nil {
		t.Rollback()
		return err
	}

	// Remove redirects to or from this collection
	_, err = t.Exec("DELETE FROM collectionredirects WHERE prev_alias = ? OR new_alias = ?", alias, alias)
	if err != nil {
		t.Rollback()
		return err
	}

	// Remove any optional collection password
	_, err = t.Exec("DELETE FROM collectionpasswords WHERE collection_id = ?", c.ID)
	if err != nil {
		t.Rollback()
		return err
	}

	// Finally, delete collection itself
	_, err = t.Exec("DELETE FROM collections WHERE id = ?", c.ID)
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

func (db *datastore) IsCollectionAttributeOn(id int64, attr string) bool {
	var v string
	err := db.QueryRow("SELECT value FROM collectionattributes WHERE collection_id = ? AND attribute = ?", id, attr).Scan(&v)
	switch {
	case err == sql.ErrNoRows:
		return false
	case err != nil:
		log.Error("Couldn't SELECT value in isCollectionAttributeOn for attribute '%s': %v", attr, err)
		return false
	}
	return v == "1"
}

func (db *datastore) CollectionHasAttribute(id int64, attr string) bool {
	var dummy string
	err := db.QueryRow("SELECT value FROM collectionattributes WHERE collection_id = ? AND attribute = ?", id, attr).Scan(&dummy)
	switch {
	case err == sql.ErrNoRows:
		return false
	case err != nil:
		log.Error("Couldn't SELECT value in collectionHasAttribute for attribute '%s': %v", attr, err)
		return false
	}
	return true
}

func (db *datastore) GetCollectionAttribute(id int64, attr string) string {
	var v string
	err := db.QueryRow("SELECT value FROM collectionattributes WHERE collection_id = ? AND attribute = ?", id, attr).Scan(&v)
	switch {
	case err == sql.ErrNoRows:
		return ""
	case err != nil:
		log.Error("Couldn't SELECT value in getCollectionAttribute for attribute '%s': %v", attr, err)
		return ""
	}
	return v
}

func (db *datastore) SetCollectionAttribute(id int64, attr, v string) error {
	_, err := db.Exec("INSERT INTO collectionattributes (collection_id, attribute, value) VALUES (?, ?, ?) "+db.upsert("collection_id", "attribute")+" value = ?", id, attr, v, v)
	if err != nil {
		log.Error("Unable to INSERT into collectionattributes: %v", err)
		return err
	}
	return nil
}

// DeleteAccount will delete the entire account for userID
func (db *datastore) DeleteAccount(userID int64) error {
	// Get all collections
	rows, err := db.Query("SELECT id, alias FROM collections WHERE owner_id = ?", userID)
	if err != nil {
		log.Error("Unable to get collections: %v", err)
		return err
	}
	defer rows.Close()
	colls := []Collection{}
	var c Collection
	for rows.Next() {
		err = rows.Scan(&c.ID, &c.Alias)
		if err != nil {
			log.Error("Unable to scan collection cols: %v", err)
			return err
		}
		colls = append(colls, c)
	}

	// Start transaction
	t, err := db.Begin()
	if err != nil {
		log.Error("Unable to begin: %v", err)
		return err
	}

	// Clean up all collection related information
	var res sql.Result
	for _, c := range colls {
		// Delete tokens
		res, err = t.Exec("DELETE FROM collectionattributes WHERE collection_id = ?", c.ID)
		if err != nil {
			t.Rollback()
			log.Error("Unable to delete attributes on %s: %v", c.Alias, err)
			return err
		}
		rs, _ := res.RowsAffected()
		log.Info("Deleted %d for %s from collectionattributes", rs, c.Alias)

		// Remove any optional collection password
		res, err = t.Exec("DELETE FROM collectionpasswords WHERE collection_id = ?", c.ID)
		if err != nil {
			t.Rollback()
			log.Error("Unable to delete passwords on %s: %v", c.Alias, err)
			return err
		}
		rs, _ = res.RowsAffected()
		log.Info("Deleted %d for %s from collectionpasswords", rs, c.Alias)

		// Remove redirects to this collection
		res, err = t.Exec("DELETE FROM collectionredirects WHERE new_alias = ?", c.Alias)
		if err != nil {
			t.Rollback()
			log.Error("Unable to delete redirects on %s: %v", c.Alias, err)
			return err
		}
		rs, _ = res.RowsAffected()
		log.Info("Deleted %d for %s from collectionredirects", rs, c.Alias)

		// Remove any collection keys
		res, err = t.Exec("DELETE FROM collectionkeys WHERE collection_id = ?", c.ID)
		if err != nil {
			t.Rollback()
			log.Error("Unable to delete keys on %s: %v", c.Alias, err)
			return err
		}
		rs, _ = res.RowsAffected()
		log.Info("Deleted %d for %s from collectionkeys", rs, c.Alias)

		// TODO: federate delete collection

		// Remove remote follows
		res, err = t.Exec("DELETE FROM remotefollows WHERE collection_id = ?", c.ID)
		if err != nil {
			t.Rollback()
			log.Error("Unable to delete remote follows on %s: %v", c.Alias, err)
			return err
		}
		rs, _ = res.RowsAffected()
		log.Info("Deleted %d for %s from remotefollows", rs, c.Alias)
	}

	// Delete collections
	res, err = t.Exec("DELETE FROM collections WHERE owner_id = ?", userID)
	if err != nil {
		t.Rollback()
		log.Error("Unable to delete collections: %v", err)
		return err
	}
	rs, _ := res.RowsAffected()
	log.Info("Deleted %d from collections", rs)

	// Delete tokens
	res, err = t.Exec("DELETE FROM accesstokens WHERE user_id = ?", userID)
	if err != nil {
		t.Rollback()
		log.Error("Unable to delete access tokens: %v", err)
		return err
	}
	rs, _ = res.RowsAffected()
	log.Info("Deleted %d from accesstokens", rs)

	// Delete user attributes
	res, err = t.Exec("DELETE FROM oauth_users WHERE user_id = ?", userID)
	if err != nil {
		t.Rollback()
		log.Error("Unable to delete oauth_users: %v", err)
		return err
	}
	rs, _ = res.RowsAffected()
	log.Info("Deleted %d from oauth_users", rs)

	// Delete posts
	// TODO: should maybe get each row so we can federate a delete
	// if so needs to be outside of transaction like collections
	res, err = t.Exec("DELETE FROM posts WHERE owner_id = ?", userID)
	if err != nil {
		t.Rollback()
		log.Error("Unable to delete posts: %v", err)
		return err
	}
	rs, _ = res.RowsAffected()
	log.Info("Deleted %d from posts", rs)

	// Delete user attributes
	res, err = t.Exec("DELETE FROM userattributes WHERE user_id = ?", userID)
	if err != nil {
		t.Rollback()
		log.Error("Unable to delete attributes: %v", err)
		return err
	}
	rs, _ = res.RowsAffected()
	log.Info("Deleted %d from userattributes", rs)

	// Delete user invites
	res, err = t.Exec("DELETE FROM userinvites WHERE owner_id = ?", userID)
	if err != nil {
		t.Rollback()
		log.Error("Unable to delete invites: %v", err)
		return err
	}
	rs, _ = res.RowsAffected()
	log.Info("Deleted %d from userinvites", rs)

	// Delete the user
	res, err = t.Exec("DELETE FROM users WHERE id = ?", userID)
	if err != nil {
		t.Rollback()
		log.Error("Unable to delete user: %v", err)
		return err
	}
	rs, _ = res.RowsAffected()
	log.Info("Deleted %d from users", rs)

	// Commit all changes to the database
	err = t.Commit()
	if err != nil {
		t.Rollback()
		log.Error("Unable to commit: %v", err)
		return err
	}

	// TODO: federate delete actor

	return nil
}

func (db *datastore) GetAPActorKeys(collectionID int64) ([]byte, []byte) {
	var pub, priv []byte
	err := db.QueryRow("SELECT public_key, private_key FROM collectionkeys WHERE collection_id = ?", collectionID).Scan(&pub, &priv)
	switch {
	case err == sql.ErrNoRows:
		// Generate keys
		pub, priv = activitypub.GenerateKeys()
		_, err = db.Exec("INSERT INTO collectionkeys (collection_id, public_key, private_key) VALUES (?, ?, ?)", collectionID, pub, priv)
		if err != nil {
			log.Error("Unable to INSERT new activitypub keypair: %v", err)
			return nil, nil
		}
	case err != nil:
		log.Error("Couldn't SELECT collectionkeys: %v", err)
		return nil, nil
	}

	return pub, priv
}

func (db *datastore) CreateUserInvite(id string, userID int64, maxUses int, expires *time.Time) error {
	_, err := db.Exec("INSERT INTO userinvites (id, owner_id, max_uses, created, expires, inactive) VALUES (?, ?, ?, "+db.now()+", ?, 0)", id, userID, maxUses, expires)
	return err
}

func (db *datastore) GetUserInvites(userID int64) (*[]Invite, error) {
	rows, err := db.Query("SELECT id, max_uses, created, expires, inactive FROM userinvites WHERE owner_id = ? ORDER BY created DESC", userID)
	if err != nil {
		log.Error("Failed selecting from userinvites: %v", err)
		return nil, impart.HTTPError{http.StatusInternalServerError, "Couldn't retrieve user invites."}
	}
	defer rows.Close()

	is := []Invite{}
	for rows.Next() {
		i := Invite{}
		err = rows.Scan(&i.ID, &i.MaxUses, &i.Created, &i.Expires, &i.Inactive)
		is = append(is, i)
	}
	return &is, nil
}

func (db *datastore) GetUserInvite(id string) (*Invite, error) {
	var i Invite
	err := db.QueryRow("SELECT id, max_uses, created, expires, inactive FROM userinvites WHERE id = ?", id).Scan(&i.ID, &i.MaxUses, &i.Created, &i.Expires, &i.Inactive)
	switch {
	case err == sql.ErrNoRows, db.isIgnorableError(err):
		return nil, impart.HTTPError{http.StatusNotFound, "Invite doesn't exist."}
	case err != nil:
		log.Error("Failed selecting invite: %v", err)
		return nil, err
	}

	return &i, nil
}

// IsUsersInvite returns true if the user with ID created the invite with code
// and an error other than sql no rows, if any. Will return false in the event
// of an error.
func (db *datastore) IsUsersInvite(code string, userID int64) (bool, error) {
	var id string
	err := db.QueryRow("SELECT id FROM userinvites WHERE id = ? AND owner_id = ?", code, userID).Scan(&id)
	if err != nil && err != sql.ErrNoRows {
		log.Error("Failed selecting invite: %v", err)
		return false, err
	}
	return id != "", nil
}

func (db *datastore) GetUsersInvitedCount(id string) int64 {
	var count int64
	err := db.QueryRow("SELECT COUNT(*) FROM usersinvited WHERE invite_id = ?", id).Scan(&count)
	switch {
	case err == sql.ErrNoRows:
		return 0
	case err != nil:
		log.Error("Failed selecting users invited count: %v", err)
		return 0
	}

	return count
}

func (db *datastore) CreateInvitedUser(inviteID string, userID int64) error {
	_, err := db.Exec("INSERT INTO usersinvited (invite_id, user_id) VALUES (?, ?)", inviteID, userID)
	return err
}

func (db *datastore) GetInstancePages() ([]*instanceContent, error) {
	return db.GetAllDynamicContent("page")
}

func (db *datastore) GetAllDynamicContent(t string) ([]*instanceContent, error) {
	where := ""
	params := []interface{}{}
	if t != "" {
		where = " WHERE content_type = ?"
		params = append(params, t)
	}
	rows, err := db.Query("SELECT id, title, content, updated, content_type FROM appcontent"+where, params...)
	if err != nil {
		log.Error("Failed selecting from appcontent: %v", err)
		return nil, impart.HTTPError{http.StatusInternalServerError, "Couldn't retrieve instance pages."}
	}
	defer rows.Close()

	pages := []*instanceContent{}
	for rows.Next() {
		c := &instanceContent{}
		err = rows.Scan(&c.ID, &c.Title, &c.Content, &c.Updated, &c.Type)
		if err != nil {
			log.Error("Failed scanning row: %v", err)
			break
		}
		pages = append(pages, c)
	}
	err = rows.Err()
	if err != nil {
		log.Error("Error after Next() on rows: %v", err)
	}

	return pages, nil
}

func (db *datastore) GetDynamicContent(id string) (*instanceContent, error) {
	c := &instanceContent{
		ID: id,
	}
	err := db.QueryRow("SELECT title, content, updated, content_type FROM appcontent WHERE id = ?", id).Scan(&c.Title, &c.Content, &c.Updated, &c.Type)
	switch {
	case err == sql.ErrNoRows:
		return nil, nil
	case err != nil:
		log.Error("Couldn't SELECT FROM appcontent for id '%s': %v", id, err)
		return nil, err
	}
	return c, nil
}

func (db *datastore) UpdateDynamicContent(id, title, content, contentType string) error {
	var err error
	if db.driverName == driverSQLite {
		_, err = db.Exec("INSERT OR REPLACE INTO appcontent (id, title, content, updated, content_type) VALUES (?, ?, ?, "+db.now()+", ?)", id, title, content, contentType)
	} else {
		_, err = db.Exec("INSERT INTO appcontent (id, title, content, updated, content_type) VALUES (?, ?, ?, "+db.now()+", ?) "+db.upsert("id")+" title = ?, content = ?, updated = "+db.now(), id, title, content, contentType, title, content)
	}
	if err != nil {
		log.Error("Unable to INSERT appcontent for '%s': %v", id, err)
	}
	return err
}

func (db *datastore) GetAllUsers(page uint) (*[]User, error) {
	limitStr := fmt.Sprintf("0, %d", adminUsersPerPage)
	if page > 1 {
		limitStr = fmt.Sprintf("%d, %d", (page-1)*adminUsersPerPage, adminUsersPerPage)
	}

	rows, err := db.Query("SELECT id, username, created, status FROM users ORDER BY created DESC LIMIT " + limitStr)
	if err != nil {
		log.Error("Failed selecting from users: %v", err)
		return nil, impart.HTTPError{http.StatusInternalServerError, "Couldn't retrieve all users."}
	}
	defer rows.Close()

	users := []User{}
	for rows.Next() {
		u := User{}
		err = rows.Scan(&u.ID, &u.Username, &u.Created, &u.Status)
		if err != nil {
			log.Error("Failed scanning GetAllUsers() row: %v", err)
			break
		}
		users = append(users, u)
	}
	return &users, nil
}

func (db *datastore) GetAllUsersCount() int64 {
	var count int64
	err := db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	switch {
	case err == sql.ErrNoRows:
		return 0
	case err != nil:
		log.Error("Failed selecting all users count: %v", err)
		return 0
	}

	return count
}

func (db *datastore) GetUserLastPostTime(id int64) (*time.Time, error) {
	var t time.Time
	err := db.QueryRow("SELECT created FROM posts WHERE owner_id = ? ORDER BY created DESC LIMIT 1", id).Scan(&t)
	switch {
	case err == sql.ErrNoRows:
		return nil, nil
	case err != nil:
		log.Error("Failed selecting last post time from posts: %v", err)
		return nil, err
	}
	return &t, nil
}

// SetUserStatus changes a user's status in the database. see Users.UserStatus
func (db *datastore) SetUserStatus(id int64, status UserStatus) error {
	_, err := db.Exec("UPDATE users SET status = ? WHERE id = ?", status, id)
	if err != nil {
		return fmt.Errorf("failed to update user status: %v", err)
	}
	return nil
}

func (db *datastore) GetCollectionLastPostTime(id int64) (*time.Time, error) {
	var t time.Time
	err := db.QueryRow("SELECT created FROM posts WHERE collection_id = ? ORDER BY created DESC LIMIT 1", id).Scan(&t)
	switch {
	case err == sql.ErrNoRows:
		return nil, nil
	case err != nil:
		log.Error("Failed selecting last post time from posts: %v", err)
		return nil, err
	}
	return &t, nil
}

func (db *datastore) GenerateOAuthState(ctx context.Context, provider string, clientID string, attachUser int64, inviteCode string) (string, error) {
	state := id.Generate62RandomString(24)
	attachUserVal := sql.NullInt64{Valid: attachUser > 0, Int64: attachUser}
	inviteCodeVal := sql.NullString{Valid: inviteCode != "", String: inviteCode}
	_, err := db.ExecContext(ctx, "INSERT INTO oauth_client_states (state, provider, client_id, used, created_at, attach_user_id, invite_code) VALUES (?, ?, ?, FALSE, "+db.now()+", ?, ?)", state, provider, clientID, attachUserVal, inviteCodeVal)
	if err != nil {
		return "", fmt.Errorf("unable to record oauth client state: %w", err)
	}
	return state, nil
}

func (db *datastore) ValidateOAuthState(ctx context.Context, state string) (string, string, int64, string, error) {
	var provider string
	var clientID string
	var attachUserID sql.NullInt64
	var inviteCode sql.NullString
	err := wf_db.RunTransactionWithOptions(ctx, db.DB, &sql.TxOptions{}, func(ctx context.Context, tx *sql.Tx) error {
		err := tx.
			QueryRowContext(ctx, "SELECT provider, client_id, attach_user_id, invite_code FROM oauth_client_states WHERE state = ? AND used = FALSE", state).
			Scan(&provider, &clientID, &attachUserID, &inviteCode)
		if err != nil {
			return err
		}

		res, err := tx.ExecContext(ctx, "UPDATE oauth_client_states SET used = TRUE WHERE state = ?", state)
		if err != nil {
			return err
		}
		rowsAffected, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if rowsAffected != 1 {
			return fmt.Errorf("state not found")
		}
		return nil
	})
	if err != nil {
		return "", "", 0, "", nil
	}
	return provider, clientID, attachUserID.Int64, inviteCode.String, nil
}

func (db *datastore) RecordRemoteUserID(ctx context.Context, localUserID int64, remoteUserID, provider, clientID, accessToken string) error {
	var err error
	if db.driverName == driverSQLite {
		_, err = db.ExecContext(ctx, "INSERT OR REPLACE INTO oauth_users (user_id, remote_user_id, provider, client_id, access_token) VALUES (?, ?, ?, ?, ?)", localUserID, remoteUserID, provider, clientID, accessToken)
	} else {
		_, err = db.ExecContext(ctx, "INSERT INTO oauth_users (user_id, remote_user_id, provider, client_id, access_token) VALUES (?, ?, ?, ?, ?) "+db.upsert("user")+" access_token = ?", localUserID, remoteUserID, provider, clientID, accessToken, accessToken)
	}
	if err != nil {
		log.Error("Unable to INSERT oauth_users for '%d': %v", localUserID, err)
	}
	return err
}

// GetIDForRemoteUser returns a user ID associated with a remote user ID.
func (db *datastore) GetIDForRemoteUser(ctx context.Context, remoteUserID, provider, clientID string) (int64, error) {
	var userID int64 = -1
	err := db.
		QueryRowContext(ctx, "SELECT user_id FROM oauth_users WHERE remote_user_id = ? AND provider = ? AND client_id = ?", remoteUserID, provider, clientID).
		Scan(&userID)
	// Not finding a record is OK.
	if err != nil && err != sql.ErrNoRows {
		return -1, err
	}
	return userID, nil
}

type oauthAccountInfo struct {
	Provider        string
	ClientID        string
	RemoteUserID    string
	DisplayName     string
	AllowDisconnect bool
}

func (db *datastore) GetOauthAccounts(ctx context.Context, userID int64) ([]oauthAccountInfo, error) {
	rows, err := db.QueryContext(ctx, "SELECT provider, client_id, remote_user_id FROM oauth_users WHERE user_id = ? ", userID)
	if err != nil {
		log.Error("Failed selecting from oauth_users: %v", err)
		return nil, impart.HTTPError{http.StatusInternalServerError, "Couldn't retrieve user oauth accounts."}
	}
	defer rows.Close()

	var records []oauthAccountInfo
	for rows.Next() {
		info := oauthAccountInfo{}
		err = rows.Scan(&info.Provider, &info.ClientID, &info.RemoteUserID)
		if err != nil {
			log.Error("Failed scanning GetAllUsers() row: %v", err)
			break
		}
		records = append(records, info)
	}
	return records, nil
}

// DatabaseInitialized returns whether or not the current datastore has been
// initialized with the correct schema.
// Currently, it checks to see if the `users` table exists.
func (db *datastore) DatabaseInitialized() bool {
	var dummy string
	var err error
	if db.driverName == driverSQLite {
		err = db.QueryRow("SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'users'").Scan(&dummy)
	} else {
		err = db.QueryRow("SHOW TABLES LIKE 'users'").Scan(&dummy)
	}
	switch {
	case err == sql.ErrNoRows:
		return false
	case err != nil:
		log.Error("Couldn't SHOW TABLES: %v", err)
		return false
	}

	return true
}

func (db *datastore) RemoveOauth(ctx context.Context, userID int64, provider string, clientID string, remoteUserID string) error {
	_, err := db.ExecContext(ctx, `DELETE FROM oauth_users WHERE user_id = ? AND provider = ? AND client_id = ? AND remote_user_id = ?`, userID, provider, clientID, remoteUserID)
	return err
}

func stringLogln(log *string, s string, v ...interface{}) {
	*log += fmt.Sprintf(s+"\n", v...)
}

func handleFailedPostInsert(err error) error {
	log.Error("Couldn't insert into posts: %v", err)
	return err
}

// Deprecated: use GetProfileURLFromHandle() instead, which returns user-facing URL instead of actor_id
func (db *datastore) GetProfilePageFromHandle(app *App, handle string) (string, error) {
	handle = strings.TrimLeft(handle, "@")
	actorIRI := ""
	parts := strings.Split(handle, "@")
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid handle format")
	}
	domain := parts[1]

	// Check non-AP instances
	if siloProfileURL := silobridge.Profile(parts[0], domain); siloProfileURL != "" {
		return siloProfileURL, nil
	}

	remoteUser, err := getRemoteUserFromHandle(app, handle)
	if err != nil {
		// can't find using handle in the table but the table may already have this user without
		// handle from a previous version
		// TODO: Make this determination. We should know whether a user exists without a handle, or doesn't exist at all
		actorIRI = RemoteLookup(handle)
		_, errRemoteUser := getRemoteUser(app, actorIRI)
		// if it exists then we need to update the handle
		if errRemoteUser == nil {
			_, err := app.db.Exec("UPDATE remoteusers SET handle = ? WHERE actor_id = ?", handle, actorIRI)
			if err != nil {
				log.Error("Couldn't update handle '%s' for user %s", handle, actorIRI)
			}
		} else {
			// this probably means we don't have the user in the table so let's try to insert it
			// here we need to ask the server for the inboxes
			remoteActor, err := activityserve.NewRemoteActor(actorIRI)
			if err != nil {
				log.Error("Couldn't fetch remote actor: %v", err)
			}
			if debugging {
				log.Info("%s %s %s %s", actorIRI, remoteActor.GetInbox(), remoteActor.GetSharedInbox(), handle)
			}
			_, err = app.db.Exec("INSERT INTO remoteusers (actor_id, inbox, shared_inbox, handle) VALUES(?, ?, ?, ?)", actorIRI, remoteActor.GetInbox(), remoteActor.GetSharedInbox(), handle)
			if err != nil {
				log.Error("Couldn't insert remote user: %v", err)
				return "", err
			}
		}
	} else {
		actorIRI = remoteUser.ActorID
	}
	return actorIRI, nil
}

func (db *datastore) AddEmailSubscription(collID, userID int64, email string, confirmed bool) (*EmailSubscriber, error) {
	friendlyChars := "0123456789BCDFGHJKLMNPQRSTVWXYZbcdfghjklmnpqrstvwxyz"
	subID := id.GenerateRandomString(friendlyChars, 8)
	token := id.GenerateRandomString(friendlyChars, 16)
	emailVal := sql.NullString{
		String: email,
		Valid:  email != "",
	}
	userIDVal := sql.NullInt64{
		Int64: userID,
		Valid: userID > 0,
	}

	_, err := db.Exec("INSERT INTO emailsubscribers (id, collection_id, user_id, email, subscribed, token, confirmed) VALUES (?, ?, ?, ?, "+db.now()+", ?, ?)", subID, collID, userIDVal, emailVal, token, confirmed)
	if err != nil {
		if mysqlErr, ok := err.(*mysql.MySQLError); ok {
			if mysqlErr.Number == mySQLErrDuplicateKey {
				// Duplicate, so just return existing subscriber information
				log.Info("Duplicate subscriber for email %s, user %d; returning existing subscriber", email, userID)
				return db.FetchEmailSubscriber(email, userID, collID)
			}
		}
		return nil, err
	}

	return &EmailSubscriber{
		ID:     subID,
		CollID: collID,
		UserID: userIDVal,
		Email:  emailVal,
		Token:  token,
	}, nil
}

func (db *datastore) IsEmailSubscriber(email string, userID, collID int64) bool {
	var dummy int
	var err error
	if email != "" {
		err = db.QueryRow("SELECT 1 FROM emailsubscribers WHERE email = ? AND collection_id = ?", email, collID).Scan(&dummy)
	} else {
		err = db.QueryRow("SELECT 1 FROM emailsubscribers WHERE user_id = ? AND collection_id = ?", userID, collID).Scan(&dummy)
	}
	switch {
	case err == sql.ErrNoRows:
		return false
	case err != nil:
		return false
	}
	return true
}

func (db *datastore) GetEmailSubscribers(collID int64, reqConfirmed bool) ([]*EmailSubscriber, error) {
	cond := ""
	if reqConfirmed {
		cond = " AND confirmed = 1"
	}
	rows, err := db.Query(`SELECT s.id, collection_id, user_id, s.email, u.email, subscribed, token, confirmed, allow_export 
FROM emailsubscribers s 
LEFT JOIN users u 
  ON u.id = user_id 
WHERE collection_id = ?`+cond+`
ORDER BY subscribed DESC`, collID)
	if err != nil {
		log.Error("Failed selecting email subscribers for collection %d: %v", collID, err)
		return nil, err
	}
	defer rows.Close()

	var subs []*EmailSubscriber
	for rows.Next() {
		s := &EmailSubscriber{}
		err = rows.Scan(&s.ID, &s.CollID, &s.UserID, &s.Email, &s.acctEmail, &s.Subscribed, &s.Token, &s.Confirmed, &s.AllowExport)
		if err != nil {
			log.Error("Failed scanning row from email subscribers: %v", err)
			continue
		}
		subs = append(subs, s)
	}
	return subs, nil
}

func (db *datastore) FetchEmailSubscriberEmail(subID, token string) (string, error) {
	var email sql.NullString
	// TODO: return user email if there's a user_id ?
	err := db.QueryRow("SELECT email FROM emailsubscribers WHERE id = ? AND token = ?", subID, token).Scan(&email)
	switch {
	case err == sql.ErrNoRows:
		return "", fmt.Errorf("Subscriber doesn't exist or token is invalid.")
	case err != nil:
		log.Error("Couldn't SELECT email from emailsubscribers: %v", err)
		return "", fmt.Errorf("Something went very wrong.")
	}

	return email.String, nil
}

func (db *datastore) FetchEmailSubscriber(email string, userID, collID int64) (*EmailSubscriber, error) {
	const emailSubCols = "id, collection_id, user_id, email, subscribed, token, confirmed, allow_export"

	s := &EmailSubscriber{}
	var row *sql.Row
	if email != "" {
		row = db.QueryRow("SELECT "+emailSubCols+" FROM emailsubscribers WHERE email = ? AND collection_id = ?", email, collID)
	} else {
		row = db.QueryRow("SELECT "+emailSubCols+" FROM emailsubscribers WHERE user_id = ? AND collection_id = ?", userID, collID)
	}
	err := row.Scan(&s.ID, &s.CollID, &s.UserID, &s.Email, &s.Subscribed, &s.Token, &s.Confirmed, &s.AllowExport)
	switch {
	case err == sql.ErrNoRows:
		return nil, nil
	case err != nil:
		return nil, err
	}
	return s, nil
}

func (db *datastore) DeleteEmailSubscriber(subID, token string) error {
	res, err := db.Exec("DELETE FROM emailsubscribers WHERE id = ? AND token = ?", subID, token)
	if err != nil {
		return err
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return impart.HTTPError{http.StatusNotFound, "Invalid token, or subscriber doesn't exist"}
	}
	return nil
}

func (db *datastore) DeleteEmailSubscriberByUser(email string, userID, collID int64) error {
	var res sql.Result
	var err error
	if email != "" {
		res, err = db.Exec("DELETE FROM emailsubscribers WHERE email = ? AND collection_id = ?", email, collID)
	} else {
		res, err = db.Exec("DELETE FROM emailsubscribers WHERE user_id = ? AND collection_id = ?", userID, collID)
	}
	if err != nil {
		return err
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return impart.HTTPError{http.StatusNotFound, "Subscriber doesn't exist"}
	}
	return nil
}

func (db *datastore) UpdateSubscriberConfirmed(subID, token string) error {
	email, err := db.FetchEmailSubscriberEmail(subID, token)
	if err != nil {
		log.Error("Didn't fetch email subscriber: %v", err)
		return err
	}

	// TODO: ensure all addresses with original name are also confirmed, e.g. matt+fake@write.as and matt@write.as are now confirmed
	_, err = db.Exec("UPDATE emailsubscribers SET confirmed = 1 WHERE email = ?", email)
	if err != nil {
		log.Error("Could not update email subscriber confirmation status: %v", err)
		return err
	}
	return nil
}

func (db *datastore) IsSubscriberConfirmed(email string) bool {
	var dummy int64
	err := db.QueryRow("SELECT 1 FROM emailsubscribers WHERE email = ? AND confirmed = 1", email).Scan(&dummy)
	switch {
	case err == sql.ErrNoRows:
		return false
	case err != nil:
		log.Error("Couldn't SELECT in isSubscriberConfirmed: %v", err)
		return false
	}

	return true
}

func (db *datastore) InsertJob(j *PostJob) error {
	res, err := db.Exec("INSERT INTO publishjobs (post_id, action, delay) VALUES (?, ?, ?)", j.PostID, j.Action, j.Delay)
	if err != nil {
		return err
	}
	jobID, err := res.LastInsertId()
	if err != nil {
		log.Error("[jobs] Couldn't get last insert ID! %s", err)
	}
	log.Info("[jobs] Queued %s job #%d for post %s, delayed %d minutes", j.Action, jobID, j.PostID, j.Delay)
	return nil
}

func (db *datastore) UpdateJobForPost(postID string, delay int64) error {
	_, err := db.Exec("UPDATE publishjobs SET delay = ? WHERE post_id = ?", delay, postID)
	if err != nil {
		return fmt.Errorf("Unable to update publish job: %s", err)
	}
	log.Info("Updated job for post %s: delay %d", postID, delay)
	return nil
}

func (db *datastore) DeleteJob(id int64) error {
	_, err := db.Exec("DELETE FROM publishjobs WHERE id = ?", id)
	if err != nil {
		return err
	}
	log.Info("[job #%d] Deleted.", id)
	return nil
}

func (db *datastore) DeleteJobByPost(postID string) error {
	_, err := db.Exec("DELETE FROM publishjobs WHERE post_id = ?", postID)
	if err != nil {
		return err
	}
	log.Info("[job] Deleted job for post %s", postID)
	return nil
}

func (db *datastore) GetJobsToRun(action string) ([]*PostJob, error) {
	timeWhere := "created < DATE_SUB(NOW(), INTERVAL delay MINUTE) AND created > DATE_SUB(NOW(), INTERVAL delay + 5 MINUTE)"
	if db.driverName == driverSQLite {
		timeWhere = "created < DATETIME('now', '-' || delay || ' MINUTE') AND created > DATETIME('now', '-' || (delay+5) || ' MINUTE')"
	}
	rows, err := db.Query(`SELECT pj.id, post_id, action, delay
		FROM publishjobs pj
		INNER JOIN posts p
			ON post_id = p.id
		WHERE action = ? AND `+timeWhere+`
		ORDER BY created ASC`, action)
	if err != nil {
		log.Error("Failed selecting from publishjobs: %v", err)
		return nil, impart.HTTPError{http.StatusInternalServerError, "Couldn't retrieve publish jobs."}
	}
	defer rows.Close()

	jobs := []*PostJob{}
	for rows.Next() {
		j := &PostJob{}
		err = rows.Scan(&j.ID, &j.PostID, &j.Action, &j.Delay)
		jobs = append(jobs, j)
	}
	return jobs, nil
}
