package agentv2

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/crypto/hkdf"
)

const (
	ProtocolVersion = "2"
	ContentType     = "application/octet-stream"
	EnvelopeVersion = byte(0x02)

	ModeEnroll  = "dj"
	ModeConnect = "lj"

	sessionIDSize = 16
	nonceSize     = 12
	headerSize    = 1 + sessionIDSize + 8
	tagSize       = 16
	replayBits    = 64
)

var (
	ErrInvalidEnvelope = errors.New("invalid encrypted envelope")
	ErrReplay          = errors.New("replayed or stale encrypted envelope")
	ErrWrongSession    = errors.New("encrypted envelope belongs to another session")
)

type HandshakeRequest struct {
	Version         string `json:"xybb"`
	Mode            string `json:"lx"`
	MachineID       string `json:"fwqbh"`
	IdentityPublic  string `json:"sfgy"`
	EphemeralPublic string `json:"lsgy"`
	Nonce           string `json:"sjm"`
	Timestamp       int64  `json:"sjc"`
	Proof           string `json:"zm"`
}

type HandshakeResponse struct {
	Version         string `json:"xybb"`
	SessionID       string `json:"hhbh"`
	IdentityPublic  string `json:"sfgy"`
	EphemeralPublic string `json:"lsgy"`
	Nonce           string `json:"sjm"`
	Timestamp       int64  `json:"sjc"`
	Enrolled        bool   `json:"ydj"`
	Signature       string `json:"qm"`
}

type ClientHandshake struct {
	Request           HandshakeRequest
	ephemeralPrivate  *ecdh.PrivateKey
	requestTranscript []byte
}

func GenerateIdentity() (ed25519.PublicKey, ed25519.PrivateKey, error) {
	return ed25519.GenerateKey(rand.Reader)
}

func NewClientHandshake(machineID string, identityPublic ed25519.PublicKey, identityPrivate ed25519.PrivateKey, enrollmentKey []byte, enrolled bool, now time.Time) (*ClientHandshake, error) {
	if len(identityPublic) != ed25519.PublicKeySize || len(identityPrivate) != ed25519.PrivateKeySize {
		return nil, errors.New("invalid Agent identity key")
	}
	ephemeralPrivate, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate Agent ephemeral key: %w", err)
	}
	nonce := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate Agent handshake nonce: %w", err)
	}
	request := HandshakeRequest{
		Version:         ProtocolVersion,
		Mode:            ModeEnroll,
		MachineID:       machineID,
		IdentityPublic:  encode(identityPublic),
		EphemeralPublic: encode(ephemeralPrivate.PublicKey().Bytes()),
		Nonce:           encode(nonce),
		Timestamp:       now.UTC().Unix(),
	}
	if enrolled {
		request.Mode = ModeConnect
	}
	transcript := RequestTranscript(request)
	if enrolled {
		request.Proof = encode(ed25519.Sign(identityPrivate, transcript))
	} else {
		if len(enrollmentKey) < 32 {
			return nil, errors.New("enrollment key must contain at least 32 bytes")
		}
		request.Proof = encode(hmacSHA256(enrollmentKey, transcript))
	}
	return &ClientHandshake{Request: request, ephemeralPrivate: ephemeralPrivate, requestTranscript: transcript}, nil
}

func (h *ClientHandshake) Complete(response HandshakeResponse, expectedPanelIdentity ed25519.PublicKey) (*Session, error) {
	if h == nil || h.ephemeralPrivate == nil {
		return nil, errors.New("client handshake is not initialized")
	}
	if response.Version != ProtocolVersion {
		return nil, fmt.Errorf("unsupported response protocol %q", response.Version)
	}
	panelIdentity, err := decodeSized(response.IdentityPublic, ed25519.PublicKeySize)
	if err != nil {
		return nil, fmt.Errorf("decode panel identity: %w", err)
	}
	if !hmac.Equal(panelIdentity, expectedPanelIdentity) {
		return nil, errors.New("panel identity does not match pinned public key")
	}
	serverEphemeral, err := decodeSized(response.EphemeralPublic, 32)
	if err != nil {
		return nil, fmt.Errorf("decode panel ephemeral key: %w", err)
	}
	sessionID, err := decodeSized(response.SessionID, sessionIDSize)
	if err != nil {
		return nil, fmt.Errorf("decode session id: %w", err)
	}
	if _, err := decodeSized(response.Nonce, 32); err != nil {
		return nil, fmt.Errorf("decode panel nonce: %w", err)
	}
	signature, err := decodeSized(response.Signature, ed25519.SignatureSize)
	if err != nil {
		return nil, fmt.Errorf("decode panel signature: %w", err)
	}
	responseTranscript := ResponseTranscript(h.requestTranscript, response)
	if !ed25519.Verify(expectedPanelIdentity, responseTranscript, signature) {
		return nil, errors.New("panel handshake signature is invalid")
	}
	peer, err := ecdh.X25519().NewPublicKey(serverEphemeral)
	if err != nil {
		return nil, fmt.Errorf("parse panel ephemeral key: %w", err)
	}
	shared, err := h.ephemeralPrivate.ECDH(peer)
	if err != nil {
		return nil, fmt.Errorf("derive client shared secret: %w", err)
	}
	return deriveSession(shared, transcriptHash(h.requestTranscript, responseTranscript), sessionID, h.Request.MachineID, false)
}

func VerifyEnrollment(request HandshakeRequest, enrollmentKey []byte) bool {
	proof, err := decodeSized(request.Proof, sha256.Size)
	if err != nil || len(enrollmentKey) < 32 || request.Mode != ModeEnroll {
		return false
	}
	return hmac.Equal(proof, hmacSHA256(enrollmentKey, RequestTranscript(request)))
}

func VerifyIdentity(request HandshakeRequest, expectedIdentity ed25519.PublicKey) bool {
	proof, err := decodeSized(request.Proof, ed25519.SignatureSize)
	if err != nil || request.Mode != ModeConnect || len(expectedIdentity) != ed25519.PublicKeySize {
		return false
	}
	claimed, err := decodeSized(request.IdentityPublic, ed25519.PublicKeySize)
	if err != nil || !hmac.Equal(claimed, expectedIdentity) {
		return false
	}
	return ed25519.Verify(expectedIdentity, RequestTranscript(request), proof)
}

func IdentityPublic(request HandshakeRequest) (ed25519.PublicKey, error) {
	decoded, err := decodeSized(request.IdentityPublic, ed25519.PublicKeySize)
	if err != nil {
		return nil, err
	}
	return ed25519.PublicKey(decoded), nil
}

func NewServerHandshake(request HandshakeRequest, panelIdentity ed25519.PrivateKey, enrolled bool, now time.Time) (HandshakeResponse, *Session, error) {
	if len(panelIdentity) != ed25519.PrivateKeySize {
		return HandshakeResponse{}, nil, errors.New("invalid panel identity private key")
	}
	agentEphemeral, err := decodeSized(request.EphemeralPublic, 32)
	if err != nil {
		return HandshakeResponse{}, nil, fmt.Errorf("decode Agent ephemeral key: %w", err)
	}
	peer, err := ecdh.X25519().NewPublicKey(agentEphemeral)
	if err != nil {
		return HandshakeResponse{}, nil, fmt.Errorf("parse Agent ephemeral key: %w", err)
	}
	ephemeralPrivate, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return HandshakeResponse{}, nil, fmt.Errorf("generate panel ephemeral key: %w", err)
	}
	shared, err := ephemeralPrivate.ECDH(peer)
	if err != nil {
		return HandshakeResponse{}, nil, fmt.Errorf("derive server shared secret: %w", err)
	}
	sessionID := make([]byte, sessionIDSize)
	serverNonce := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, sessionID); err != nil {
		return HandshakeResponse{}, nil, fmt.Errorf("generate session id: %w", err)
	}
	if _, err := io.ReadFull(rand.Reader, serverNonce); err != nil {
		return HandshakeResponse{}, nil, fmt.Errorf("generate panel nonce: %w", err)
	}
	response := HandshakeResponse{
		Version:         ProtocolVersion,
		SessionID:       encode(sessionID),
		IdentityPublic:  encode(panelIdentity.Public().(ed25519.PublicKey)),
		EphemeralPublic: encode(ephemeralPrivate.PublicKey().Bytes()),
		Nonce:           encode(serverNonce),
		Timestamp:       now.UTC().Unix(),
		Enrolled:        enrolled,
	}
	requestTranscript := RequestTranscript(request)
	responseTranscript := ResponseTranscript(requestTranscript, response)
	response.Signature = encode(ed25519.Sign(panelIdentity, responseTranscript))
	session, err := deriveSession(shared, transcriptHash(requestTranscript, responseTranscript), sessionID, request.MachineID, true)
	if err != nil {
		return HandshakeResponse{}, nil, err
	}
	return response, session, nil
}

func RequestTranscript(request HandshakeRequest) []byte {
	var transcript []byte
	transcript = appendField(transcript, "ca2")
	transcript = appendField(transcript, request.Version)
	transcript = appendField(transcript, request.Mode)
	transcript = appendField(transcript, request.MachineID)
	transcript = appendField(transcript, request.IdentityPublic)
	transcript = appendField(transcript, request.EphemeralPublic)
	transcript = appendField(transcript, request.Nonce)
	transcript = appendInt64(transcript, request.Timestamp)
	return transcript
}

func ResponseTranscript(requestTranscript []byte, response HandshakeResponse) []byte {
	transcript := append([]byte(nil), requestTranscript...)
	transcript = appendField(transcript, response.Version)
	transcript = appendField(transcript, response.SessionID)
	transcript = appendField(transcript, response.IdentityPublic)
	transcript = appendField(transcript, response.EphemeralPublic)
	transcript = appendField(transcript, response.Nonce)
	transcript = appendInt64(transcript, response.Timestamp)
	if response.Enrolled {
		transcript = append(transcript, 1)
	} else {
		transcript = append(transcript, 0)
	}
	return transcript
}

func ValidateHandshakeShape(request HandshakeRequest, now time.Time, maxSkew time.Duration) error {
	if request.Version != ProtocolVersion || (request.Mode != ModeEnroll && request.Mode != ModeConnect) || request.MachineID == "" {
		return errors.New("invalid handshake metadata")
	}
	if _, err := decodeSized(request.IdentityPublic, ed25519.PublicKeySize); err != nil {
		return errors.New("invalid Agent identity public key")
	}
	if _, err := decodeSized(request.EphemeralPublic, 32); err != nil {
		return errors.New("invalid Agent ephemeral public key")
	}
	if _, err := decodeSized(request.Nonce, 32); err != nil {
		return errors.New("invalid Agent nonce")
	}
	requestTime := time.Unix(request.Timestamp, 0)
	if requestTime.Before(now.Add(-maxSkew)) || requestTime.After(now.Add(maxSkew)) {
		return errors.New("handshake timestamp is outside the accepted window")
	}
	return nil
}

func EncodeHandshake(value any) ([]byte, error) {
	return json.Marshal(value)
}

func DecodeHandshake(data []byte, target any) error {
	if len(data) == 0 || len(data) > 16<<10 {
		return errors.New("invalid handshake size")
	}
	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("decode handshake: %w", err)
	}
	return nil
}

type Session struct {
	machineID     string
	sessionID     [sessionIDSize]byte
	sendAEAD      cipher.AEAD
	recvAEAD      cipher.AEAD
	sendNonce     [nonceSize]byte
	recvNonce     [nonceSize]byte
	sendDirection byte
	recvDirection byte
	sendSeq       atomic.Uint64
	recvMu        sync.Mutex
	recvWindow    replayWindow
}

func (s *Session) MachineID() string { return s.machineID }
func (s *Session) ID() string        { return encode(s.sessionID[:]) }

func (s *Session) Seal(method, path string, plaintext []byte) ([]byte, error) {
	seq := s.sendSeq.Add(1)
	if seq == 0 {
		return nil, errors.New("encrypted session sequence exhausted")
	}
	nonce := makeNonce(s.sendNonce, seq)
	aad := associatedData(s.sessionID, s.machineID, method, path, s.sendDirection, seq)
	ciphertext := s.sendAEAD.Seal(nil, nonce[:], plaintext, aad)
	envelope := make([]byte, headerSize+len(ciphertext))
	envelope[0] = EnvelopeVersion
	copy(envelope[1:1+sessionIDSize], s.sessionID[:])
	binary.BigEndian.PutUint64(envelope[1+sessionIDSize:headerSize], seq)
	copy(envelope[headerSize:], ciphertext)
	return envelope, nil
}

func (s *Session) Open(method, path string, envelope []byte) ([]byte, error) {
	if len(envelope) < headerSize+tagSize || envelope[0] != EnvelopeVersion {
		return nil, ErrInvalidEnvelope
	}
	if !hmac.Equal(envelope[1:1+sessionIDSize], s.sessionID[:]) {
		return nil, ErrWrongSession
	}
	seq := binary.BigEndian.Uint64(envelope[1+sessionIDSize : headerSize])
	if seq == 0 {
		return nil, ErrInvalidEnvelope
	}

	s.recvMu.Lock()
	defer s.recvMu.Unlock()
	candidate := s.recvWindow
	if !candidate.accept(seq) {
		return nil, ErrReplay
	}
	nonce := makeNonce(s.recvNonce, seq)
	aad := associatedData(s.sessionID, s.machineID, method, path, s.recvDirection, seq)
	plaintext, err := s.recvAEAD.Open(nil, nonce[:], envelope[headerSize:], aad)
	if err != nil {
		return nil, ErrInvalidEnvelope
	}
	s.recvWindow = candidate
	return plaintext, nil
}

func SessionIDFromEnvelope(envelope []byte) (string, error) {
	if len(envelope) < headerSize+tagSize || envelope[0] != EnvelopeVersion {
		return "", ErrInvalidEnvelope
	}
	return encode(envelope[1 : 1+sessionIDSize]), nil
}

type replayWindow struct {
	maxSeq uint64
	bitmap uint64
}

func (w *replayWindow) accept(seq uint64) bool {
	if seq == 0 {
		return false
	}
	if seq > w.maxSeq {
		shift := seq - w.maxSeq
		if shift >= replayBits {
			w.bitmap = 0
		} else {
			w.bitmap <<= shift
		}
		w.maxSeq = seq
		w.bitmap |= 1
		return true
	}
	difference := w.maxSeq - seq
	if difference >= replayBits {
		return false
	}
	bit := uint64(1) << difference
	if w.bitmap&bit != 0 {
		return false
	}
	w.bitmap |= bit
	return true
}

func deriveSession(shared, salt, sessionID []byte, machineID string, server bool) (*Session, error) {
	if len(sessionID) != sessionIDSize || machineID == "" {
		return nil, errors.New("invalid encrypted session binding")
	}
	reader := hkdf.New(sha256.New, shared, salt, []byte("ca2"))
	material := make([]byte, 32+32+nonceSize+nonceSize)
	if _, err := io.ReadFull(reader, material); err != nil {
		return nil, fmt.Errorf("derive encrypted session: %w", err)
	}
	c2sKey := material[:32]
	s2cKey := material[32:64]
	c2sNonce := material[64 : 64+nonceSize]
	s2cNonce := material[64+nonceSize:]

	newAEAD := func(key []byte) (cipher.AEAD, error) {
		block, err := aes.NewCipher(key)
		if err != nil {
			return nil, err
		}
		return cipher.NewGCM(block)
	}
	c2sAEAD, err := newAEAD(c2sKey)
	if err != nil {
		return nil, err
	}
	s2cAEAD, err := newAEAD(s2cKey)
	if err != nil {
		return nil, err
	}
	session := &Session{machineID: machineID}
	copy(session.sessionID[:], sessionID)
	if server {
		session.sendAEAD, session.recvAEAD = s2cAEAD, c2sAEAD
		copy(session.sendNonce[:], s2cNonce)
		copy(session.recvNonce[:], c2sNonce)
		session.sendDirection, session.recvDirection = 1, 0
	} else {
		session.sendAEAD, session.recvAEAD = c2sAEAD, s2cAEAD
		copy(session.sendNonce[:], c2sNonce)
		copy(session.recvNonce[:], s2cNonce)
		session.sendDirection, session.recvDirection = 0, 1
	}
	return session, nil
}

func associatedData(sessionID [sessionIDSize]byte, machineID, method, path string, direction byte, seq uint64) []byte {
	data := []byte{EnvelopeVersion, direction}
	data = append(data, sessionID[:]...)
	data = appendField(data, machineID)
	data = appendField(data, method)
	data = appendField(data, path)
	return appendUint64(data, seq)
}

func transcriptHash(request, response []byte) []byte {
	hash := sha256.New()
	hash.Write(request)
	hash.Write(response)
	return hash.Sum(nil)
}

func hmacSHA256(key, data []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return mac.Sum(nil)
}

func appendField(target []byte, value string) []byte {
	target = appendUint64(target, uint64(len(value)))
	return append(target, value...)
}

func appendInt64(target []byte, value int64) []byte {
	return appendUint64(target, uint64(value))
}

func appendUint64(target []byte, value uint64) []byte {
	var encoded [8]byte
	binary.BigEndian.PutUint64(encoded[:], value)
	return append(target, encoded[:]...)
}

func makeNonce(base [nonceSize]byte, seq uint64) [nonceSize]byte {
	nonce := base
	var encoded [8]byte
	binary.BigEndian.PutUint64(encoded[:], seq)
	for index := range encoded {
		nonce[nonceSize-len(encoded)+index] ^= encoded[index]
	}
	return nonce
}

func encode(value []byte) string {
	return base64.RawURLEncoding.EncodeToString(value)
}

func decodeSized(value string, size int) ([]byte, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || len(decoded) != size {
		return nil, errors.New("invalid encoded value")
	}
	return decoded, nil
}
