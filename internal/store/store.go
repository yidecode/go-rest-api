// Package store defines the persistence layer: domain models and the Store interface.
package store

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNotFound = errors.New("not found")
	ErrConflict = errors.New("already exists")
)

// User is an account that can own tasks.
type User struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

// TaskStatus is the lifecycle state of a task.
type TaskStatus string

const (
	TaskOpen       TaskStatus = "open"
	TaskInProgress TaskStatus = "in_progress"
	TaskDone       TaskStatus = "done"
)

// Valid reports whether s is a known task status.
func (s TaskStatus) Valid() bool {
	switch s {
	case TaskOpen, TaskInProgress, TaskDone:
		return true
	}
	return false
}

// Task is a to-do item owned by a user.
type Task struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description,omitempty"`
	Status      TaskStatus `json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// TaskInput is the writable payload for creating/updating a task.
type TaskInput struct {
	Title       string
	Description string
	Status      TaskStatus
}

// TaskFilter narrows ListTasks results.
type TaskFilter struct {
	Status TaskStatus
	Query  string
	Limit  int
	Offset int
}

// Store abstracts task persistence so handlers stay storage-agnostic.
type Store interface {
	CreateUser(ctx context.Context, username, passwordHash string) (User, error)
	GetUserByUsername(ctx context.Context, username string) (User, error)

	CreateTask(ctx context.Context, ownerID string, in TaskInput) (Task, error)
	ListTasks(ctx context.Context, ownerID string, f TaskFilter) ([]Task, error)
	GetTask(ctx context.Context, ownerID, id string) (Task, error)
	UpdateTask(ctx context.Context, ownerID, id string, in TaskInput) (Task, error)
	DeleteTask(ctx context.Context, ownerID, id string) error

	Close() error
}
