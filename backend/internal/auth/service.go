package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
)

type NonceChallenge struct {
	Address   string    `json:"address"`
	ChainID   string    `json:"chainId"`
	Message   string    `json:"message"`
	Nonce     string    `json:"nonce"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type Session struct {
	Token     string    `json:"token"`
	Address   string    `json:"address"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type nonceRecord struct {
	address   string
	message   string
	expiresAt time.Time
	used      bool
}

type sessionRecord struct {
	address   string
	expiresAt time.Time
}

type Service struct {
	mu       sync.Mutex
	nonces   map[string]nonceRecord // key: lower(address)
	ttl      time.Duration
	secret   []byte
}

func NewService() *Service {
	secret := strings.TrimSpace(os.Getenv("AUTH_SECRET"))
	if secret == "" {
		// Ephemeral secret; tokens will not survive server restart. Users should set AUTH_SECRET in .env.
		secret = randHex(32)
	}
	return &Service{
		nonces:   make(map[string]nonceRecord),
		ttl:      15 * time.Minute,
		secret:   []byte(secret),
	}
}

func (s *Service) cleanupExpiredNoncesLocked(now time.Time) {
	for k, v := range s.nonces {
		if v.used || now.After(v.expiresAt) {
			delete(s.nonces, k)
		}
	}
}

func (s *Service) NewChallenge(address string, chainID string, origin string) (NonceChallenge, error) {
	addr := strings.ToLower(strings.TrimSpace(address))
	if !common.IsHexAddress(addr) {
		return NonceChallenge{}, errors.New("invalid address")
	}
	if strings.TrimSpace(chainID) == "" {
		return NonceChallenge{}, errors.New("missing chainId")
	}
	nonce := randHex(16)
	now := time.Now().UTC()
	expires := now.Add(10 * time.Minute)

	msg := buildSIWEMessage(origin, addr, chainID, nonce, now, expires)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupExpiredNoncesLocked(now)
	s.nonces[addr] = nonceRecord{
		address:   addr,
		message:   msg,
		expiresAt: expires,
		used:      false,
	}

	return NonceChallenge{
		Address:   addr,
		ChainID:   chainID,
		Message:   msg,
		Nonce:     nonce,
		ExpiresAt: expires,
	}, nil
}

func (s *Service) Verify(ctx context.Context, address, message, signature string) (Session, error) {
	_ = ctx

	addr := strings.ToLower(strings.TrimSpace(address))
	if !common.IsHexAddress(addr) {
		return Session{}, errors.New("invalid address")
	}
	if strings.TrimSpace(message) == "" {
		return Session{}, errors.New("missing message")
	}
	if strings.TrimSpace(signature) == "" {
		return Session{}, errors.New("missing signature")
	}

	s.mu.Lock()
	now := time.Now()
	s.cleanupExpiredNoncesLocked(now)
	rec, ok := s.nonces[addr]
	s.mu.Unlock()
	if !ok {
		return Session{}, errors.New("no nonce challenge for address (please login again)")
	}
	if rec.used {
		return Session{}, errors.New("nonce already used (please login again)")
	}
	if now.After(rec.expiresAt) {
		s.mu.Lock()
		delete(s.nonces, addr)
		s.mu.Unlock()
		return Session{}, errors.New("nonce expired (please login again)")
	}
	if rec.message != message {
		return Session{}, errors.New("message mismatch (please login again)")
	}

	sig, err := hexutil.Decode(signature)
	if err != nil {
		return Session{}, errors.New("invalid signature hex")
	}
	if len(sig) != 65 {
		return Session{}, errors.New("invalid signature length")
	}
	// MetaMask may return v as 27/28.
	if sig[64] >= 27 {
		sig[64] -= 27
	}

	hash := accounts.TextHash([]byte(message))
	pub, err := crypto.SigToPub(hash, sig)
	if err != nil {
		return Session{}, errors.New("failed to recover public key from signature")
	}
	recovered := strings.ToLower(crypto.PubkeyToAddress(*pub).Hex())
	if recovered != addr {
		return Session{}, fmt.Errorf("signature does not match address (recovered %s)", recovered)
	}

	expires := time.Now().Add(s.ttl)

	s.mu.Lock()
	delete(s.nonces, addr)
	s.mu.Unlock()

	tok, err := s.mintToken(addr, expires)
	if err != nil {
		return Session{}, err
	}
	return Session{Token: tok, Address: addr, ExpiresAt: expires}, nil
}

func (s *Service) Authenticate(token string) (string, bool) {
	tok := strings.TrimSpace(token)
	if tok == "" {
		return "", false
	}
	addr, exp, ok := s.verifyToken(tok)
	if !ok {
		return "", false
	}
	if time.Now().After(time.Unix(exp, 0)) {
		return "", false
	}
	return addr, true
}

type tokenPayload struct {
	Address string `json:"address"`
	Exp     int64  `json:"exp"`
}

func (s *Service) mintToken(address string, expiresAt time.Time) (string, error) {
	p := tokenPayload{Address: strings.ToLower(strings.TrimSpace(address)), Exp: expiresAt.Unix()}
	b, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	payload := base64.RawURLEncoding.EncodeToString(b)
	mac := hmac.New(sha256.New, s.secret)
	_, _ = mac.Write([]byte(payload))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return payload + "." + sig, nil
}

func (s *Service) verifyToken(token string) (address string, exp int64, ok bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return "", 0, false
	}
	payload, sig := parts[0], parts[1]
	if payload == "" || sig == "" {
		return "", 0, false
	}
	mac := hmac.New(sha256.New, s.secret)
	_, _ = mac.Write([]byte(payload))
	expect := mac.Sum(nil)
	got, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil {
		return "", 0, false
	}
	if !hmac.Equal(expect, got) {
		return "", 0, false
	}
	raw, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return "", 0, false
	}
	var p tokenPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return "", 0, false
	}
	if !common.IsHexAddress(p.Address) {
		return "", 0, false
	}
	return strings.ToLower(p.Address), p.Exp, true
}

func buildSIWEMessage(origin, address, chainID, nonce string, issuedAt, expiresAt time.Time) string {
	origin = strings.TrimSpace(origin)
	if origin == "" {
		origin = "AI Bot Chain Studio"
	}

	return fmt.Sprintf(
		"%s wants you to sign in with your Ethereum account:\n%s\n\nURI: %s\nVersion: 1\nChain ID: %s\nNonce: %s\nIssued At: %s\nExpiration Time: %s",
		origin,
		address,
		origin,
		chainID,
		nonce,
		issuedAt.Format(time.RFC3339),
		expiresAt.Format(time.RFC3339),
	)
}

func randHex(nbytes int) string {
	b := make([]byte, nbytes)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
