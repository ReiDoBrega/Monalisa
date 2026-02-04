package cdm

import (
	"sync"

	"monalisa/pkg/exceptions"
	"monalisa/pkg/license"
	"monalisa/pkg/module"
	"monalisa/pkg/types"

	"github.com/google/uuid"
)

type CDM struct {
	module   *module.Module
	sessions map[string]*Session
	mu       sync.RWMutex
}

func FromModule(mod *module.Module) *CDM {
	return &CDM{
		module:   mod,
		sessions: make(map[string]*Session),
	}
}

func (c *CDM) Open() (string, error) {
	sessionID := uuid.New().String()
	store := c.module.CreateStore()

	session := NewSession(sessionID, c.module, store)
	if err := session.Initialize(); err != nil {
		return "", err
	}

	c.mu.Lock()
	c.sessions[sessionID] = session
	c.mu.Unlock()

	return sessionID, nil
}

func (c *CDM) Close(sessionID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if session, ok := c.sessions[sessionID]; ok {
		session.Cleanup()
		delete(c.sessions, sessionID)
	}
}

func (c *CDM) ParseLicense(sessionID string, lic *license.License) error {
	session, err := c.getSession(sessionID)
	if err != nil {
		return err
	}
	return session.ParseLicense(lic)
}

func (c *CDM) GetKeys(sessionID string, keyType types.KeyType) ([]*types.Key, error) {
	session, err := c.getSession(sessionID)
	if err != nil {
		return nil, err
	}
	return session.GetKeys(keyType), nil
}

func (c *CDM) getSession(sessionID string) (*Session, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	session, ok := c.sessions[sessionID]
	if !ok {
		return nil, exceptions.NewSessionError("Session not found: %s", sessionID)
	}
	return session, nil
}
