package repository

import (
	"bytes"
	"context"
	"sync"
)

type Memory struct {
	mu    sync.Mutex
	links map[string]Link
}

func NewMemory() *Memory {
	return &Memory{
		links: make(map[string]Link),
	}
}

var _ Repository = (*Memory)(nil)

func (m *Memory) FindByCode(ctx context.Context, code string) (Link, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	link, exists := m.links[code]
	if !exists {
		return Link{}, ErrNotFound
	}
	return link, nil
}

func (m *Memory) FindByHash(ctx context.Context, hash []byte) (Link, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, l := range m.links {
		if bytes.Equal(l.URLHash, hash) {
			return l, nil
		}
	}
	return Link{}, ErrNotFound
}

func (m *Memory) Insert(ctx context.Context, link Link) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.links[link.ShortCode]; exists {
		return &ConflictError{Constraint: ConstraintShortCode}
	}

	for _, l := range m.links {
		if bytes.Equal(l.URLHash, link.URLHash) {
			return &ConflictError{Constraint: ConstraintURLHash}
		}
	}
	m.links[link.ShortCode] = link
	return nil
}

func (m *Memory) Ping(ctx context.Context) error {
	return nil
}
