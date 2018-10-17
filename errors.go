package writefreely

import (
	"github.com/writeas/impart"
	"net/http"
)

// Commonly returned HTTP errors
var (
	ErrBadAccessToken = impart.HTTPError{http.StatusUnauthorized, "Invalid access token."}
	ErrNoAccessToken  = impart.HTTPError{http.StatusBadRequest, "Authorization token required."}

	ErrForbiddenCollection  = impart.HTTPError{http.StatusForbidden, "You don't have permission to add to this collection."}
	ErrUnauthorizedEditPost = impart.HTTPError{http.StatusUnauthorized, "Invalid editing credentials."}
	ErrUnauthorizedGeneral  = impart.HTTPError{http.StatusUnauthorized, "You don't have permission to do that."}

	ErrInternalGeneral = impart.HTTPError{http.StatusInternalServerError, "The humans messed something up. They've been notified."}

	ErrCollectionPageNotFound = impart.HTTPError{http.StatusNotFound, "Collection page doesn't exist."}
	ErrPostNotFound           = impart.HTTPError{Status: http.StatusNotFound, Message: "Post not found."}
	ErrPostUnpublished        = impart.HTTPError{Status: http.StatusGone, Message: "Post unpublished by author."}
	ErrPostFetchError         = impart.HTTPError{Status: http.StatusInternalServerError, Message: "We encountered an error getting the post. The humans have been alerted."}

	ErrUserNotFound = impart.HTTPError{http.StatusNotFound, "User doesn't exist."}
)

// Post operation errors
var (
	ErrPostNoUpdatableVals = impart.HTTPError{http.StatusBadRequest, "Supply some properties to update."}
)
