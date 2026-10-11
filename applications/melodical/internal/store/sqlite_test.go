package store

import (
	"context"
	"melodical/internal/music"
	"path/filepath"
	"sync"
	"testing"
)

func TestDurabilityIdempotencyAndTiming(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "music.db")
	s, err := OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	q := music.Generate("piano", "keys", "D", 1)
	if err = s.SaveQuestion(ctx, q); err != nil {
		t.Fatal(err)
	}
	a := music.Answer{Points: q.Expected, ElapsedMS: 4200}
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			f, e := s.Submit(ctx, q, a)
			if e != nil || !f.Correct {
				t.Errorf("submit: %+v %v", f, e)
			}
		}()
	}
	wg.Wait()
	settings := Defaults()
	settings.Instrument = "guitar"
	if err = s.SaveSettings(ctx, settings); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	stats, err := s.Stats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(stats.Reviews) != 1 || stats.Reviews[0].Attempts != 1 || stats.Reviews[0].SpeedTotal != 4200 || stats.Reviews[0].SpeedCount != 1 {
		t.Fatalf("duplicate retries or timing corrupted progress: %+v", stats)
	}
	loaded, err := s.Settings(ctx)
	if err != nil || loaded.Instrument != "guitar" {
		t.Fatal(loaded, err)
	}
	loadedQ, err := s.Question(ctx, q.ID)
	if err != nil || !loadedQ.Validate(a).Correct {
		t.Fatal("question didn't survive restart", err)
	}
	for _, kind := range []string{"assisted", "interrupted"} {
		q = music.Generate("piano", "keys", "D", 1)
		if err = s.SaveQuestion(ctx, q); err != nil {
			t.Fatal(err)
		}
		a.Assisted = kind == "assisted"
		a.Interrupted = kind == "interrupted"
		if _, err = s.Submit(ctx, q, a); err != nil {
			t.Fatal(err)
		}
	}
	stats, err = s.Stats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	r := stats.Reviews[0]
	if r.Attempts != 3 || r.SpeedCount != 1 || r.Unaided != 2 {
		t.Fatalf("hint/background attempts must not count as speed: %+v", r)
	}
}
