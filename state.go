package main

import (
	"sync"
	"time"

	"go.mau.fi/whatsmeow/store/sqlstore"
)

// global container for creating new devices on re-pair
var sqlContainer *sqlstore.Container

// ChatMode represents the current flow for a user
type ChatMode string

const (
	ModeMenu         ChatMode = "menu"
	ModeInfo         ChatMode = "info"
	ModeResendVote   ChatMode = "resend_vote"
	ModeRegistration ChatMode = "registration"
)

// Session keeps simple per-user state in memory
type Session struct {
	Mode    ChatMode
	Updated time.Time
}

var (
	sessions   = make(map[string]*Session)
	sessionsMu sync.Mutex
)

// sessionTTL defines how long a session is considered valid since last update
const sessionTTL = 15 * time.Minute

func getSession(key string) *Session {
	sessionsMu.Lock()
	defer sessionsMu.Unlock()
	s, ok := sessions[key]
	if !ok {
		s = &Session{Mode: ModeMenu, Updated: time.Now()}
		sessions[key] = s
		return s
	}
	// Expire session if TTL passed
	if time.Since(s.Updated) > sessionTTL {
		s.Mode = ModeMenu
		s.Updated = time.Now()
	}
	return s
}

func setSessionMode(key string, mode ChatMode) {
	sessionsMu.Lock()
	if s, ok := sessions[key]; ok {
		s.Mode = mode
		s.Updated = time.Now()
	} else {
		sessions[key] = &Session{Mode: mode, Updated: time.Now()}
	}
	sessionsMu.Unlock()
}
