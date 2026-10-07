package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sort"
	"strings"
	"sync"
	"time"
)

// MemoryStore is a thread-safe in-memory Store. It is the default when no
// DATABASE_URL is configured, and it is what the test suite exercises.
type MemoryStore struct {
	mu    sync.RWMutex
	users map[string]User   // by username
	tasks map[string][]Task // by ownerID
}

// NewMemoryStore returns an empty in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		users: make(map[string]User),
		tasks: make(map[string][]Task),
	}
}

func newID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func (m *MemoryStore) CreateUser(ctx context.Context, username, passwordHash string) (User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.users[username]; ok {
		return User{}, ErrConflict
	}
	u := User{ID: newID(), Username: username, PasswordHash: passwordHash, CreatedAt: time.Now().UTC()}
	m.users[username] = u
	return u, nil
}

func (m *MemoryStore) GetUserByUsername(ctx context.Context, username string) (User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	u, ok := m.users[username]
	if !ok {
		return User{}, ErrNotFound
	}
	return u, nil
}

func (m *MemoryStore) CreateTask(ctx context.Context, ownerID string, in TaskInput) (Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().UTC()
	status := in.Status
	if status == "" {
		status = TaskOpen
	}
	t := Task{ID: newID(), Title: in.Title, Description: in.Description, Status: status, CreatedAt: now, UpdatedAt: now}
	m.tasks[ownerID] = append(m.tasks[ownerID], t)
	return t, nil
}

func (m *MemoryStore) ListTasks(ctx context.Context, ownerID string, f TaskFilter) ([]Task, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []Task
	for _, t := range m.tasks[ownerID] {
		if f.Status != "" && t.Status != f.Status {
			continue
		}
		if f.Query != "" && !strings.Contains(strings.ToLower(t.Title+" "+t.Description), strings.ToLower(f.Query)) {
			continue
		}
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	if f.Offset >= len(out) {
		return []Task{}, nil
	}
	out = out[f.Offset:]
	if f.Limit > 0 && len(out) > f.Limit {
		out = out[:f.Limit]
	}
	return out, nil
}

func (m *MemoryStore) findTask(ownerID, id string) (int, Task, bool) {
	for i, t := range m.tasks[ownerID] {
		if t.ID == id {
			return i, t, true
		}
	}
	return 0, Task{}, false
}

func (m *MemoryStore) GetTask(ctx context.Context, ownerID, id string) (Task, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, t, ok := m.findTask(ownerID, id)
	if !ok {
		return Task{}, ErrNotFound
	}
	return t, nil
}

func (m *MemoryStore) UpdateTask(ctx context.Context, ownerID, id string, in TaskInput) (Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	i, t, ok := m.findTask(ownerID, id)
	if !ok {
		return Task{}, ErrNotFound
	}
	t.Title = in.Title
	t.Description = in.Description
	if in.Status.Valid() {
		t.Status = in.Status
	}
	t.UpdatedAt = time.Now().UTC()
	m.tasks[ownerID][i] = t
	return t, nil
}

func (m *MemoryStore) DeleteTask(ctx context.Context, ownerID, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	i, _, ok := m.findTask(ownerID, id)
	if !ok {
		return ErrNotFound
	}
	m.tasks[ownerID] = append(m.tasks[ownerID][:i], m.tasks[ownerID][i+1:]...)
	return nil
}

// Close is a no-op for the memory store.
func (m *MemoryStore) Close() error { return nil }
