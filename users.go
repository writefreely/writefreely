package writefreely

import (
	"time"

	"github.com/guregu/null/zero"
	"github.com/writeas/web-core/data"
	"github.com/writeas/web-core/log"
)

type (
	// User is a consistent user object in the database and all contexts (auth
	// and non-auth) in the API.
	User struct {
		ID         int64       `json:"-"`
		Username   string      `json:"username"`
		HashedPass []byte      `json:"-"`
		HasPass    bool        `json:"has_pass"`
		Email      zero.String `json:"email"`
		Created    time.Time   `json:"created"`

		clearEmail string `json:"email"`
	}
)

// EmailClear decrypts and returns the user's email, caching it in the user
// object.
func (u *User) EmailClear(keys *keychain) string {
	if u.clearEmail != "" {
		return u.clearEmail
	}

	if u.Email.Valid && u.Email.String != "" {
		email, err := data.Decrypt(keys.emailKey, []byte(u.Email.String))
		if err != nil {
			log.Error("Error decrypting user email: %v", err)
		} else {
			u.clearEmail = string(email)
			return u.clearEmail
		}
	}
	return ""
}

// Cookie strips down an AuthUser to contain only information necessary for
// cookies.
func (u User) Cookie() *User {
	u.HashedPass = []byte{}

	return &u
}
