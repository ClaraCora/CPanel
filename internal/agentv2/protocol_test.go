package agentv2

import (
	"crypto/ed25519"
	"errors"
	"testing"
	"time"
)

func testSessions(t *testing.T) (*Session, *Session, HandshakeRequest) {
	t.Helper()
	panelPublic, panelPrivate, err := GenerateIdentity()
	if err != nil {
		t.Fatal(err)
	}
	agentPublic, agentPrivate, err := GenerateIdentity()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_800_000_000, 0)
	clientHandshake, err := NewClientHandshake("mch_test", agentPublic, agentPrivate, []byte("0123456789abcdef0123456789abcdef"), false, now)
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyEnrollment(clientHandshake.Request, []byte("0123456789abcdef0123456789abcdef")) {
		t.Fatal("enrollment proof did not verify")
	}
	response, serverSession, err := NewServerHandshake(clientHandshake.Request, panelPrivate, true, now)
	if err != nil {
		t.Fatal(err)
	}
	clientSession, err := clientHandshake.Complete(response, panelPublic)
	if err != nil {
		t.Fatal(err)
	}
	return clientSession, serverSession, clientHandshake.Request
}

func TestHandshakeAndDirectionalEncryption(t *testing.T) {
	client, server, _ := testSessions(t)
	request, err := client.Seal("POST", "/ca/cc/fwq/jd", []byte(`{"fwqbh":"mch_test"}`))
	if err != nil {
		t.Fatal(err)
	}
	plain, err := server.Open("POST", "/ca/cc/fwq/jd", request)
	if err != nil {
		t.Fatal(err)
	}
	if string(plain) != `{"fwqbh":"mch_test"}` {
		t.Fatalf("unexpected request plaintext %q", plain)
	}
	response, err := server.Seal("POST", "/ca/cc/fwq/jd", []byte(`{"jd":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	plain, err = client.Open("POST", "/ca/cc/fwq/jd", response)
	if err != nil {
		t.Fatal(err)
	}
	if string(plain) != `{"jd":[]}` {
		t.Fatalf("unexpected response plaintext %q", plain)
	}
}

func TestEnvelopeBindsMethodPathAndMachine(t *testing.T) {
	client, server, _ := testSessions(t)
	envelope, err := client.Seal("POST", "/ca/cc/bg", []byte(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := server.Open("POST", "/ca/cc/fwq/jd", envelope); !errors.Is(err, ErrInvalidEnvelope) {
		t.Fatalf("wrong path error = %v", err)
	}
	if _, err := server.Open("GET", "/ca/cc/bg", envelope); !errors.Is(err, ErrInvalidEnvelope) {
		t.Fatalf("wrong method error = %v", err)
	}
	if _, err := server.Open("POST", "/ca/cc/bg", envelope); err != nil {
		t.Fatalf("valid envelope rejected after forged attempts: %v", err)
	}
}

func TestForgedHighSequenceDoesNotAdvanceReplayWindow(t *testing.T) {
	client, server, _ := testSessions(t)
	valid, err := client.Seal("POST", "/ca/cc/bg", []byte(`{"yb":"0"}`))
	if err != nil {
		t.Fatal(err)
	}
	forged := append([]byte(nil), valid...)
	for index := 17; index < 25; index++ {
		forged[index] = 0xff
	}
	if _, err := server.Open("POST", "/ca/cc/bg", forged); !errors.Is(err, ErrInvalidEnvelope) {
		t.Fatalf("forged envelope error = %v", err)
	}
	if _, err := server.Open("POST", "/ca/cc/bg", valid); err != nil {
		t.Fatalf("valid envelope rejected after forged high sequence: %v", err)
	}
	if _, err := server.Open("POST", "/ca/cc/bg", valid); !errors.Is(err, ErrReplay) {
		t.Fatalf("replayed envelope error = %v", err)
	}
}

func TestIdentityHandshakeAndPinnedPanelKey(t *testing.T) {
	panelPublic, panelPrivate, err := GenerateIdentity()
	if err != nil {
		t.Fatal(err)
	}
	agentPublic, agentPrivate, err := GenerateIdentity()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	handshake, err := NewClientHandshake("mch_identity", agentPublic, agentPrivate, nil, true, now)
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyIdentity(handshake.Request, agentPublic) {
		t.Fatal("Agent identity proof did not verify")
	}
	response, _, err := NewServerHandshake(handshake.Request, panelPrivate, true, now)
	if err != nil {
		t.Fatal(err)
	}
	wrongPublic, _, _ := ed25519.GenerateKey(nil)
	if _, err := handshake.Complete(response, wrongPublic); err == nil {
		t.Fatal("handshake accepted an unpinned panel identity")
	}
	if _, err := handshake.Complete(response, panelPublic); err != nil {
		t.Fatalf("handshake rejected pinned panel identity: %v", err)
	}
}

func TestHandshakeTimestampValidation(t *testing.T) {
	_, _, request := testSessions(t)
	now := time.Unix(request.Timestamp, 0)
	if err := ValidateHandshakeShape(request, now, 2*time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := ValidateHandshakeShape(request, now.Add(3*time.Minute), 2*time.Minute); err == nil {
		t.Fatal("stale handshake timestamp was accepted")
	}
}
