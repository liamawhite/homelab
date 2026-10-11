package store

import (
	"context"
	"melodical/internal/music"
)

type Settings struct {
	Instrument string `json:"instrument"`
	Topic      string `json:"topic"`
	Key        string `json:"key"`
	Difficulty string `json:"difficulty"`
	Mode       string `json:"mode"`
	Count      int    `json:"count"`
	Minutes    int    `json:"minutes"`
	Sound      bool   `json:"sound"`
	Theme      string `json:"theme"`
}

func Defaults() Settings {
	return Settings{"both", "mixed", "auto", "auto", "quick", 5, 3, false, "system"}
}

type Review struct {
	Skill      string `json:"skill"`
	Instrument string `json:"instrument"`
	Topic      string `json:"topic"`
	Key        string `json:"key"`
	Attempts   int64  `json:"attempts"`
	Correct    int64  `json:"correct"`
	Unaided    int64  `json:"unaided"`
	SpeedTotal int64  `json:"speedTotal"`
	SpeedCount int64  `json:"speedCount"`
	Streak     int64  `json:"streak"`
	DueAt      int64  `json:"dueAt"`
	LastSeen   int64  `json:"lastSeen"`
}
type Recent struct {
	Instrument string `json:"instrument"`
	Topic      string `json:"topic"`
	Key        string `json:"key"`
	Correct    bool   `json:"correct"`
	Assisted   bool   `json:"assisted"`
	CreatedAt  int64  `json:"createdAt"`
}
type Stats struct {
	Reviews []Review `json:"reviews"`
	Recent  []Recent `json:"recent"`
}

// Store is the application persistence boundary; adapters own their SQL and transactions.
type Store interface {
	SaveQuestion(context.Context, music.Question) error
	Question(context.Context, string) (music.Question, error)
	Submit(context.Context, music.Question, music.Answer) (music.Feedback, error)
	Stats(context.Context) (Stats, error)
	Settings(context.Context) (Settings, error)
	SaveSettings(context.Context, Settings) error
	Close() error
}
