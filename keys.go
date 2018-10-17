package writefreely

import (
	"io/ioutil"
)

type keychain struct {
	emailKey, cookieAuthKey, cookieKey []byte
}

func initKeys(app *app) error {
	var err error
	app.keys = &keychain{}

	app.keys.emailKey, err = ioutil.ReadFile("keys/email.aes256")
	if err != nil {
		return err
	}

	app.keys.cookieAuthKey, err = ioutil.ReadFile("keys/cookies_auth.aes256")
	if err != nil {
		return err
	}

	app.keys.cookieKey, err = ioutil.ReadFile("keys/cookies_enc.aes256")
	if err != nil {
		return err
	}

	return nil
}
