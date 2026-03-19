package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

func UserIsAuthorized(userId int64, authorizedUserIds []int64) bool {
	if len(authorizedUserIds) == 0 {
		return true
	}
	for _, allowedUserId := range authorizedUserIds {
		if userId == allowedUserId {
			return true
		}
	}
	return false
}

func LoadAuthorizedUserIds(authorizedUsersEnv string) ([]int64, error) {
	authorizedUsersEnvContent := strings.TrimSpace(os.Getenv(authorizedUsersEnv))
	if authorizedUsersEnvContent == "" {
		return []int64{}, nil
	}
	authorizedUserIds := strings.Split(authorizedUsersEnvContent, ",")
	ids := []int64{}
	for _, authorizedUserId := range authorizedUserIds {
		id, err := strconv.ParseInt(authorizedUserId, 10, 0)
		if err != nil {
			return []int64{}, fmt.Errorf("unable to parse %s into an int64: %s", authorizedUserId, err)
		}
		ids = append(ids, id)
	}
	return ids, nil
}
