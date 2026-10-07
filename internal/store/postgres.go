package store

import (
	"context"
	"errors"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresStore is a PostgreSQL-backed Store.
type PostgresStore struct {
	pool *pgxpool.Pool
}

const schema = `
CREATE TABLE IF NOT EXISTS users (
	id TEXT PRIMARY KEY,
	username TEXT UNIQUE NOT NULL,
	password_hash TEXT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS tasks (
	id TEXT PRIMARY KEY,
	owner_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	title TEXT NOT NULL,
	description TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'open',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_tasks_owner ON tasks(owner_id);
`

// NewPostgresStore connects to PostgreSQL and ensures the schema exists.
func NewPostgresStore(ctx context.Context, databaseURL string) (*PostgresStore, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	if _, err := pool.Exec(ctx, schema); err != nil {
		pool.Close()
		return nil, err
	}
	return &PostgresStore{pool: pool}, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func (p *PostgresStore) CreateUser(ctx context.Context, username, passwordHash string) (User, error) {
	u := User{ID: newID(), Username: username, PasswordHash: passwordHash}
	err := p.pool.QueryRow(ctx,
		`INSERT INTO users (id, username, password_hash) VALUES ($1, $2, $3) RETURNING created_at`,
		u.ID, u.Username, u.PasswordHash).Scan(&u.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return User{}, ErrConflict
		}
		return User{}, err
	}
	return u, nil
}

func (p *PostgresStore) GetUserByUsername(ctx context.Context, username string) (User, error) {
	var u User
	err := p.pool.QueryRow(ctx,
		`SELECT id, username, password_hash, created_at FROM users WHERE username = $1`,
		username).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrNotFound
		}
		return User{}, err
	}
	return u, nil
}

func (p *PostgresStore) CreateTask(ctx context.Context, ownerID string, in TaskInput) (Task, error) {
	status := in.Status
	if status == "" {
		status = TaskOpen
	}
	var t Task
	err := p.pool.QueryRow(ctx,
		`INSERT INTO tasks (id, owner_id, title, description, status) VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, title, description, status, created_at, updated_at`,
		newID(), ownerID, in.Title, in.Description, string(status)).
		Scan(&t.ID, &t.Title, &t.Description, &t.Status, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return Task{}, err
	}
	return t, nil
}

func (p *PostgresStore) ListTasks(ctx context.Context, ownerID string, f TaskFilter) ([]Task, error) {
	query := `SELECT id, title, description, status, created_at, updated_at FROM tasks WHERE owner_id = $1`
	args := []any{ownerID}
	if f.Status != "" {
		query += ` AND status = $2`
		args = append(args, string(f.Status))
	}
	if f.Query != "" {
		query += ` AND (title ILIKE '%' || $` + strconv.Itoa(len(args)+1) + ` || '%' OR description ILIKE '%' || $` + strconv.Itoa(len(args)+1) + ` || '%')`
		args = append(args, f.Query)
	}
	query += ` ORDER BY created_at DESC`
	limit := f.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	query += ` LIMIT $` + strconv.Itoa(len(args)+1) + ` OFFSET $` + strconv.Itoa(len(args)+2)
	args = append(args, limit, max(f.Offset, 0))

	rows, err := p.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tasks := []Task{}
	for rows.Next() {
		var t Task
		if err := rows.Scan(&t.ID, &t.Title, &t.Description, &t.Status, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

func (p *PostgresStore) GetTask(ctx context.Context, ownerID, id string) (Task, error) {
	var t Task
	err := p.pool.QueryRow(ctx,
		`SELECT id, title, description, status, created_at, updated_at FROM tasks WHERE owner_id = $1 AND id = $2`,
		ownerID, id).Scan(&t.ID, &t.Title, &t.Description, &t.Status, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Task{}, ErrNotFound
		}
		return Task{}, err
	}
	return t, nil
}

func (p *PostgresStore) UpdateTask(ctx context.Context, ownerID, id string, in TaskInput) (Task, error) {
	status := in.Status
	if !status.Valid() {
		// keep existing status when the payload omits it
		cur, err := p.GetTask(ctx, ownerID, id)
		if err != nil {
			return Task{}, err
		}
		status = cur.Status
	}
	var t Task
	err := p.pool.QueryRow(ctx,
		`UPDATE tasks SET title = $3, description = $4, status = $5, updated_at = now()
		 WHERE owner_id = $1 AND id = $2
		 RETURNING id, title, description, status, created_at, updated_at`,
		ownerID, id, in.Title, in.Description, string(status)).
		Scan(&t.ID, &t.Title, &t.Description, &t.Status, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Task{}, ErrNotFound
		}
		return Task{}, err
	}
	return t, nil
}

func (p *PostgresStore) DeleteTask(ctx context.Context, ownerID, id string) error {
	ct, err := p.pool.Exec(ctx, `DELETE FROM tasks WHERE owner_id = $1 AND id = $2`, ownerID, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Close releases the connection pool.
func (p *PostgresStore) Close() error {
	p.pool.Close()
	return nil
}
