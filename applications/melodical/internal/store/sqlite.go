package store

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"melodical/internal/music"
	"melodical/internal/store/sqlitedb"
	_ "modernc.org/sqlite"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

//go:embed sql/schema.sql
var schema string

type SQLite struct {
	db      *sql.DB
	queries *sqlitedb.Queries
}

func OpenSQLite(path string) (*SQLite, error) {
	if path != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
			return nil, err
		}
	}
	dsn := path
	if path != ":memory:" {
		absolute, err := filepath.Abs(path)
		if err != nil {
			return nil, err
		}
		dsn = (&url.URL{Scheme: "file", Path: absolute}).String()
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	for _, statement := range []string{"PRAGMA busy_timeout = 5000", "PRAGMA foreign_keys = ON", "PRAGMA journal_mode = WAL", schema} {
		if _, err = db.Exec(statement); err != nil {
			db.Close()
			return nil, fmt.Errorf("initialize SQLite: %w", err)
		}
	}
	adapter := &SQLite{db, sqlitedb.New(db)}
	if err = adapter.queries.PruneQuestions(context.Background(), time.Now().Add(-30*24*time.Hour).Unix()); err != nil {
		db.Close()
		return nil, err
	}
	return adapter, nil
}
func (s *SQLite) Close() error { return s.db.Close() }
func (s *SQLite) SaveQuestion(ctx context.Context, q music.Question) error {
	b, err := json.Marshal(q.Stored())
	if err != nil {
		return err
	}
	return s.queries.PutQuestion(ctx, sqlitedb.PutQuestionParams{ID: q.ID, Payload: string(b), CreatedAt: time.Now().Unix()})
}
func (s *SQLite) Question(ctx context.Context, id string) (music.Question, error) {
	b, err := s.queries.GetQuestion(ctx, id)
	if err != nil {
		return music.Question{}, err
	}
	var saved music.StoredQuestion
	err = json.Unmarshal([]byte(b), &saved)
	return saved.Question(), err
}
func bint(b bool) int64 {
	if b {
		return 1
	}
	return 0
}
func (s *SQLite) Submit(ctx context.Context, q music.Question, a music.Answer) (music.Feedback, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return music.Feedback{}, err
	}
	defer tx.Rollback()
	queries := s.queries.WithTx(tx)
	existing, err := queries.GetAttempt(ctx, q.ID)
	if err == nil {
		var f music.Feedback
		err = json.Unmarshal([]byte(existing), &f)
		return f, err
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return music.Feedback{}, err
	}
	f := q.Validate(a)
	payload, err := json.Marshal(f)
	if err != nil {
		return f, err
	}
	now := time.Now().Unix()
	if err = queries.PutAttempt(ctx, sqlitedb.PutAttemptParams{QuestionID: q.ID, Skill: q.Skill, Instrument: q.Instrument, Topic: q.Topic, KeyName: q.Key, Correct: bint(f.Correct), Assisted: bint(a.Assisted), Interrupted: bint(a.Interrupted), ElapsedMs: a.ElapsedMS, Level: int64(q.Level), Feedback: string(payload), CreatedAt: now}); err != nil {
		return f, err
	}
	review, err := queries.GetReview(ctx, q.Skill)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return f, err
	}
	review.Attempts++
	review.Correct += bint(f.Correct)
	review.LastSeen = now
	if f.Correct && !a.Assisted {
		review.Unaided++
		review.Streak++
	} else {
		review.Streak = 0
	}
	if f.Correct && !a.Assisted && !a.Interrupted && a.ElapsedMS > 0 {
		review.SpeedTotal += a.ElapsedMS
		review.SpeedCount++
	}
	delay := int64(10 * 60)
	if review.Streak > 0 {
		delay = int64(math.Pow(2, float64(min(review.Streak-1, 5)))) * 24 * 60 * 60
	}
	review.DueAt = now + delay
	err = queries.PutReview(ctx, sqlitedb.PutReviewParams{Skill: q.Skill, Instrument: q.Instrument, Topic: q.Topic, KeyName: q.Key, Attempts: review.Attempts, Correct: review.Correct, Unaided: review.Unaided, SpeedTotal: review.SpeedTotal, SpeedCount: review.SpeedCount, Streak: review.Streak, DueAt: review.DueAt, LastSeen: now})
	if err != nil {
		return f, err
	}
	return f, tx.Commit()
}
func (s *SQLite) Stats(ctx context.Context) (Stats, error) {
	reviews, err := s.queries.ListReviews(ctx)
	if err != nil {
		return Stats{}, err
	}
	recent, err := s.queries.RecentAttempts(ctx)
	if err != nil {
		return Stats{}, err
	}
	out := Stats{Reviews: []Review{}, Recent: []Recent{}}
	for _, r := range reviews {
		out.Reviews = append(out.Reviews, Review{r.Skill, r.Instrument, r.Topic, r.KeyName, r.Attempts, r.Correct, r.Unaided, r.SpeedTotal, r.SpeedCount, r.Streak, r.DueAt, r.LastSeen})
	}
	for _, r := range recent {
		out.Recent = append(out.Recent, Recent{r.Instrument, r.Topic, r.KeyName, r.Correct == 1, r.Assisted == 1, r.CreatedAt})
	}
	return out, nil
}
func (s *SQLite) Settings(ctx context.Context) (Settings, error) {
	payload, err := s.queries.GetSettings(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return Defaults(), nil
	}
	if err != nil {
		return Settings{}, err
	}
	var v Settings
	err = json.Unmarshal([]byte(payload), &v)
	return v, err
}
func (s *SQLite) SaveSettings(ctx context.Context, v Settings) error {
	payload, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return s.queries.PutSettings(ctx, string(payload))
}
