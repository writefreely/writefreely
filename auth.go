package writefreely

// AuthenticateUser ensures a user with the given accessToken is valid. Call
// it before any operations that require authentication or optionally associate
// data with a user account.
// Returns an error if the given accessToken is invalid. Otherwise the
// associated user ID is returned.
func AuthenticateUser(db writestore, accessToken string) (int64, error) {
	if accessToken == "" {
		return 0, ErrNoAccessToken
	}
	userID := db.GetUserID(accessToken)
	if userID == -1 {
		return 0, ErrBadAccessToken
	}

	return userID, nil
}
