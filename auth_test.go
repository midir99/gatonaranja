package main

import "testing"

func TestUserIsAuthorized(t *testing.T) {
	t.Run("authorizedUserIDs is empty", func(t *testing.T) {
		// If authorizedUserIDs is an empty list, everyone is authorized
		authorizedUserIDs := []int64{}
		var userID int64 = 22
		got := UserIsAuthorized(userID, authorizedUserIDs)
		want := true
		if got != want {
			t.Fatalf("got %v, want %v", got, want)
		}
	})

	t.Run("userID is authorized", func(t *testing.T) {
		// If authorizedUserIDs is not empty, only the ids in the list are authorized
		authorizedUserIDs := []int64{1, 2, 3, 4, 5}
		var userID int64 = 2
		got := UserIsAuthorized(userID, authorizedUserIDs)
		want := true
		if got != want {
			t.Fatalf("got %v, want %v", got, want)
		}
	})

	t.Run("userID is not authorized", func(t *testing.T) {
		// If authorizedUserIDs is not empty, only the ids in the list are authorized
		authorizedUserIDs := []int64{1, 2, 3, 4, 5}
		var userID int64 = 22
		got := UserIsAuthorized(userID, authorizedUserIDs)
		want := false
		if got != want {
			t.Fatalf("got %v, want %v", got, want)
		}
	})
}
