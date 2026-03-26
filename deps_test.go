package main

import (
	"errors"
	"testing"
)

func TestValidateRequiredDependencies(t *testing.T) {
	t.Run("all dependencies are available", func(t *testing.T) {
		productionLookPath := lookPath
		lookPath = func(file string) (string, error) {
			return "/usr/bin/" + file, nil
		}
		defer func() {
			lookPath = productionLookPath
		}()

		err := ValidateRequiredDependencies()
		if err != nil {
			t.Fatalf("got error %q, want nil", err.Error())
		}
	})

	t.Run("missing dependency returns error", func(t *testing.T) {
		productionLookPath := lookPath
		lookPath = func(file string) (string, error) {
			if file == "yt-dlp" {
				return "", errors.New("not found")
			}
			return "/usr/bin/" + file, nil
		}
		defer func() {
			lookPath = productionLookPath
		}()

		err := ValidateRequiredDependencies()
		if err == nil {
			t.Fatal("got nil error, want error")
		}
		if err.Error() != `dependency yt-dlp is not installed in the system: not found` {
			t.Fatalf("got %q, want %q", err.Error(), `dependency yt-dlp is not installed in the system: not found`)
		}
	})
}
