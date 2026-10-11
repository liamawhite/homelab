package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"math/rand/v2"
	"melodical/internal/music"
	"melodical/internal/store"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
)

type Server struct{ store store.Store }

func New(storage store.Store, assets fs.FS) http.Handler {
	s := &Server{storage}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		if _, err := storage.Settings(r.Context()); err != nil {
			fail(w, 503, "Storage unavailable")
			return
		}
		write(w, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /api/stats", s.stats)
	mux.HandleFunc("GET /api/settings", s.settings)
	mux.HandleFunc("PUT /api/settings", s.saveSettings)
	mux.HandleFunc("POST /api/questions", s.question)
	mux.HandleFunc("POST /api/questions/{id}/answers", s.answer)
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) { fail(w, 404, "Unknown API endpoint") })
	files := http.FileServer(http.FS(assets))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" && r.Method != "HEAD" {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Path != "/" && !strings.HasPrefix(r.URL.Path, "/assets/") {
			http.NotFound(w, r)
			return
		}
		if r.URL.Path == "/" {
			w.Header().Set("Cache-Control", "no-cache")
		} else {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		files.ServeHTTP(w, r)
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; font-src 'self'; base-uri 'self'; frame-ancestors 'none'")
		mux.ServeHTTP(w, r)
	})
}
func write(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}
func decode(w http.ResponseWriter, r *http.Request, out any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 32768)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		fail(w, 400, "Invalid request body")
		return false
	}
	if err := d.Decode(new(any)); err != io.EOF {
		fail(w, 400, "Expected one JSON object")
		return false
	}
	return true
}
func internal(w http.ResponseWriter, err error) {
	slog.Error("request failed", "error", err)
	fail(w, 500, "Could not save or load practice. Please try again.")
}
func (s *Server) stats(w http.ResponseWriter, r *http.Request) {
	v, err := s.store.Stats(r.Context())
	if err != nil {
		internal(w, err)
		return
	}
	write(w, v)
}
func (s *Server) settings(w http.ResponseWriter, r *http.Request) {
	v, err := s.store.Settings(r.Context())
	if err != nil {
		internal(w, err)
		return
	}
	write(w, v)
}
func ValidSettings(v store.Settings) bool {
	return slices.Contains([]string{"piano", "guitar", "both"}, v.Instrument) && slices.Contains(append([]string{"mixed"}, music.Topics...), v.Topic) && slices.Contains(append([]string{"auto"}, music.Keys...), v.Key) && slices.Contains([]string{"auto", "1", "2", "3"}, v.Difficulty) && slices.Contains([]string{"quick", "timed", "free"}, v.Mode) && v.Count >= 1 && v.Count <= 100 && v.Minutes >= 1 && v.Minutes <= 60 && slices.Contains([]string{"system", "light", "dark"}, v.Theme)
}
func (s *Server) saveSettings(w http.ResponseWriter, r *http.Request) {
	var v store.Settings
	if !decode(w, r, &v) {
		return
	}
	if !ValidSettings(v) {
		fail(w, 400, "Invalid practice settings")
		return
	}
	if err := s.store.SaveSettings(r.Context(), v); err != nil {
		internal(w, err)
		return
	}
	write(w, v)
}

type Request struct {
	Instrument string `json:"instrument"`
	Topic      string `json:"topic"`
	Key        string `json:"key"`
	Difficulty string `json:"difficulty"`
	RetryID    string `json:"retryId,omitempty"`
}

func (s *Server) question(w http.ResponseWriter, r *http.Request) {
	var v Request
	if !decode(w, r, &v) {
		return
	}
	settings := store.Defaults()
	settings.Instrument = v.Instrument
	settings.Topic = v.Topic
	settings.Key = v.Key
	settings.Difficulty = v.Difficulty
	if !ValidSettings(settings) {
		fail(w, 400, "Invalid exercise options")
		return
	}
	if v.RetryID != "" {
		old, err := s.store.Question(r.Context(), v.RetryID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				fail(w, 404, "Exercise no longer available")
			} else {
				internal(w, err)
			}
			return
		}
		old.ID = music.ID()
		if err = s.store.SaveQuestion(r.Context(), old); err != nil {
			internal(w, err)
			return
		}
		write(w, old)
		return
	}
	stats, err := s.store.Stats(r.Context())
	if err != nil {
		internal(w, err)
		return
	}
	instrument := v.Instrument
	if instrument == "both" {
		instrument = []string{"piano", "guitar"}[rand.IntN(2)]
	}
	topic := v.Topic
	if topic == "mixed" {
		topics := music.Topics
		topic = topics[rand.IntN(len(topics))]
	}
	key := v.Key
	if key == "auto" {
		keys := []string{"C", "G", "D", "F"}
		var attempts int64
		for _, rev := range stats.Reviews {
			if rev.Instrument == instrument && rev.Topic == topic {
				attempts += rev.Attempts
			}
		}
		if attempts >= 15 || v.Difficulty == "3" {
			keys = music.Keys
		}
		key = keys[rand.IntN(len(keys))]
	}
	// Most questions revisit due/weak material; the remainder explores new combinations.
	if rand.IntN(4) != 0 {
		candidates := []store.Review{}
		for _, rev := range stats.Reviews {
			if rev.Instrument == instrument && (v.Topic == "mixed" || rev.Topic == topic) && (v.Key == "auto" || rev.Key == v.Key) && (rev.DueAt <= time.Now().Unix() || rev.Unaided*5 < rev.Attempts*4) {
				candidates = append(candidates, rev)
			}
		}
		if len(candidates) > 0 {
			chosen := candidates[rand.IntN(len(candidates))]
			topic = chosen.Topic
			key = chosen.Key
		}
	}
	level := 1
	if v.Difficulty != "auto" {
		level, _ = strconv.Atoi(v.Difficulty)
	} else {
		var attempts, correct, speedTotal, speedCount int64
		var recentMisses bool
		for _, rev := range stats.Reviews {
			if rev.Instrument == instrument && rev.Topic == topic {
				attempts += rev.Attempts
				correct += rev.Unaided
				speedTotal += rev.SpeedTotal
				speedCount += rev.SpeedCount
				if rev.Streak == 0 && rev.LastSeen > time.Now().Add(-10*time.Minute).Unix() {
					recentMisses = true
				}
			}
		}
		if attempts >= 8 && correct*10 >= attempts*8 && !recentMisses {
			level = 2
		}
		if attempts >= 20 && correct*10 >= attempts*9 && speedCount > 0 && speedTotal/speedCount < 8000 && !recentMisses {
			level = 3
		}
	}
	q := music.Generate(instrument, topic, key, level)
	if err = s.store.SaveQuestion(r.Context(), q); err != nil {
		internal(w, err)
		return
	}
	write(w, q)
}
func (s *Server) answer(w http.ResponseWriter, r *http.Request) {
	var a music.Answer
	if !decode(w, r, &a) {
		return
	}
	if a.ElapsedMS < 0 || a.ElapsedMS > 24*60*60*1000 || len(a.Points) > 100 || len(a.Choice) > 200 {
		fail(w, 400, "Invalid answer")
		return
	}
	q, err := s.store.Question(r.Context(), r.PathValue("id"))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			fail(w, 404, "Exercise not found")
		} else {
			internal(w, err)
		}
		return
	}
	f, err := s.store.Submit(r.Context(), q, a)
	if err != nil {
		internal(w, err)
		return
	}
	write(w, f)
}
