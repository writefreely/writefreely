package writefreely

import (
	"github.com/writeas/impart"
	"net/http"
)

// Commonly returned HTTP errors
var (
	ErrInternalCookieSession = impart.HTTPError{http.StatusInternalServerError, "Could not get cookie session."}
)
