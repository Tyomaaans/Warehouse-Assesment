package domain

import "time"

type SessionEntity struct {
	SessionID    string
	UserID       string
	Role         string
	RefreshToken string
	IssuedAt     time.Time
	LastActiveAt time.Time
	ExpiresAt    time.Time
}

type RefreshTokenEntity struct {
	UserID     string
	SessionID  string
	Role       string
	RememberMe bool
	CreatedAt  time.Time
	ExpiresAt  time.Time
	TTL        time.Duration
}

type TokenPairEntity struct {
	AccessToken  string
	RefreshToken string
	SessionID    string
	TTL          time.Duration
}
