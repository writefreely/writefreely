package writefreely

import (
	"io/ioutil"
	"path/filepath"
)

const (
	keysDir = "keys"
)

var (
	emailKeyPath      = filepath.Join(keysDir, "email.aes256")
	cookieAuthKeyPath = filepath.Join(keysDir, "cookies_auth.aes256")
	cookieKeyPath     = filepath.Join(keysDir, "cookies_enc.aes256")
)

type keychain struct {
	emailKey, cookieAuthKey, cookieKey []byte
}

func initKeys(app *app) error {
	var err error
	app.keys = &keychain{}

	app.keys.emailKey, err = ioutil.ReadFile(emailKeyPath)
	if err != nil {
		return err
	}

	app.keys.cookieAuthKey, err = ioutil.ReadFile(cookieAuthKeyPath)
	if err != nil {
		return err
	}

	app.keys.cookieKey, err = ioutil.ReadFile(cookieKeyPath)
	if err != nil {
		return err
	}

	return nil
}
