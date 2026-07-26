package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/liamawhite/workouts/internal/storage/sqlcgen"
)

// ErrUserNotFound is returned by Delete when no user matches the given ID.
var ErrUserNotFound = errors.New("user not found")

// User is a row from the users table.
type User struct {
	ID        string
	Name      string
	CreatedAt time.Time
}

// Users is the repository for the users table, backed by sqlc-generated
// queries (see internal/storage/queries.sql and sqlcgen/) - this type just
// adapts sqlcgen's raw string created_at column to a time.Time on the way
// out, and generates IDs/timestamps on the way in, so callers never deal
// with the storage-level representation.
type Users struct {
	q *sqlcgen.Queries
}

// NewUsers returns a Users repository backed by db.
func NewUsers(db *sql.DB) *Users {
	return &Users{q: sqlcgen.New(db)}
}

// List returns every user, oldest first.
func (u *Users) List(ctx context.Context) ([]User, error) {
	rows, err := u.q.ListUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing users: %w", err)
	}

	users := make([]User, 0, len(rows))
	for _, row := range rows {
		user, err := fromRow(row)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}

	return users, nil
}

// Create inserts a new user with a generated ID and returns it.
func (u *Users) Create(ctx context.Context, name string) (User, error) {
	row, err := u.q.CreateUser(ctx, sqlcgen.CreateUserParams{
		ID:        uuid.NewString(),
		Name:      name,
		CreatedAt: time.Now().UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return User{}, fmt.Errorf("creating user: %w", err)
	}

	return fromRow(row)
}

// Delete removes the user with the given ID. It returns ErrUserNotFound if
// no such user exists.
func (u *Users) Delete(ctx context.Context, id string) error {
	affected, err := u.q.DeleteUser(ctx, id)
	if err != nil {
		return fmt.Errorf("deleting user: %w", err)
	}
	if affected == 0 {
		return ErrUserNotFound
	}

	return nil
}

func fromRow(row sqlcgen.User) (User, error) {
	createdAt, err := time.Parse(time.RFC3339Nano, row.CreatedAt)
	if err != nil {
		return User{}, fmt.Errorf("parsing user created_at: %w", err)
	}

	return User{ID: row.ID, Name: row.Name, CreatedAt: createdAt}, nil
}
