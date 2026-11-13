package security

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSessionLimitsAreThirtyDays(t *testing.T) {
	require.Equal(t, 30*24*time.Hour, AbsoluteTimeout)
	// Equal limits make the absolute deadline the effective one.
	require.Equal(t, AbsoluteTimeout, IdleTimeout)
	// Sensitive operations keep their short password-verification window.
	require.Equal(t, 10*time.Minute, ReauthTimeout)
}

// An unused session must not end before the 30-day limit, and must end at it.
func TestSessionLastsThirtyDaysWithoutActivity(t *testing.T) {
	s, _ := fixture(t)
	now := s.data.HighWater
	s.now = func() time.Time { return now }
	token, err := s.CreateSession()
	require.NoError(t, err)
	start := now

	now = start.Add(AbsoluteTimeout - time.Second)
	session, err := s.Session(token, false)
	require.NoError(t, err, "session ended before the 30-day limit")
	require.True(t, session.ExpiresAt.Equal(start.Add(AbsoluteTimeout)))
	require.True(t, session.LastActive.Equal(start), "a check without activity must not renew the session")

	now = start.Add(AbsoluteTimeout)
	_, err = s.Session(token, false)
	require.ErrorIs(t, err, ErrInvalidSession, "session outlived the 30-day limit")
	require.NotContains(t, s.data.Sessions, digest(token), "expired session was not removed")

	// An observed expiry is durable: a small clock step back cannot revive it.
	now = now.Add(-10 * time.Second)
	_, err = s.Session(token, false)
	require.ErrorIs(t, err, ErrInvalidSession)
}

// A reauthenticated session keeps the deadline of the original sign-in.
func TestReauthenticationKeepsThirtyDayDeadline(t *testing.T) {
	s, _ := fixture(t)
	now := s.data.HighWater
	s.now = func() time.Time { return now }
	token, err := s.CreateSession()
	require.NoError(t, err)
	deadline := now.Add(AbsoluteTimeout)

	now = now.Add(29 * 24 * time.Hour)
	rotated, err := s.Reauthenticate(token, "old")
	require.NoError(t, err)
	session, err := s.Session(rotated, true)
	require.NoError(t, err)
	require.True(t, session.ExpiresAt.Equal(deadline))
	require.True(t, session.VerifiedAt.Equal(now))

	now = deadline
	_, err = s.Session(rotated, true)
	require.ErrorIs(t, err, ErrInvalidSession, "reauthentication extended the 30-day limit")
}

// Revocation does not depend on the session lifetime: a session that is weeks
// old is still ended by every existing revocation path.
func TestLongLivedSessionRemainsRevocable(t *testing.T) {
	for _, path := range []string{"sign-out", "sign-out-all", "password-change", "clock-rollback"} {
		t.Run(path, func(t *testing.T) {
			s, _ := fixture(t)
			now := s.data.HighWater
			s.now = func() time.Time { return now }
			token, err := s.CreateSession()
			require.NoError(t, err)
			now = now.Add(20 * 24 * time.Hour)
			_, err = s.Session(token, true)
			require.NoError(t, err)

			switch path {
			case "sign-out":
				require.NoError(t, s.Revoke(token))
			case "sign-out-all":
				require.NoError(t, s.RevokeAll())
			case "password-change":
				require.NoError(t, s.ChangePassword("old", "a unique replacement phrase 42"))
			case "clock-rollback":
				now = now.Add(-61 * time.Second)
			}

			_, err = s.Session(token, false)
			require.ErrorIs(t, err, ErrInvalidSession)
		})
	}
}
