package writefreely

import (
	"fmt"
	"github.com/writeas/impart"
	"github.com/writeas/web-core/auth"
	"net/http"
)

func adminResetPassword(app *app, u *User, newPass string) error {
	hashedPass, err := auth.HashPass([]byte(newPass))
	if err != nil {
		return impart.HTTPError{http.StatusInternalServerError, fmt.Sprintf("Could not create password hash: %v", err)}
	}

	err = app.db.ChangePassphrase(u.ID, true, "", hashedPass)
	if err != nil {
		return impart.HTTPError{http.StatusInternalServerError, fmt.Sprintf("Could not update passphrase: %v", err)}
	}
	return nil
}
