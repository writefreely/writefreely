/*
 * Copyright © 2018 A Bunch Tell LLC.
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
	"net/http"
	"strings"
	"time"

	"github.com/guregu/null"
	"github.com/guregu/null/zero"
	uuid "github.com/nu7hatch/gouuid"
	"github.com/writeas/impart"
	"github.com/writeas/nerds/store"
	"github.com/writeas/web-core/activitypub"
	"github.com/writeas/web-core/auth"
	"github.com/writeas/web-core/data"
	"github.com/writeas/web-core/id"
	"github.com/writeas/web-core/log"
	"github.com/writeas/web-core/query"
	"github.com/writeas/writefreely/author"
)

const (
	mySQLErrDuplicateKey = 1062

	driverMySQL  = "mysql"
	driverSQLite = "sqlite3"
)

var (
	SQLiteEnabled bool
)

type writestore interface {
	CreateUser(*User, string) error
	UpdateUserEmail(keys *keychain, userID int64, email string) error
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
	DeleteAccount(userID int64) (l *string, err error)
	ChangeSettings(app *app, u *User, s *userSettings) error
	ChangePassphrase(userID int64, sudo bool, curPass string, hashedPass []byte) error

	GetCollections(u *User) (*[]Collection, error)
	GetPublishableCollections(u *User) (*[]Collection, error)
	GetMeStats(u *User) userMeStats
	GetTotalCollections() (int64, error)
	GetTotalPosts() (int64, error)
	GetTopPosts(u *User, alias string) (*[]PublicPost, error)
	GetAnonymousPosts(u *User) (*[]PublicPost, error)
	GetUserPosts(u *User) (*[]PublicPost, error)

	CreateOwnedPost(post *SubmittedPost, accessToken, collAlias string) (*PublicPost, error)
	CreatePost(userID, collID int64, post *SubmittedPost) (*Post, error)
	UpdateOwnedPost(post *AuthenticatedPost, userID int64) error
	GetEditablePost(id, editToken string) (*PublicPost, error)
	PostIDExists(id string) bool
	GetPost(id string, collectionID int64) (*PublicPost, error)
	GetOwnedPost(id string, ownerID int64) (*PublicPost, error)
	GetPostProperty(id string, collectionID int64, property string) (interface{}, error)

	CreateCollectionFromToken(string, string, string) (*Collection, error)
	CreateCollection(string, string, int64) (*Collection, error)
	GetCollectionBy(condition string, value interface{}) (*Collection, error)
	GetCollection(alias string) (*Collection, error)
	GetCollectionForPad(alias string) (*Collection, error)
	GetCollectionByID(id int64) (*Collection, error)
	UpdateCollection(c *SubmittedCollection, alias string) error
	DeleteCollection(alias string, userID int64) error

	UpdatePostPinState(pinned bool, postID string, collID, ownerID, pos int64) error
	GetLastPinnedPostPos(collID int64) int64
	GetPinnedPosts(coll *CollectionObj) (*[]PublicPost, error)
	RemoveCollectionRedirect(t *sql.Tx, alias string) error
	GetCollectionRedirect(alias string) (new string)
	IsCollectionAttributeOn(id int64, attr string) bool
	CollectionHasAttribute(id int64, attr string) bool

	CanCollect(cpr *ClaimPostRequest, userID int64) bool
	AttemptClaim(p *ClaimPostRequest, query string, params []interface{}, slugIdx int) (sql.Result, error)
	DispersePosts(userID int64, postIDs []string) (*[]ClaimPostResult, error)
	ClaimPosts(userID int64, collAlias string, posts *[]ClaimPostRequest) (*[]ClaimPostResult, error)

	GetPostsCount(c *CollectionObj, includeFuture bool)
	GetPosts(c *Collection, page int, includeFuture, forceRecentFirst bool) (*[]PublicPost, error)
	GetPostsTagged(c *Collection, tag string, page int, includeFuture bool) (*[]PublicPost, error)

	GetAPFollowers(c *Collection) (*[]RemoteUser, error)
	GetAPActorKeys(collectionID int64) ([]byte, []byte)

	GetDynamicContent(id string) (string, *time.Time, error)
	UpdateDynamicContent(id, content string) error
	GetAllUsers(page uint) (*[]User, error)
	GetAllUsersCount() int64
	GetUserLastPostTime(id int64) (*time.Time, error)
	GetCollectionLastPostTime(id int64) (*time.Time, error)
}

type datastore struct {
	*sql.DB
	driverName string
}

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

func (db *datastore) dateSub(l int, unit string) string {
	if db.driverName == driverSQLite {
		return fmt.Sprintf("DATETIME('now', '-%d %s')", l, unit)
	}
	return fmt.Sprintf("DATE_SUB(NOW(), INTERVAL %d %s)", l, unit)
}

func (db *datastore) CreateUser(u *User, collectionTitle string) error {
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
	res, err = t.Exec("INSERT INTO collections (alias, title, description, privacy, owner_id, view_count) VALUES (?, ?, ?, ?, ?, ?)", u.Username, collectionTitle, "", CollUnlisted, u.ID, 0)
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
func (db *datastore) UpdateUserEmail(keys *keychain, userID int64, email string) error {
	encEmail, err := data.Encrypt(keys.emailKey, email)
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

func (db *datastore) CreateCollectionFromToken(alias, title, accessToken string) (*Collection, error) {
	userID := db.GetUserID(accessToken)
	if userID == -1 {
		return nil, ErrBadAccessToken
	}

	return db.CreateCollection(alias, title, userID)
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

func (db *datastore) CreateCollection(alias, title string, userID int64) (*Collection, error) {
	if db.PostIDExists(alias) {
		return nil, impart.HTTPError{http.StatusConflict, "Invalid collection name."}
	}

	// All good, so create new collection
	res, err := db.Exec("INSERT INTO collections (alias, title, description, privacy, owner_id, view_count) VALUES (?, ?, ?, ?, ?, ?)", alias, title, "", CollUnlisted, userID, 0)
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
	}

	c.ID, err = res.LastInsertId()
	if err != nil {
		log.Error("Couldn't get collection LastInsertId: %v\n", err)
	}

	return c, nil
}

func (db *datastore) GetUserByID(id int64) (*User, error) {
	u := &User{ID: id}

	err := db.QueryRow("SELECT username, password, email, created FROM users WHERE id = ?", id).Scan(&u.Username, &u.HashedPass, &u.Email, &u.Created)
	switch {
	case err == sql.ErrNoRows:
		return nil, ErrUserNotFound
	case err != nil:
		log.Error("Couldn't SELECT user password: %v", err)
		return nil, err
	}

	return u, nil
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

	err := db.QueryRow("SELECT id, password, email, created FROM users WHERE username = ?", username).Scan(&u.ID, &u.HashedPass, &u.Email, &u.Created)
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

	err := db.QueryRow("SELECT id, password, email, created FROM users WHERE id = ?", u.ID).Scan(&u.ID, &u.HashedPass, &u.Email, &u.Created)
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
	err := db.QueryRow("SELECT username, one_time FROM accesstokens LEFT JOIN users ON user_id = id WHERE token = ? AND (expires IS NULL OR expires > NOW())", t).Scan(&username, &oneTime)
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
	err := db.QueryRow("SELECT user_id, username, one_time FROM accesstokens LEFT JOIN users ON user_id = id WHERE token = ? AND (expires IS NULL OR expires > NOW())", t).Scan(&userID, &username, &oneTime)
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
	err := db.QueryRow("SELECT user_id, sudo, one_time FROM accesstokens WHERE token = ? AND (expires IS NULL OR expires > NOW())", t).Scan(&userID, &sudo, &oneTime)
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
	res, err := db.Exec("DELETE FROM accesstokens WHERE token = ?", accessToken)
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
	err := db.QueryRow("SELECT token FROM accesstokens WHERE user_id = ? AND (expires IS NULL OR expires > NOW()) ORDER BY created DESC LIMIT 1", userID).Scan(&t)
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
		expirationVal = fmt.Sprintf("DATE_ADD(NOW(), INTERVAL %d SECOND)", validSecs)
	}

	_, err = db.Exec("INSERT INTO accesstokens (token, user_id, one_time, expires) VALUES (?, ?, ?, "+expirationVal+")", string(binTok), userID, oneTime)
	if err != nil {
		log.Error("Couldn't INSERT accesstoken: %v", err)
		return "", err
	}

	return u.String(), nil
}

func (db *datastore) CreateOwnedPost(post *SubmittedPost, accessToken, collAlias string) (*PublicPost, error) {
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
	friendlyID := store.GenerateFriendlyRandomString(idLen)

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
			if post.Title != nil && *post.Title != "" {
				slugVal = getSlug(*post.Title, post.Language.String)
				if slugVal == "" {
					slugVal = getSlug(*post.Content, post.Language.String)
				}
			} else {
				slugVal = getSlug(*post.Content, post.Language.String)
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
	if post.Created != nil {
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
	var styleSheet, script, format zero.String
	row := db.QueryRow("SELECT id, alias, title, description, style_sheet, script, format, owner_id, privacy, view_count FROM collections WHERE "+condition, value)

	err := row.Scan(&c.ID, &c.Alias, &c.Title, &c.Description, &styleSheet, &script, &format, &c.OwnerID, &c.Visibility, &c.Views)
	switch {
	case err == sql.ErrNoRows:
		return nil, impart.HTTPError{http.StatusNotFound, "Collection doesn't exist."}
	case err != nil:
		log.Error("Failed selecting from collections: %v", err)
		return nil, err
	}
	c.StyleSheet = styleSheet.String
	c.Script = script.String
	c.Format = format.String
	c.Public = c.IsPublic()

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

func (db *datastore) UpdateCollection(c *SubmittedCollection, alias string) error {
	q := query.NewUpdate().
		SetStringPtr(c.Title, "title").
		SetStringPtr(c.Description, "description").
		SetNullString(c.StyleSheet, "style_sheet").
		SetNullString(c.Script, "script")

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

	if q.Updates == "" {
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

	// Update rest of the collection data
	res, err = db.Exec("UPDATE collections SET "+q.Updates+" WHERE "+q.Conditions, q.Params...)
	if err != nil {
		log.Error("Unable to update collection: %v", err)
		return err
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

	if p.Content == "" {
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

	if p.Content == "" {
		return nil, ErrPostUnpublished
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

	if p.Content == "" {
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

// GetPosts retrieves all standard (non-pinned) posts for the given Collection.
// It will return future posts if `includeFuture` is true.
// TODO: change includeFuture to isOwner, since that's how it's used
func (db *datastore) GetPosts(c *Collection, page int, includeFuture, forceRecentFirst bool) (*[]PublicPost, error) {
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
	rows, err := db.Query("SELECT "+postCols+" FROM posts WHERE collection_id = ? AND pinned_position IS NULL "+timeCondition+" ORDER BY created "+order+limitStr, collID)
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
		p.formatContent(c, includeFuture)

		posts = append(posts, p.processPost())
	}
	err = rows.Err()
	if err != nil {
		log.Error("Error after Next() on rows: %v", err)
	}

	return &posts, nil
}

// GetPostsTagged retrieves all posts on the given Collection that contain the
// given tag.
// It will return future posts if `includeFuture` is true.
// TODO: change includeFuture to isOwner, since that's how it's used
func (db *datastore) GetPostsTagged(c *Collection, tag string, page int, includeFuture bool) (*[]PublicPost, error) {
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
	rows, err := db.Query("SELECT "+postCols+" FROM posts WHERE collection_id = ? AND LOWER(content) RLIKE ? "+timeCondition+" ORDER BY created "+order+limitStr, collID, "#"+strings.ToLower(tag)+"[[:>:]]")
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
		p.formatContent(c, includeFuture)

		posts = append(posts, p.processPost())
	}
	err = rows.Err()
	if err != nil {
		log.Error("Error after Next() on rows: %v", err)
	}

	return &posts, nil
}

func (db *datastore) GetAPFollowers(c *Collection) (*[]RemoteUser, error) {
	rows, err := db.Query("SELECT actor_id, inbox, shared_inbox FROM remotefollows f INNER JOIN remoteusers u ON f.remote_user_id = u.id WHERE collection_id = ?", c.ID)
	if err != nil {
		log.Error("Failed selecting from followers: %v", err)
		return nil, impart.HTTPError{http.StatusInternalServerError, "Couldn't retrieve followers."}
	}
	defer rows.Close()

	followers := []RemoteUser{}
	for rows.Next() {
		f := RemoteUser{}
		err = rows.Scan(&f.ActorID, &f.Inbox, &f.SharedInbox)
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

func (db *datastore) ClaimPosts(userID int64, collAlias string, posts *[]ClaimPostRequest) (*[]ClaimPostResult, error) {
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
				coll, err = db.CreateCollection(postCollAlias, "", userID)
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

func (db *datastore) GetPinnedPosts(coll *CollectionObj) (*[]PublicPost, error) {
	// FIXME: sqlite-backed instances don't include ellipsis on truncated titles
	rows, err := db.Query("SELECT id, slug, title, "+db.clip("content", 80)+", pinned_position FROM posts WHERE collection_id = ? AND pinned_position IS NOT NULL ORDER BY pinned_position ASC", coll.ID)
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

		pp := p.processPost()
		pp.Collection = coll
		posts = append(posts, pp)
	}
	return &posts, nil
}

func (db *datastore) GetCollections(u *User) (*[]Collection, error) {
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
		c.URL = c.CanonicalURL()
		c.Public = c.IsPublic()

		colls = append(colls, c)
	}
	err = rows.Err()
	if err != nil {
		log.Error("Error after Next() on rows: %v", err)
	}

	return &colls, nil
}

func (db *datastore) GetPublishableCollections(u *User) (*[]Collection, error) {
	c, err := db.GetCollections(u)
	if err != nil {
		return nil, err
	}

	if len(*c) == 0 {
		return nil, impart.HTTPError{http.StatusInternalServerError, "You don't seem to have any blogs; they might've moved to another account. Try logging out and logging into your other account."}
	}
	return c, nil
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
	err = db.QueryRow(`SELECT COUNT(*) FROM collections`).Scan(&collCount)
	if err != nil {
		log.Error("Unable to fetch collections count: %v", err)
	}
	return
}

func (db *datastore) GetTotalPosts() (postCount int64, err error) {
	err = db.QueryRow(`SELECT COUNT(*) FROM posts`).Scan(&postCount)
	if err != nil {
		log.Error("Unable to fetch posts count: %v", err)
	}
	return
}

func (db *datastore) GetTopPosts(u *User, alias string) (*[]PublicPost, error) {
	params := []interface{}{u.ID}
	where := ""
	if alias != "" {
		where = " AND alias = ?"
		params = append(params, alias)
	}
	rows, err := db.Query("SELECT p.id, p.slug, p.view_count, p.title, c.alias, c.title, c.description, c.view_count FROM posts p LEFT JOIN collections c ON p.collection_id = c.id WHERE p.owner_id = ?"+where+" ORDER BY p.view_count DESC, created DESC LIMIT 25", params...)
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
		err = rows.Scan(&p.ID, &p.Slug, &p.ViewCount, &p.Title, &alias, &title, &description, &views)
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

func (db *datastore) GetAnonymousPosts(u *User) (*[]PublicPost, error) {
	rows, err := db.Query("SELECT id, view_count, title, created, updated, content FROM posts WHERE owner_id = ? AND collection_id IS NULL ORDER BY created DESC", u.ID)
	if err != nil {
		log.Error("Failed selecting from posts: %v", err)
		return nil, impart.HTTPError{http.StatusInternalServerError, "Couldn't retrieve user anonymous posts."}
	}
	defer rows.Close()

	posts := []PublicPost{}
	for rows.Next() {
		p := Post{}
		err = rows.Scan(&p.ID, &p.ViewCount, &p.Title, &p.Created, &p.Updated, &p.Content)
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
func (db *datastore) ChangeSettings(app *app, u *User, s *userSettings) error {
	var errPass error
	q := query.NewUpdate()

	// Update email if given
	if s.Email != "" {
		encEmail, err := data.Encrypt(app.keys.emailKey, s.Email)
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
	if err != nil && err != sql.ErrNoRows {
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

func (db *datastore) DeleteAccount(userID int64) (l *string, err error) {
	debug := ""
	l = &debug

	t, err := db.Begin()
	if err != nil {
		stringLogln(l, "Unable to begin: %v", err)
		return
	}

	// Get all collections
	rows, err := db.Query("SELECT id, alias FROM collections WHERE owner_id = ?", userID)
	if err != nil {
		t.Rollback()
		stringLogln(l, "Unable to get collections: %v", err)
		return
	}
	defer rows.Close()
	colls := []Collection{}
	var c Collection
	for rows.Next() {
		err = rows.Scan(&c.ID, &c.Alias)
		if err != nil {
			t.Rollback()
			stringLogln(l, "Unable to scan collection cols: %v", err)
			return
		}
		colls = append(colls, c)
	}

	var res sql.Result
	for _, c := range colls {
		// TODO: user deleteCollection() func
		// Delete tokens
		res, err = t.Exec("DELETE FROM collectionattributes WHERE collection_id = ?", c.ID)
		if err != nil {
			t.Rollback()
			stringLogln(l, "Unable to delete attributes on %s: %v", c.Alias, err)
			return
		}
		rs, _ := res.RowsAffected()
		stringLogln(l, "Deleted %d for %s from collectionattributes", rs, c.Alias)

		// Remove any optional collection password
		res, err = t.Exec("DELETE FROM collectionpasswords WHERE collection_id = ?", c.ID)
		if err != nil {
			t.Rollback()
			stringLogln(l, "Unable to delete passwords on %s: %v", c.Alias, err)
			return
		}
		rs, _ = res.RowsAffected()
		stringLogln(l, "Deleted %d for %s from collectionpasswords", rs, c.Alias)

		// Remove redirects to this collection
		res, err = t.Exec("DELETE FROM collectionredirects WHERE new_alias = ?", c.Alias)
		if err != nil {
			t.Rollback()
			stringLogln(l, "Unable to delete redirects on %s: %v", c.Alias, err)
			return
		}
		rs, _ = res.RowsAffected()
		stringLogln(l, "Deleted %d for %s from collectionredirects", rs, c.Alias)
	}

	// Delete collections
	res, err = t.Exec("DELETE FROM collections WHERE owner_id = ?", userID)
	if err != nil {
		t.Rollback()
		stringLogln(l, "Unable to delete collections: %v", err)
		return
	}
	rs, _ := res.RowsAffected()
	stringLogln(l, "Deleted %d from collections", rs)

	// Delete tokens
	res, err = t.Exec("DELETE FROM accesstokens WHERE user_id = ?", userID)
	if err != nil {
		t.Rollback()
		stringLogln(l, "Unable to delete access tokens: %v", err)
		return
	}
	rs, _ = res.RowsAffected()
	stringLogln(l, "Deleted %d from accesstokens", rs)

	// Delete posts
	res, err = t.Exec("DELETE FROM posts WHERE owner_id = ?", userID)
	if err != nil {
		t.Rollback()
		stringLogln(l, "Unable to delete posts: %v", err)
		return
	}
	rs, _ = res.RowsAffected()
	stringLogln(l, "Deleted %d from posts", rs)

	res, err = t.Exec("DELETE FROM userattributes WHERE user_id = ?", userID)
	if err != nil {
		t.Rollback()
		stringLogln(l, "Unable to delete attributes: %v", err)
		return
	}
	rs, _ = res.RowsAffected()
	stringLogln(l, "Deleted %d from userattributes", rs)

	res, err = t.Exec("DELETE FROM users WHERE id = ?", userID)
	if err != nil {
		t.Rollback()
		stringLogln(l, "Unable to delete user: %v", err)
		return
	}
	rs, _ = res.RowsAffected()
	stringLogln(l, "Deleted %d from users", rs)

	err = t.Commit()
	if err != nil {
		t.Rollback()
		stringLogln(l, "Unable to commit: %v", err)
		return
	}

	return
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

func (db *datastore) GetDynamicContent(id string) (string, *time.Time, error) {
	var c string
	var u *time.Time
	err := db.QueryRow("SELECT content, updated FROM appcontent WHERE id = ?", id).Scan(&c, &u)
	switch {
	case err == sql.ErrNoRows:
		return "", nil, nil
	case err != nil:
		log.Error("Couldn't SELECT FROM appcontent for id '%s': %v", id, err)
		return "", nil, err
	}
	return c, u, nil
}

func (db *datastore) UpdateDynamicContent(id, content string) error {
	var err error
	if db.driverName == driverSQLite {
		_, err = db.Exec("INSERT OR REPLACE INTO appcontent (id, content, updated) VALUES (?, ?, "+db.now()+")", id, content)
	} else {
		_, err = db.Exec("INSERT INTO appcontent (id, content, updated) VALUES (?, ?, "+db.now()+") "+db.upsert("id")+" content = ?, updated = "+db.now(), id, content, content)
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

	rows, err := db.Query("SELECT id, username, created FROM users ORDER BY created DESC LIMIT " + limitStr)
	if err != nil {
		log.Error("Failed selecting from posts: %v", err)
		return nil, impart.HTTPError{http.StatusInternalServerError, "Couldn't retrieve user posts."}
	}
	defer rows.Close()

	users := []User{}
	for rows.Next() {
		u := User{}
		err = rows.Scan(&u.ID, &u.Username, &u.Created)
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

func stringLogln(log *string, s string, v ...interface{}) {
	*log += fmt.Sprintf(s+"\n", v...)
}

func handleFailedPostInsert(err error) error {
	log.Error("Couldn't insert into posts: %v", err)
	return err
}
