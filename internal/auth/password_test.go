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
	if _, err := HashPassword("1234567"); err == nil {
		t.Fatal("expected seven-character password to fail")
	}
	if _, err := HashPassword("12345678"); err != nil {
		t.Fatalf("expected eight-character password to pass: %v", err)
	}
	if _, err := HashPassword("密码测试八个字符"); err != nil {
		t.Fatalf("expected eight-or-more Unicode characters to pass: %v", err)
	}
}
