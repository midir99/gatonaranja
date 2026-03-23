package main

import "slices"

// UserIsAuthorized reports whether the given Telegram user ID is allowed to
// use the bot. If no authorized user IDs are configured, it allows everyone.
func UserIsAuthorized(userID int64, authorizedUserIDs []int64) bool {
	if len(authorizedUserIDs) == 0 {
		return true
	}
	return slices.Contains(authorizedUserIDs, userID)
}
