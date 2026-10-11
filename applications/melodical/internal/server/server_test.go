package server

import (
	"bytes"
	"encoding/json"
	"io"
	"io/fs"
	"melodical/internal/music"
	"melodical/internal/store"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func TestPracticeAPI(t *testing.T) {
	storage, err := store.OpenSQLite(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	assets := fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("<title>Melodical</title>")}}
	var filesystem fs.FS = assets
	s := httptest.NewServer(New(storage, filesystem))
	defer s.Close()
	request := func(method, path string, body any) (int, []byte) {
		t.Helper()
		payload, _ := json.Marshal(body)
		r, err := http.NewRequest(method, s.URL+path, bytes.NewReader(payload))
		if err != nil {
			t.Fatal(err)
		}
		res, err := http.DefaultClient.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		data, _ := io.ReadAll(res.Body)
		return res.StatusCode, data
	}
	status, body := request("POST", "/api/questions", Request{Instrument: "piano", Topic: "keys", Key: "D", Difficulty: "1"})
	if status != 200 {
		t.Fatal(status, string(body))
	}
	var q music.Question
	if err = json.Unmarshal(body, &q); err != nil {
		t.Fatal(err)
	}
	private, err := storage.Question(t.Context(), q.ID)
	if err != nil {
		t.Fatal(err)
	}
	status, body = request("POST", "/api/questions/"+q.ID+"/answers", music.Answer{Points: private.Expected, ElapsedMS: 2000})
	if status != 200 {
		t.Fatal(status, string(body))
	}
	var f music.Feedback
	json.Unmarshal(body, &f)
	if !f.Correct {
		t.Fatal(string(body))
	}
	status, _ = request("POST", "/api/questions", Request{Instrument: "piano", Topic: "keys", Key: "bad", Difficulty: "1"})
	if status != 400 {
		t.Fatal("invalid options accepted")
	}
	status, _ = request("POST", "/api/questions/missing/answers", music.Answer{})
	if status != 404 {
		t.Fatal("unknown question response", status)
	}
	status, _ = request("GET", "/api/not-real", nil)
	if status != 404 {
		t.Fatal("API fallback", status)
	}
	status, body = request("GET", "/", nil)
	if status != 200 || !bytes.Contains(body, []byte("Melodical")) {
		t.Fatal("static assets unavailable", status)
	}
	status, body = request("GET", "/api/stats", nil)
	if status != 200 || !bytes.Contains(body, []byte(`"attempts":1`)) {
		t.Fatal("progress not saved", status, string(body))
	}
}
