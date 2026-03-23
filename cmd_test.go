package main

import "testing"

func compareInt64Array(t *testing.T, got, want []int64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range len(got) {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got[i], want[i])
		}
	}
}

func TestValidateAuthorizedUsers(t *testing.T) {
	t.Run("authorizedUsers is valid", func(t *testing.T) {
		t.Run("empty list", func(t *testing.T) {
			authorizedUsers := ""
			want := []int64{}
			got, err := validateAuthorizedUsers(authorizedUsers)
			if err != nil {
				t.Fatalf("err must be nil, got %s", err)
			}
			compareInt64Array(t, got, want)
		})

		t.Run("non-empty list", func(t *testing.T) {
			authorizedUsers := "1111111111,   1111111112,1111111113"
			want := []int64{1111111111, 1111111112, 1111111113}
			got, err := validateAuthorizedUsers(authorizedUsers)
			if err != nil {
				t.Fatalf("err must be nil, got %s", err)
			}
			compareInt64Array(t, got, want)
		})

		t.Run("duplicate user IDs", func(t *testing.T) {
			authorizedUsers := "1111111111,1111111111,1111111113"
			want := []int64{1111111111, 1111111113}
			got, err := validateAuthorizedUsers(authorizedUsers)
			if err != nil {
				t.Fatalf("err must be nil, got %s", err)
			}
			compareInt64Array(t, got, want)
		})

		t.Run("user IDs with spaces", func(t *testing.T) {
			authorizedUsers := " 1111111111 ,1111111112,   1111111113	\n,1111111114"
			want := []int64{1111111111, 1111111112, 1111111113, 1111111114}
			got, err := validateAuthorizedUsers(authorizedUsers)
			if err != nil {
				t.Fatalf("err must be nil, got %s", err)
			}
			compareInt64Array(t, got, want)
		})
	})

	t.Run("authorizedUsers is not valid", func(t *testing.T) {
		t.Run("invalid values 1", func(t *testing.T) {
			authorizedUsers := "rachelcorrierip,esperanza,123kj432"
			want := `invalid authorized user ID "rachelcorrierip": must be a valid Telegram user ID`
			_, err := validateAuthorizedUsers(authorizedUsers)
			if err == nil {
				t.Fatal("expected an error, got nil")
			}
			if err.Error() != want {
				t.Fatalf("got %q, want %q", err.Error(), want)
			}
		})

		t.Run("invalid values 2", func(t *testing.T) {
			authorizedUsers := "1111111111, 1111 111111 ,1111111113"
			want := `invalid authorized user ID "1111 111111": must be a valid Telegram user ID`
			_, err := validateAuthorizedUsers(authorizedUsers)
			if err == nil {
				t.Fatal("expected an error, got nil")
			}
			if err.Error() != want {
				t.Fatalf("got %q, want %q", err.Error(), want)
			}
		})
	})

}
func TestValidateTelegramBotToken(t *testing.T) {

}
func TestFlagOrEnv(t *testing.T) {

}
func TestParseConfig(t *testing.T) {

}
