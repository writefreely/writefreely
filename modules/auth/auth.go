package auth

import (
	uuid "github.com/nu7hatch/gouuid"
	"strings"

	"code.as/writeas/web/modules/log"
)

// GetToken parses out the user token from an Authorization header.
func GetToken(header string) []byte {
	var accessToken []byte
	if len(header) > 0 {
		f := strings.Fields(header)
		if len(f) == 2 && f[0] == "Token" {
			t, err := uuid.ParseHex(f[1])
			if err != nil {
				log.Error("Couldn't parseHex on '%s': %v", accessToken, err)
			} else {
				accessToken = t[:]
			}
		}
	}
	return accessToken
}
