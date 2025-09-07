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
	ModeProfil       ChatMode = "profil"
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

// getSessionWithNew is like getSession but also returns whether the session
// is newly created or has just been reset due to TTL expiry. Use this to
// trigger first-message greetings.
func getSessionWithNew(key string) (*Session, bool) {
	sessionsMu.Lock()
	defer sessionsMu.Unlock()
	if s, ok := sessions[key]; ok {
		if time.Since(s.Updated) > sessionTTL {
			s.Mode = ModeMenu
			s.Updated = time.Now()
			return s, true
		}
		return s, false
	}
	s := &Session{Mode: ModeMenu, Updated: time.Now()}
	sessions[key] = s
	return s, true
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
