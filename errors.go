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
	"net/http"

	"github.com/writeas/impart"
)

// Commonly returned HTTP errors
var (
	ErrBadFormData    = impart.HTTPError{http.StatusBadRequest, "Expected valid form data."}
	ErrBadJSON        = impart.HTTPError{http.StatusBadRequest, "Expected valid JSON object."}
	ErrBadJSONArray   = impart.HTTPError{http.StatusBadRequest, "Expected valid JSON array."}
	ErrBadAccessToken = impart.HTTPError{http.StatusUnauthorized, "Invalid access token."}
	ErrNoAccessToken  = impart.HTTPError{http.StatusBadRequest, "Authorization token required."}
	ErrNotLoggedIn    = impart.HTTPError{http.StatusUnauthorized, "Not logged in."}

	ErrForbiddenCollection        = impart.HTTPError{http.StatusForbidden, "You don't have permission to add to this collection."}
	ErrForbiddenEditPost          = impart.HTTPError{http.StatusForbidden, "You don't have permission to update this post."}
	ErrUnauthorizedEditPost       = impart.HTTPError{http.StatusUnauthorized, "Invalid editing credentials."}
	ErrUnauthorizedGeneral        = impart.HTTPError{http.StatusUnauthorized, "You don't have permission to do that."}
	ErrBadRequestedType           = impart.HTTPError{http.StatusNotAcceptable, "Bad requested Content-Type."}
	ErrCollectionUnauthorizedRead = impart.HTTPError{http.StatusUnauthorized, "You don't have permission to access this collection."}

	ErrNoPublishableContent = impart.HTTPError{http.StatusBadRequest, "Supply something to publish."}

	ErrInternalGeneral       = impart.HTTPError{http.StatusInternalServerError, "The humans messed something up. They've been notified."}
	ErrInternalCookieSession = impart.HTTPError{http.StatusInternalServerError, "Could not get cookie session."}

	ErrCollectionNotFound     = impart.HTTPError{http.StatusNotFound, "Collection doesn't exist."}
	ErrCollectionGone         = impart.HTTPError{http.StatusGone, "This blog was unpublished."}
	ErrCollectionPageNotFound = impart.HTTPError{http.StatusNotFound, "Collection page doesn't exist."}
	ErrPostNotFound           = impart.HTTPError{Status: http.StatusNotFound, Message: "Post not found."}
	ErrPostBanned             = impart.HTTPError{Status: http.StatusGone, Message: "Post removed."}
	ErrPostUnpublished        = impart.HTTPError{Status: http.StatusGone, Message: "Post unpublished by author."}
	ErrPostFetchError         = impart.HTTPError{Status: http.StatusInternalServerError, Message: "We encountered an error getting the post. The humans have been alerted."}

	ErrUserNotFound      = impart.HTTPError{http.StatusNotFound, "User doesn't exist."}
	ErrUserNotFoundEmail = impart.HTTPError{http.StatusNotFound, "Please enter your username instead of your email address."}

	ErrUserSuspended = impart.HTTPError{http.StatusForbidden, "Account is suspended, contact the administrator."}
)

// Post operation errors
var (
	ErrPostNoUpdatableVals = impart.HTTPError{http.StatusBadRequest, "Supply some properties to update."}
)
