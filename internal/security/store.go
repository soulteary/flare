// Package security persists only administrator credentials and revocable sessions.
package security

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	_ "embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
	"golang.org/x/sys/unix"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidSession     = errors.New("invalid session")
	ErrReauthRequired     = errors.New("recent password verification required")
	ErrDeploymentManaged  = errors.New("password is managed by deployment configuration")
	ErrFaulted            = errors.New("security state requires recovery")
	ErrLocked             = errors.New("security state is locked by another process")
	ErrBusy               = errors.New("password verification capacity exceeded")
	ErrPasswordPolicy     = errors.New("password does not meet policy")
)

const ModeTrusted = "trusted"

const (
	IdleTimeout     = 30 * time.Minute
	AbsoluteTimeout = 12 * time.Hour
	ReauthTimeout   = 10 * time.Minute
)

type passwordHash struct {
	Salt    []byte
	Hash    []byte
	Memory  uint32
	Time    uint32
	Threads uint8
}
type Session struct {
	CreatedAt  time.Time
	LastActive time.Time
	ExpiresAt  time.Time
	VerifiedAt time.Time
	Generation uint64
}

func (s Session) Recent() bool { return time.Since(s.VerifiedAt) <= ReauthTimeout }

type Info struct {
	Initialized bool
	Username    string
	Mode        string
	InstanceID  string
	Generation  uint64
}
type state struct {
	Version    int
	InstanceID string
	Username   string
	Mode       string
	Password   passwordHash
	Generation uint64
	HighWater  time.Time
	Sessions   map[string]Session
}
type envelope struct {
	Checksum string
	State    json.RawMessage
}
type Store struct {
	mu      sync.Mutex
	dir     string
	lock    *os.File
	data    state
	now     func() time.Time
	faulted bool
	write   func(string, []byte) error
}

var authenticationSlots = make(chan struct{}, 9)
var hashSlot = make(chan struct{}, 1)
var hashWaiters = make(chan struct{}, 8)

func derive(password string, p passwordHash) ([]byte, error) {
	select {
	case hashWaiters <- struct{}{}:
	default:
		return nil, ErrBusy
	}
	defer func() { <-hashWaiters }()
	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()
	select {
	case hashSlot <- struct{}{}:
	case <-timer.C:
		return nil, ErrBusy
	}
	defer func() { <-hashSlot }()
	return argon2.IDKey([]byte(password), p.Salt, p.Time, p.Memory, p.Threads, 32), nil
}
func newHash(password string) (passwordHash, error) {
	p := passwordHash{Salt: make([]byte, 16), Memory: 19456, Time: 2, Threads: 1}
	if _, e := rand.Read(p.Salt); e != nil {
		return p, e
	}
	var e error
	p.Hash, e = derive(password, p)
	return p, e
}
func validHash(p passwordHash) bool {
	return len(p.Salt) == 16 && len(p.Hash) == 32 && p.Memory == 19456 && p.Time == 2 && p.Threads == 1
}
func randomToken() (string, error) {
	b := make([]byte, 32)
	_, e := rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b), e
}
func digest(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }
func Open(dir string) (*Store, error) {
	if info, e := os.Lstat(dir); e == nil && (!info.IsDir() || info.Mode()&os.ModeSymlink != 0) {
		return nil, ErrFaulted
	} else if e != nil && !os.IsNotExist(e) {
		return nil, e
	}
	if e := os.MkdirAll(dir, 0700); e != nil {
		return nil, e
	}
	// #nosec G302 -- This is a private directory; owner traversal requires the execute bit.
	if e := os.Chmod(dir, 0700); e != nil {
		return nil, e
	}
	// #nosec G304 -- Operator-selected state directory and a fixed filename; reject symlinks and non-regular files below.
	f, e := os.OpenFile(filepath.Join(dir, "instance.lock"), os.O_CREATE|os.O_RDWR|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0600)
	if e != nil {
		return nil, e
	}
	if info, statErr := f.Stat(); statErr != nil || !info.Mode().IsRegular() {
		return nil, errors.Join(ErrFaulted, f.Close())
	}
	if e = unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); e != nil {
		return nil, errors.Join(ErrLocked, f.Close())
	}
	s := &Store{dir: dir, lock: f, now: time.Now}
	s.write = s.atomicWrite
	fail := func() (*Store, error) { return nil, errors.Join(ErrFaulted, s.Close()) }
	marker, me := readStateFile(filepath.Join(dir, "complete"))
	raw, re := readStateFile(filepath.Join(dir, "state.json"))
	journal, je := readStateFile(filepath.Join(dir, "commit.json"))
	// A durable journal is the transaction's commit intent; replay is idempotent.
	if je == nil {
		var pending envelope
		if json.Unmarshal(journal, &pending) != nil || digest(string(pending.State)) != pending.Checksum {
			return fail()
		}
		var next state
		if json.Unmarshal(pending.State, &next) != nil || next.Version != 1 || next.InstanceID == "" {
			return fail()
		}
		if me == nil && string(marker) != next.InstanceID {
			return fail()
		}
		if e = s.atomicWrite("state.json", journal); e != nil {
			return fail()
		}
		if e = s.atomicWrite("complete", []byte(next.InstanceID)); e != nil {
			return fail()
		}
		if e = os.Remove(filepath.Join(dir, "commit.json")); e != nil {
			return fail()
		}
		raw = journal
		re = nil
		marker = []byte(next.InstanceID)
		me = nil
	} else if !os.IsNotExist(je) {
		return fail()
	}
	if os.IsNotExist(me) && os.IsNotExist(re) {
		s.data = state{Version: 1, Sessions: map[string]Session{}}
		return s, nil
	}
	if me != nil || re != nil {
		return fail()
	}
	var env envelope
	if json.Unmarshal(raw, &env) != nil || digest(string(env.State)) != env.Checksum {
		return fail()
	}
	if json.Unmarshal(env.State, &s.data) != nil || s.data.Version != 1 || s.data.InstanceID != string(marker) || s.data.InstanceID == "" || s.data.Sessions == nil {
		return fail()
	}
	if s.data.Mode != ModeTrusted && (!validHash(s.data.Password) || (s.data.Mode != "local" && s.data.Mode != "deployment")) {
		return fail()
	}
	if _, e = s.clockLocked(); e != nil {
		return fail()
	}
	return s, nil
}
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lock == nil {
		return nil
	}
	e := s.lock.Close()
	s.lock = nil
	return e
}
func (s *Store) atomicWrite(name string, b []byte) error {
	f, e := os.CreateTemp(s.dir, ".pending-")
	if e != nil {
		return e
	}
	defer os.Remove(f.Name())
	if e = f.Chmod(0600); e == nil {
		_, e = f.Write(b)
	}
	if e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e == nil {
		e = ce
	}
	if e == nil {
		e = os.Rename(f.Name(), filepath.Join(s.dir, name))
	}
	if e == nil {
		var d *os.File
		d, e = os.Open(s.dir)
		if e == nil {
			e = errors.Join(d.Sync(), d.Close())
		}
	}
	return e
}
func (s *Store) commit(next state) error {
	if s.faulted || s.lock == nil {
		return ErrFaulted
	}
	raw, e := json.Marshal(next)
	if e != nil {
		return e
	}
	b, e := json.Marshal(envelope{digest(string(raw)), raw})
	if e != nil {
		return e
	}
	items := []struct {
		n string
		b []byte
	}{{"commit.json", b}, {"state.json", b}}
	if s.data.InstanceID == "" {
		items = append(items, struct {
			n string
			b []byte
		}{"complete", []byte(next.InstanceID)})
	} else {
		// Instance identity is immutable after initialization. Validate the
		// already-durable marker rather than atomically rewriting it for every
		// session read/touch. Missing or altered markers fault before a new
		// commit intent is published; Open still recreates it during journal replay.
		marker, err := readStateFile(filepath.Join(s.dir, "complete"))
		if err != nil || string(marker) != s.data.InstanceID || next.InstanceID != s.data.InstanceID {
			s.faulted = true
			return fmt.Errorf("%w: instance marker changed", ErrFaulted)
		}
	}
	for _, item := range items {
		if e = s.write(item.n, item.b); e != nil {
			s.faulted = true
			return fmt.Errorf("%w: transaction persistence failed", ErrFaulted)
		}
	}
	if e = os.Remove(filepath.Join(s.dir, "commit.json")); e != nil {
		s.faulted = true
		return ErrFaulted
	}
	d, e := os.Open(s.dir)
	if e == nil {
		e = errors.Join(d.Sync(), d.Close())
	}
	if e != nil {
		s.faulted = true
		return ErrFaulted
	}
	s.data = next
	return nil
}
func (s *Store) clone() state {
	n := s.data
	n.Sessions = make(map[string]Session, len(s.data.Sessions))
	for k, v := range s.data.Sessions {
		n.Sessions[k] = v
	}
	return n
}
func (s *Store) clockLocked() (time.Time, error) {
	if s.faulted || s.lock == nil {
		return time.Time{}, ErrFaulted
	}
	now := s.now().UTC()
	if now.Before(s.data.HighWater.Add(-60 * time.Second)) {
		n := s.clone()
		n.Generation++
		n.Sessions = map[string]Session{}
		n.HighWater = now
		return now, s.commit(n)
	}
	if now.Before(s.data.HighWater) {
		now = s.data.HighWater
	}
	return now, nil
}
func (s *Store) Info() Info {
	s.mu.Lock()
	defer s.mu.Unlock()
	return Info{s.data.InstanceID != "", s.data.Username, s.data.Mode, s.data.InstanceID, s.data.Generation}
}

//go:embed common-passwords.txt
var weakPasswordResource string
var weakPasswords = func() map[string]bool {
	result := map[string]bool{}
	for _, line := range strings.Split(weakPasswordResource, "\n") {
		if line != "" && !strings.HasPrefix(line, "#") {
			result[strings.ToLower(line)] = true
		}
	}
	return result
}()

func ValidatePassword(p string) error {
	if !utf8.ValidString(p) || utf8.RuneCountInString(p) < 15 || utf8.RuneCountInString(p) > 128 || len(p) > 512 {
		return ErrPasswordPolicy
	}
	if weakPasswords[strings.ToLower(p)] {
		return ErrPasswordPolicy
	}
	// Repeating very short units is a predictable pattern regardless of length.
	runes := []rune(p)
	for size := 1; size <= 4; size++ {
		if len(runes)%size != 0 {
			continue
		}
		repeated := true
		for i := size; i < len(runes); i++ {
			if runes[i] != runes[i%size] {
				repeated = false
				break
			}
		}
		if repeated {
			return ErrPasswordPolicy
		}
	}
	return nil
}
func validateCredentials(u, p, m string) error {
	if u == "" || len(u) > 256 || len(p) == 0 || len(u)+len(p) > 8192 || (m != "local" && m != "deployment") {
		return errors.New("invalid credential input")
	}
	return nil
}
func (s *Store) Initialize(u, p, m string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data.InstanceID != "" {
		return errors.New("security state already initialized")
	}
	if m == ModeTrusted {
		if u != "" || p != "" {
			return errors.New("trusted mode has no administrator")
		}
		id, e := randomToken()
		if e != nil {
			return e
		}
		return s.commit(state{Version: 1, InstanceID: id, Mode: m, Generation: 1, HighWater: s.now().UTC(), Sessions: map[string]Session{}})
	}
	if e := validateCredentials(u, p, m); e != nil {
		return e
	}
	h, e := newHash(p)
	if e != nil {
		return e
	}
	id, e := randomToken()
	if e != nil {
		return e
	}
	return s.commit(state{Version: 1, InstanceID: id, Username: u, Mode: m, Password: h, Generation: 1, HighWater: s.now().UTC(), Sessions: map[string]Session{}})
}
func (s *Store) verifyLocked(u, p string) error {
	if s.faulted || s.lock == nil {
		return ErrFaulted
	}
	h := s.data.Password
	if !validHash(h) {
		h = passwordHash{Salt: make([]byte, 16), Hash: make([]byte, 32), Memory: 19456, Time: 2, Threads: 1}
	}
	got, e := derive(p, h)
	if e != nil {
		return e
	}
	match := subtle.ConstantTimeCompare(got, h.Hash)
	if match != 1 || u != s.data.Username || u == "" {
		return ErrInvalidCredentials
	}
	return nil
}
func (s *Store) Verify(u, p string) error {
	s.mu.Lock()
	if s.faulted || s.lock == nil {
		s.mu.Unlock()
		return ErrFaulted
	}
	h, generation, username := s.data.Password, s.data.Generation, s.data.Username
	s.mu.Unlock()
	if !validHash(h) {
		h = passwordHash{Salt: make([]byte, 16), Hash: make([]byte, 32), Memory: 19456, Time: 2, Threads: 1}
	}
	got, e := derive(p, h)
	if e != nil {
		return e
	}
	match := subtle.ConstantTimeCompare(got, h.Hash)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.faulted || s.lock == nil {
		return ErrFaulted
	}
	if match != 1 || u != username || u == "" || generation != s.data.Generation {
		return ErrInvalidCredentials
	}
	return nil
}

// Authenticate verifies credentials and creates a session under the same state lock.
func (s *Store) Authenticate(u, p string) (string, error) {
	// Admission precedes the state lock so authentication cannot accumulate an unbounded queue.
	select {
	case authenticationSlots <- struct{}{}:
	default:
		return "", ErrBusy
	}
	defer func() { <-authenticationSlots }()
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.verifyLocked(u, p); e != nil {
		return "", e
	}
	return s.createSessionLocked()
}
func (s *Store) CreateSession() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.createSessionLocked()
}
func (s *Store) createSessionLocked() (string, error) {
	if s.data.Username == "" {
		return "", ErrInvalidCredentials
	}
	now, e := s.clockLocked()
	if e != nil {
		return "", e
	}
	token, e := randomToken()
	if e != nil {
		return "", e
	}
	n := s.clone()
	for k, v := range n.Sessions {
		if !now.Before(v.ExpiresAt) || now.Sub(v.LastActive) >= IdleTimeout {
			delete(n.Sessions, k)
		}
	}
	if len(n.Sessions) >= 4096 {
		return "", ErrBusy
	}
	n.Sessions[digest(token)] = Session{now, now, now.Add(AbsoluteTimeout), now, n.Generation}
	n.HighWater = now
	return token, s.commit(n)
}
func (s *Store) sessionLocked(token string, touch bool) (Session, error) {
	now, e := s.clockLocked()
	if e != nil {
		return Session{}, e
	}
	v, ok := s.data.Sessions[digest(token)]
	if !ok || v.Generation != s.data.Generation || !now.Before(v.ExpiresAt) || now.Sub(v.LastActive) >= IdleTimeout {
		if ok {
			n := s.clone()
			delete(n.Sessions, digest(token))
			n.HighWater = now
			if e = s.commit(n); e != nil {
				return Session{}, e
			}
		}
		return Session{}, ErrInvalidSession
	}
	if touch || now.After(s.data.HighWater) {
		n := s.clone()
		if touch {
			v.LastActive = now
			n.Sessions[digest(token)] = v
		}
		n.HighWater = now
		if e = s.commit(n); e != nil {
			return Session{}, e
		}
	}
	return v, nil
}
func (s *Store) Session(token string, touch bool) (Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sessionLocked(token, touch)
}

// SessionProof belongs to one HTTP request and one Store. It is not a reusable
// authentication credential. Callers must obtain a new proof for every request.
// Ordinary proofs have a durable SessionSnapshot authorization point and a
// bounded monotonic lifetime. Tentative buffered-read proofs have no new clock
// observation and require a final durable Session decision before any output.
type SessionProof struct {
	owner      *Store
	token      string
	generation uint64
	started    time.Time
	tentative  bool
}

const sessionProofLifetime = time.Second

func (s *Store) SessionSnapshot(token string, touch bool) (Session, *SessionProof, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, err := s.sessionLocked(token, touch)
	if err != nil && !errors.Is(err, ErrInvalidSession) {
		return value, nil, err
	}
	return value, &SessionProof{owner: s, token: token, generation: s.data.Generation, started: time.Now()}, err
}

// BeginBufferedRead creates a tentative identity using only already-durable
// time and live store state. It does not observe a new calendar time or grant
// permission to release output. Only explicitly whitelisted, buffered views
// without external side effects may use it. Before releasing ANY outcome the
// caller must call Session with a fresh clock observation; only a successful
// foreground GET may touch idle activity. Writes, exports and online weather
// views must keep the ordinary durable entry authorization.
// Invalid identities have no tentative proof: callers retain their ordinary
// durable anonymous/error path. A tentative proof is request-local and never a
// reusable authentication credential.
func (s *Store) BeginBufferedRead(token string) (Session, *SessionProof, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.faulted || s.lock == nil {
		return Session{}, nil, ErrFaulted
	}
	value, ok := s.data.Sessions[digest(token)]
	now := s.data.HighWater
	if !ok || value.Generation != s.data.Generation || !now.Before(value.ExpiresAt) || now.Sub(value.LastActive) >= IdleTimeout {
		return Session{}, nil, ErrInvalidSession
	}
	return value, &SessionProof{owner: s, token: token, generation: s.data.Generation, tentative: true}, nil
}

// GuardSession rechecks revocation, expiry and store health for a request's
// durable authorization point without another disk transaction. It never reads
// the wall clock. Once its monotonic lifetime is exhausted, it observes the wall
// clock through the ordinary durable session path before refreshing the proof.
// Tentative buffered-read proofs never sample the clock or perform this refresh;
// they require the caller's final durable Session decision before any output.
// Writes and exports must continue to use fresh Session/WithAuthorizedWrite.
func (s *Store) GuardSession(proof *SessionProof) (Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.faulted || s.lock == nil {
		return Session{}, ErrFaulted
	}
	if proof == nil || proof.owner != s {
		return Session{}, ErrInvalidSession
	}
	if !proof.tentative && time.Since(proof.started) >= sessionProofLifetime {
		value, err := s.sessionLocked(proof.token, false)
		if err == nil || errors.Is(err, ErrInvalidSession) {
			proof.started = time.Now()
			proof.generation = s.data.Generation
		}
		return value, err
	}
	value, ok := s.data.Sessions[digest(proof.token)]
	if !ok || proof.generation != s.data.Generation || value.Generation != s.data.Generation {
		return Session{}, ErrInvalidSession
	}
	now := s.data.HighWater
	if !now.Before(value.ExpiresAt) || now.Sub(value.LastActive) >= IdleTimeout {
		if proof.tentative {
			return Session{}, ErrInvalidSession
		}
		next := s.clone()
		delete(next.Sessions, digest(proof.token))
		if err := s.commit(next); err != nil {
			return Session{}, err
		}
		return Session{}, ErrInvalidSession
	}
	return value, nil
}
func (s *Store) Reauthenticate(token, p string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.sessionLocked(token, false)
	if e != nil {
		return "", e
	}
	if e = s.verifyLocked(s.data.Username, p); e != nil {
		return "", e
	}
	now, e := s.clockLocked()
	if e != nil {
		return "", e
	}
	t, e := randomToken()
	if e != nil {
		return "", e
	}
	n := s.clone()
	delete(n.Sessions, digest(token))
	v.VerifiedAt = now
	v.LastActive = now
	n.Sessions[digest(t)] = v
	n.HighWater = now
	return t, s.commit(n)
}
func (s *Store) Revoke(token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := s.clone()
	delete(n.Sessions, digest(token))
	return s.commit(n)
}
func (s *Store) RevokeAll() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := s.clone()
	n.Generation++
	n.Sessions = map[string]Session{}
	return s.commit(n)
}
func (s *Store) credentialsLocked(u, p, m string) error {
	if e := validateCredentials(u, p, m); e != nil {
		return e
	}
	h, e := newHash(p)
	if e != nil {
		return e
	}
	n := s.clone()
	n.Username = u
	n.Password = h
	n.Mode = m
	n.Generation++
	n.Sessions = map[string]Session{}
	return s.commit(n)
}
func (s *Store) SetCredentials(u, p, m string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data.InstanceID == "" {
		return errors.New("initialize security state first")
	}
	if u == s.data.Username && m == s.data.Mode && s.verifyLocked(u, p) == nil {
		return nil
	}
	return s.credentialsLocked(u, p, m)
}
func (s *Store) ChangePassword(current, next string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data.Mode != "local" {
		return ErrDeploymentManaged
	}
	if e := s.verifyLocked(s.data.Username, current); e != nil {
		return e
	}
	if e := ValidatePassword(next); e != nil {
		return e
	}
	return s.credentialsLocked(s.data.Username, next, "local")
}
func (s *Store) ResetPassword(p string, takeLocal bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data.Mode == "deployment" && !takeLocal {
		return ErrDeploymentManaged
	}
	if e := ValidatePassword(p); e != nil {
		return e
	}
	return s.credentialsLocked(s.data.Username, p, "local")
}

// ChangePasswordAndCreateSession commits password replacement, all revocations and
// the replacement current session in one durable transaction.
func (s *Store) ChangePasswordAndCreateSession(token, current, next string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data.Mode != "local" {
		return "", ErrDeploymentManaged
	}
	if e := ValidatePassword(next); e != nil {
		return "", e
	}
	return s.replaceCredentialsAndSessionLocked(token, current, next)
}

// TakeLocalControl requires the active administrator's current password.
func (s *Store) TakeLocalControl(token, current string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data.Mode != "deployment" {
		return "", errors.New("credentials are already locally managed")
	}
	return s.replaceCredentialsAndSessionLocked(token, current, current)
}
func (s *Store) replaceCredentialsAndSessionLocked(token, current, next string) (string, error) {
	if _, e := s.sessionLocked(token, false); e != nil {
		return "", e
	}
	if e := s.verifyLocked(s.data.Username, current); e != nil {
		return "", e
	}
	h, e := newHash(next)
	if e != nil {
		return "", e
	}
	now, e := s.clockLocked()
	if e != nil {
		return "", e
	}
	replacement, e := randomToken()
	if e != nil {
		return "", e
	}
	n := s.clone()
	n.Password = h
	n.Mode = "local"
	n.Generation++
	n.HighWater = now
	n.Sessions = map[string]Session{digest(replacement): {CreatedAt: now, LastActive: now, ExpiresAt: now.Add(AbsoluteTimeout), VerifiedAt: now, Generation: n.Generation}}
	return replacement, s.commit(n)
}

func readStateFile(path string) ([]byte, error) {
	// #nosec G304 -- Internal state filenames in the operator-selected directory; NOFOLLOW and regular-file/size checks constrain reads.
	f, e := os.OpenFile(path, os.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	info, e := f.Stat()
	if e != nil {
		return nil, e
	}
	if !info.Mode().IsRegular() || info.Size() > 8<<20 {
		return nil, ErrFaulted
	}
	return io.ReadAll(io.LimitReader(f, 8<<20))
}

// WithAuthorizedWrite orders a YAML commit against session revocation. The
// callback must not call Store methods: the security lock remains held until the
// content transaction finishes. Revocation either commits first (write denied)
// or waits for the already-authorized write to finish.
func (s *Store) WithAuthorizedWrite(token string, trusted bool, action func() error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.faulted || s.lock == nil {
		return ErrFaulted
	}
	if !trusted {
		if _, e := s.sessionLocked(token, false); e != nil {
			return e
		}
	}
	return action()
}

// CheckWritable probes the configured local store rather than guessing mount
// persistence. It does not verify that a container will retain the directory.
func (s *Store) CheckWritable() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.faulted || s.lock == nil {
		return ErrFaulted
	}
	f, e := os.CreateTemp(s.dir, ".write-probe-")
	if e != nil {
		s.faulted = true
		return ErrFaulted
	}
	name := f.Name()
	defer os.Remove(name)
	e = f.Sync()
	closeErr := f.Close()
	if e == nil {
		e = closeErr
	}
	if e == nil {
		e = os.Remove(name)
	}
	if e != nil {
		s.faulted = true
		return ErrFaulted
	}
	return nil
}
