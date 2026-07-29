package securebox

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

func TestSealRoundTrip(t *testing.T) {
	key := base64.RawStdEncoding.EncodeToString(make([]byte, 32))
	box, err := New(key)
	if err != nil {
		t.Fatal(err)
	}
	input := json.RawMessage(`{"token":"secret"}`)
	sealed, err := box.Seal(input)
	if err != nil {
		t.Fatal(err)
	}
	if string(sealed) == string(input) {
		t.Fatal("sealed value contains plaintext")
	}
	opened, err := box.Open(sealed)
	if err != nil {
		t.Fatal(err)
	}
	if string(opened) != string(input) {
		t.Fatalf("round trip = %s, want %s", opened, input)
	}
}
