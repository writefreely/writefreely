/*
 * Copyright © 2018-2020 Musing Studio LLC.
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

	ErrUnavailable = impart.HTTPError{http.StatusServiceUnavailable, "Service temporarily unavailable due to high load."}

	ErrCollectionNotFound     = impart.HTTPError{http.StatusNotFound, "Collection doesn't exist."}
	ErrCollectionGone         = impart.HTTPError{http.StatusGone, "This blog was unpublished."}
	ErrCollectionPageNotFound = impart.HTTPError{http.StatusNotFound, "Collection page doesn't exist."}
	ErrPostNotFound           = impart.HTTPError{Status: http.StatusNotFound, Message: "Post not found."}
	ErrPostBanned             = impart.HTTPError{Status: http.StatusGone, Message: "Post removed."}
	ErrPostUnpublished        = impart.HTTPError{Status: http.StatusGone, Message: "Post unpublished by author."}
	ErrPostFetchError         = impart.HTTPError{Status: http.StatusInternalServerError, Message: "We encountered an error getting the post. The humans have been alerted."}

	//ErrUserNotFound       = impart.HTTPError{http.StatusNotFound, "User doesn't exist."}
	ErrUserNotFound       = impart.HTTPError{http.StatusNotFound, "L'utente non esiste, <a rel='me' href='https://livellosegreto.it/@log'>richiedi un invito per crearlo</a>.<br/> Se ne hai già uno, collegalo al tuo account LS da Account Settings, Linked Accounts.<br/><br/> User doesn't exist, <a rel='me' href='https://livellosegreto.it/@log'>request an invitation to create it.</a>.<br/> If you already have one, link it to your LS account from Account Settings, Linked Accounts."}
	ErrRemoteUserNotFound = impart.HTTPError{http.StatusNotFound, "Remote user not found."}
	ErrUserNotFoundEmail  = impart.HTTPError{http.StatusNotFound, "Please enter your username instead of your email address."}

	ErrUserSilenced = impart.HTTPError{http.StatusForbidden, "Account is silenced."}

	ErrDisabledPasswordAuth = impart.HTTPError{http.StatusForbidden, "Password authentication is disabled."}
)

// Post operation errors
var (
	ErrPostNoUpdatableVals = impart.HTTPError{http.StatusBadRequest, "Supply some properties to update."}
)
