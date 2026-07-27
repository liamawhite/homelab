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

// ErrBlockNotFound is returned when no block matches the given ID.
var ErrBlockNotFound = errors.New("block not found")

// Block is a row from the blocks table - one time period within a Cycle
// (the "week" replacement term). Append-only, ordered by Position.
type Block struct {
	ID        string
	CycleID   string
	Name      string
	Position  int64
	CreatedAt time.Time
}

// Blocks is the repository for the blocks table, backed by sqlc-generated
// queries (see internal/storage/queries.sql and sqlcgen/).
type Blocks struct {
	db *sql.DB
	q  *sqlcgen.Queries
}

// NewBlocks returns a Blocks repository backed by db.
func NewBlocks(db *sql.DB) *Blocks {
	return &Blocks{db: db, q: sqlcgen.New(db)}
}

// ListByCycle returns every block for cycleID, ordered by position.
func (b *Blocks) ListByCycle(ctx context.Context, cycleID string) ([]Block, error) {
	rows, err := b.q.ListBlocksByCycle(ctx, cycleID)
	if err != nil {
		return nil, fmt.Errorf("listing blocks: %w", err)
	}

	blocks := make([]Block, 0, len(rows))
	for _, row := range rows {
		block, err := blockFromRow(row)
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, block)
	}

	return blocks, nil
}

// Get returns the block with the given ID. It returns ErrBlockNotFound if
// no such block exists.
func (b *Blocks) Get(ctx context.Context, id string) (Block, error) {
	row, err := b.q.GetBlock(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Block{}, ErrBlockNotFound
		}
		return Block{}, fmt.Errorf("getting block: %w", err)
	}

	return blockFromRow(row)
}

// Create appends a new block to the end of cycleID's block list.
func (b *Blocks) Create(ctx context.Context, cycleID, name string) (Block, error) {
	var block Block
	err := withTx(ctx, b.db, func(tx *sql.Tx) error {
		q := b.q.WithTx(tx)

		maxPos, err := q.MaxBlockPosition(ctx, cycleID)
		if err != nil {
			return fmt.Errorf("finding max block position: %w", err)
		}
		pos, err := asPosition(maxPos)
		if err != nil {
			return err
		}

		row, err := q.CreateBlock(ctx, sqlcgen.CreateBlockParams{
			ID:        uuid.NewString(),
			CycleID:   cycleID,
			Name:      name,
			Position:  pos + 1,
			CreatedAt: time.Now().UTC().Format(time.RFC3339Nano),
		})
		if err != nil {
			return fmt.Errorf("creating block: %w", err)
		}

		block, err = blockFromRow(row)
		return err
	})
	if err != nil {
		return Block{}, err
	}

	return block, nil
}

// Delete removes the block with the given ID, cascading to its
// exercise_sets. It returns ErrBlockNotFound if no such block exists.
func (b *Blocks) Delete(ctx context.Context, id string) error {
	affected, err := b.q.DeleteBlock(ctx, id)
	if err != nil {
		return fmt.Errorf("deleting block: %w", err)
	}
	if affected == 0 {
		return ErrBlockNotFound
	}

	return nil
}

func blockFromRow(row sqlcgen.Block) (Block, error) {
	createdAt, err := time.Parse(time.RFC3339Nano, row.CreatedAt)
	if err != nil {
		return Block{}, fmt.Errorf("parsing block created_at: %w", err)
	}

	return Block{
		ID:        row.ID,
		CycleID:   row.CycleID,
		Name:      row.Name,
		Position:  row.Position,
		CreatedAt: createdAt,
	}, nil
}
