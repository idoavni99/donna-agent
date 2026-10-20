package agent

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/OpenRouterTeam/go-sdk/models/components"
)

const (
	defaultMaxSessions        = 128
	defaultSessionTTL         = 2 * time.Hour
	defaultMaxHistoryMessages = 24
	maxSessionIDBytes         = 128
	maxApprovedSessionURLs    = 10
	maxRetainedURLBytes       = 2048
)

var (
	ErrInvalidChatSessionID = errors.New("invalid chat session ID")
	ErrChatSessionNotFound  = errors.New("chat session not found or expired")
	ErrChatSessionCapacity  = errors.New("chat session capacity reached")
)

type chatSession struct {
	mu           sync.Mutex
	lastUsed     time.Time
	exchanges    [][]components.ChatMessages
	historyCount int
	approvedURLs []string
	allowedHosts []string
}

func (runtime *Runtime) acquireSession(requestedID string) (string, *chatSession, error) {
	requestedID = strings.TrimSpace(requestedID)
	if len(requestedID) > maxSessionIDBytes || !validSessionID(requestedID) {
		return "", nil, ErrInvalidChatSessionID
	}

	now := time.Now()
	runtime.sessionsMu.Lock()
	defer runtime.sessionsMu.Unlock()
	for id, session := range runtime.sessions {
		if now.Sub(session.lastUsed) > runtime.sessionTTL {
			delete(runtime.sessions, id)
		}
	}

	if requestedID != "" {
		session, exists := runtime.sessions[requestedID]
		if !exists {
			return "", nil, ErrChatSessionNotFound
		}
		session.lastUsed = now
		return requestedID, session, nil
	}
	if len(runtime.sessions) >= runtime.maxSessions {
		return "", nil, ErrChatSessionCapacity
	}

	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", nil, err
	}
	id := hex.EncodeToString(bytes)
	session := &chatSession{lastUsed: now}
	runtime.sessions[id] = session
	return id, session, nil
}

func (runtime *Runtime) releaseSession(id string, session *chatSession) {
	session.mu.Unlock()
	runtime.sessionsMu.Lock()
	if runtime.sessions[id] == session {
		session.lastUsed = time.Now()
	}
	runtime.sessionsMu.Unlock()
}

func validSessionID(id string) bool {
	for _, character := range id {
		if !(character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' || character == '-' || character == '_') {
			return false
		}
	}
	return true
}

func (session *chatSession) addURLs(urls []string, maxAllowedHosts int) {
	for _, rawURL := range urls {
		if len(session.approvedURLs) == maxApprovedSessionURLs || len(rawURL) > maxRetainedURLBytes {
			break
		}
		if !containsString(session.approvedURLs, rawURL) {
			session.approvedURLs = append(session.approvedURLs, rawURL)
		}
	}
	for _, rawURL := range session.approvedURLs {
		parsed, err := url.Parse(rawURL)
		if err != nil {
			continue
		}
		host := parsed.Hostname()
		if len(session.allowedHosts) == maxAllowedHosts {
			break
		}
		if !containsString(session.allowedHosts, host) {
			session.allowedHosts = append(session.allowedHosts, host)
		}
	}
}

func (session *chatSession) appendExchange(exchange []components.ChatMessages, maxHistoryMessages int) {
	session.exchanges = append(session.exchanges, exchange)
	session.historyCount += len(exchange)
	for len(session.exchanges) > 1 && session.historyCount > maxHistoryMessages {
		session.historyCount -= len(session.exchanges[0])
		session.exchanges = session.exchanges[1:]
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
