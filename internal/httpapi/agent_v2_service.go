package httpapi

import (
	"context"
	"crypto/ed25519"
	"crypto/hmac"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"cpanel/internal/agentv2"
	"cpanel/internal/domain"
	"cpanel/internal/securebox"
	"cpanel/internal/store"
)

const (
	agentV2HandshakeSkew = 2 * time.Minute
	agentV2NonceTTL      = 5 * time.Minute
	agentV2SessionTTL    = 2 * time.Hour
)

type agentV2SessionRecord struct {
	session   *agentv2.Session
	machine   domain.AgentMachine
	expiresAt time.Time
}

type agentV2Service struct {
	store *store.Store
	box   *securebox.Box

	identityMu sync.Mutex
	identity   ed25519.PrivateKey

	sessionsMu sync.RWMutex
	sessions   map[string]agentV2SessionRecord

	noncesMu sync.Mutex
	nonces   map[string]time.Time
}

func newAgentV2Service(dataStore *store.Store, box *securebox.Box) *agentV2Service {
	return &agentV2Service{
		store:    dataStore,
		box:      box,
		sessions: make(map[string]agentV2SessionRecord),
		nonces:   make(map[string]time.Time),
	}
}

func (s *agentV2Service) panelPublicKey(ctx context.Context) (ed25519.PublicKey, error) {
	identity, err := s.panelIdentity(ctx)
	if err != nil {
		return nil, err
	}
	return identity.Public().(ed25519.PublicKey), nil
}

func (s *agentV2Service) panelIdentity(ctx context.Context) (ed25519.PrivateKey, error) {
	s.identityMu.Lock()
	defer s.identityMu.Unlock()
	if len(s.identity) == ed25519.PrivateKeySize {
		return s.identity, nil
	}

	stored, err := s.store.AgentPanelIdentity(ctx)
	if errors.Is(err, store.ErrNotFound) {
		publicKey, privateKey, generateErr := agentv2.GenerateIdentity()
		if generateErr != nil {
			return nil, generateErr
		}
		seedJSON, marshalErr := json.Marshal(base64.RawURLEncoding.EncodeToString(privateKey.Seed()))
		if marshalErr != nil {
			return nil, marshalErr
		}
		sealed, sealErr := s.box.Seal(seedJSON)
		if sealErr != nil {
			return nil, sealErr
		}
		if createErr := s.store.CreateAgentPanelIdentity(ctx, sealed, publicKey); createErr != nil {
			return nil, createErr
		}
		stored, err = s.store.AgentPanelIdentity(ctx)
	}
	if err != nil {
		return nil, err
	}
	plain, err := s.box.Open(stored.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("open panel identity: %w", err)
	}
	var encodedSeed string
	if err := json.Unmarshal(plain, &encodedSeed); err != nil {
		return nil, fmt.Errorf("decode panel identity seed: %w", err)
	}
	seed, err := base64.RawURLEncoding.DecodeString(encodedSeed)
	if err != nil || len(seed) != ed25519.SeedSize {
		return nil, errors.New("stored panel identity seed is invalid")
	}
	privateKey := ed25519.NewKeyFromSeed(seed)
	if !hmac.Equal(privateKey.Public().(ed25519.PublicKey), stored.PublicKey) {
		return nil, errors.New("stored panel identity public key does not match its private key")
	}
	s.identity = privateKey
	return s.identity, nil
}

func (s *agentV2Service) acceptHandshake(ctx context.Context, request agentv2.HandshakeRequest, now time.Time) (agentv2.HandshakeResponse, error) {
	if err := agentv2.ValidateHandshakeShape(request, now, agentV2HandshakeSkew); err != nil {
		return agentv2.HandshakeResponse{}, err
	}
	machine, err := s.store.AgentMachineByID(ctx, request.MachineID)
	if err != nil {
		return agentv2.HandshakeResponse{}, err
	}
	claimedIdentity, err := agentv2.IdentityPublic(request)
	if err != nil {
		return agentv2.HandshakeResponse{}, err
	}

	enrolled := false
	identity, identityErr := s.store.AgentIdentity(ctx, request.MachineID)
	switch {
	case identityErr == nil:
		if identity.Status != "active" || !agentv2.VerifyIdentity(request, ed25519.PublicKey(identity.PublicKey)) {
			return agentv2.HandshakeResponse{}, errors.New("Agent identity verification failed")
		}
		enrolled = true
	case !errors.Is(identityErr, store.ErrNotFound):
		return agentv2.HandshakeResponse{}, identityErr
	default:
		enrollmentKey, keyErr := s.enrollmentKey(ctx)
		if keyErr != nil || !agentv2.VerifyEnrollment(request, enrollmentKey) {
			return agentv2.HandshakeResponse{}, errors.New("Agent enrollment verification failed")
		}
		if _, err := s.store.EnrollAgentIdentity(ctx, request.MachineID, claimedIdentity); err != nil {
			return agentv2.HandshakeResponse{}, err
		}
		enrolled = true
	}
	if !s.reserveNonce(request.MachineID, request.Nonce, now) {
		return agentv2.HandshakeResponse{}, errors.New("Agent handshake was replayed")
	}
	panelIdentity, err := s.panelIdentity(ctx)
	if err != nil {
		return agentv2.HandshakeResponse{}, err
	}
	response, session, err := agentv2.NewServerHandshake(request, panelIdentity, enrolled, now)
	if err != nil {
		return agentv2.HandshakeResponse{}, err
	}
	s.sessionsMu.Lock()
	s.sessions[session.ID()] = agentV2SessionRecord{session: session, machine: machine, expiresAt: now.Add(agentV2SessionTTL)}
	s.sessionsMu.Unlock()
	if err := s.store.TouchAgentV2(ctx, request.MachineID); err != nil {
		s.deleteSession(session.ID())
		return agentv2.HandshakeResponse{}, err
	}
	return response, nil
}

func (s *agentV2Service) enrollmentKey(ctx context.Context) ([]byte, error) {
	sealed, err := s.store.RawSetting(ctx, "agent", "communication_key")
	if err != nil {
		return nil, err
	}
	plain, err := s.box.Open(sealed)
	if err != nil {
		return nil, err
	}
	var key string
	if err := json.Unmarshal(plain, &key); err != nil || len(key) < 32 {
		return nil, errors.New("Agent enrollment key is unavailable")
	}
	return []byte(key), nil
}

func (s *agentV2Service) sessionForEnvelope(envelope []byte, now time.Time) (agentV2SessionRecord, error) {
	sessionID, err := agentv2.SessionIDFromEnvelope(envelope)
	if err != nil {
		return agentV2SessionRecord{}, err
	}
	s.sessionsMu.RLock()
	record, ok := s.sessions[sessionID]
	s.sessionsMu.RUnlock()
	if !ok || now.After(record.expiresAt) {
		if ok {
			s.deleteSession(sessionID)
		}
		return agentV2SessionRecord{}, errors.New("encrypted Agent session is unavailable")
	}
	return record, nil
}

func (s *agentV2Service) sessionByID(sessionID string, now time.Time) (agentV2SessionRecord, error) {
	s.sessionsMu.RLock()
	record, ok := s.sessions[sessionID]
	s.sessionsMu.RUnlock()
	if !ok || now.After(record.expiresAt) {
		if ok {
			s.deleteSession(sessionID)
		}
		return agentV2SessionRecord{}, errors.New("encrypted Agent session is unavailable")
	}
	return record, nil
}

func (s *agentV2Service) deleteSession(sessionID string) {
	s.sessionsMu.Lock()
	delete(s.sessions, sessionID)
	s.sessionsMu.Unlock()
}

func (s *agentV2Service) deleteMachineSessions(machineID string) {
	s.sessionsMu.Lock()
	for sessionID, record := range s.sessions {
		if record.machine.ID == machineID {
			delete(s.sessions, sessionID)
		}
	}
	s.sessionsMu.Unlock()
}

func (s *agentV2Service) reserveNonce(machineID, nonce string, now time.Time) bool {
	s.noncesMu.Lock()
	defer s.noncesMu.Unlock()
	for key, expiresAt := range s.nonces {
		if now.After(expiresAt) {
			delete(s.nonces, key)
		}
	}
	key := machineID + ":" + nonce
	if expiresAt, exists := s.nonces[key]; exists && now.Before(expiresAt) {
		return false
	}
	s.nonces[key] = now.Add(agentV2NonceTTL)
	return true
}
