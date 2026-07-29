package auth

import "testing"

func TestPasswordRoundTrip(t *testing.T) {
	encoded, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyPassword(encoded, "correct horse battery staple") {
		t.Fatal("expected password to verify")
	}
	if VerifyPassword(encoded, "wrong password") {
		t.Fatal("wrong password verified")
	}
}

func TestPasswordMinimumLength(t *testing.T) {
	if _, err := HashPassword("too-short"); err == nil {
		t.Fatal("expected short password to fail")
	}
}
