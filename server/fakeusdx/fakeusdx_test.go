package fakeusdx_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/joshsymonds/sound-stage/server/fakeusdx"
	"github.com/joshsymonds/sound-stage/server/stableid"
)

func doRequest(t *testing.T, fake *fakeusdx.Fake, method, path string) (*httptest.ResponseRecorder, []byte) {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	rec := httptest.NewRecorder()
	fake.ServeHTTP(rec, req)
	return rec, rec.Body.Bytes()
}

func TestSongs_Empty(t *testing.T) {
	fake := fakeusdx.New()

	rec, body := doRequest(t, fake, http.MethodGet, "/songs")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if string(body) != "[]" {
		t.Errorf("body = %q, want %q (must be empty array, not null)", body, "[]")
	}
}

func TestSongs_OrderAndIDs(t *testing.T) {
	fake := fakeusdx.New()
	seed := []fakeusdx.Song{
		{Title: "Bohemian Rhapsody", Artist: "Queen", Duet: false},
		{Title: "Dancing Queen", Artist: "ABBA", Duet: false},
		{Title: "Africa", Artist: "Toto", Duet: false},
	}
	fake.LoadSongs(seed)

	rec, body := doRequest(t, fake, http.MethodGet, "/songs")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var got []struct {
		ID     string `json:"id"`
		Title  string `json:"title"`
		Artist string `json:"artist"`
		Duet   bool   `json:"duet"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal: %v, body=%s", err, body)
	}
	if len(got) != 3 {
		t.Fatalf("got %d songs, want 3", len(got))
	}

	for i, want := range seed {
		if got[i].Title != want.Title {
			t.Errorf("index %d: title = %q, want %q (order must be preserved)", i, got[i].Title, want.Title)
		}
		if got[i].Artist != want.Artist {
			t.Errorf("index %d: artist = %q, want %q", i, got[i].Artist, want.Artist)
		}
		wantID := stableid.Compute(want.Artist, want.Title, want.Duet)
		if got[i].ID != wantID {
			t.Errorf("index %d: id = %q, want %q (must match stableid.Compute)", i, got[i].ID, wantID)
		}
	}
}

func TestSongs_DuetFlag(t *testing.T) {
	fake := fakeusdx.New()
	fake.LoadSongs([]fakeusdx.Song{
		{Title: "Islands in the Stream", Artist: "Kenny Rogers & Dolly Parton", Duet: true},
	})

	_, body := doRequest(t, fake, http.MethodGet, "/songs")

	var got []map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d songs, want 1", len(got))
	}
	duet, ok := got[0]["duet"].(bool)
	if !ok {
		t.Fatalf("duet field missing or not bool: %v", got[0])
	}
	if !duet {
		t.Errorf("duet = false, want true")
	}
}

func TestNowPlaying_Idle(t *testing.T) {
	fake := fakeusdx.New()

	rec, body := doRequest(t, fake, http.MethodGet, "/now-playing")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if string(body) != "null" {
		t.Errorf("body = %q, want %q", body, "null")
	}
}

func TestNowPlaying_Playing(t *testing.T) {
	fake := fakeusdx.New()
	fake.LoadSongs([]fakeusdx.Song{
		{Title: "Take On Me", Artist: "a-ha", Duet: false},
	})
	id := stableid.Compute("a-ha", "Take On Me", false)

	if err := fake.SetCurrentPlaying(id, 42.715, 243.981); err != nil {
		t.Fatalf("SetCurrentPlaying: %v", err)
	}

	rec, body := doRequest(t, fake, http.MethodGet, "/now-playing")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var got struct {
		ID       string  `json:"id"`
		Title    string  `json:"title"`
		Artist   string  `json:"artist"`
		Elapsed  float64 `json:"elapsed"`
		Duration float64 `json:"duration"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal: %v, body=%s", err, body)
	}
	if got.ID != id {
		t.Errorf("id = %q, want %q", got.ID, id)
	}
	if got.Title != "Take On Me" {
		t.Errorf("title = %q", got.Title)
	}
	if got.Artist != "a-ha" {
		t.Errorf("artist = %q", got.Artist)
	}
	if got.Elapsed != 42.715 {
		t.Errorf("elapsed = %v, want 42.715", got.Elapsed)
	}
	if got.Duration != 243.981 {
		t.Errorf("duration = %v, want 243.981", got.Duration)
	}
}

func TestNowPlaying_AfterSetIdle(t *testing.T) {
	fake := fakeusdx.New()
	fake.LoadSongs([]fakeusdx.Song{{Title: "X", Artist: "Y", Duet: false}})
	id := stableid.Compute("Y", "X", false)
	if err := fake.SetCurrentPlaying(id, 1.0, 100.0); err != nil {
		t.Fatalf("SetCurrentPlaying: %v", err)
	}

	fake.SetIdle()

	_, body := doRequest(t, fake, http.MethodGet, "/now-playing")
	if string(body) != "null" {
		t.Errorf("after SetIdle, body = %q, want %q", body, "null")
	}
}

func TestSetCurrentPlaying_UnknownID(t *testing.T) {
	fake := fakeusdx.New()
	fake.LoadSongs([]fakeusdx.Song{{Title: "Known", Artist: "Artist", Duet: false}})

	err := fake.SetCurrentPlaying("deadbeefdeadbeef", 1.0, 100.0)
	if err == nil {
		t.Fatal("SetCurrentPlaying with unknown ID: err = nil, want error")
	}

	_, body := doRequest(t, fake, http.MethodGet, "/now-playing")
	if string(body) != "null" {
		t.Errorf("state must be unchanged, body = %q, want %q", body, "null")
	}
}

func TestMethodNotAllowed(t *testing.T) {
	fake := fakeusdx.New()

	for _, path := range []string{"/songs", "/now-playing"} {
		rec, body := doRequest(t, fake, http.MethodPost, path)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("POST %s: status = %d, want 405", path, rec.Code)
		}
		var got map[string]string
		if err := json.Unmarshal(body, &got); err != nil {
			t.Errorf("POST %s: body not JSON: %v", path, err)
			continue
		}
		if got["error"] == "" {
			t.Errorf("POST %s: error field empty in %q", path, body)
		}
	}
}

func TestUnknownPath(t *testing.T) {
	fake := fakeusdx.New()

	rec, body := doRequest(t, fake, http.MethodGet, "/foo")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
	var got map[string]string
	if err := json.Unmarshal(body, &got); err != nil {
		t.Errorf("body not JSON: %v, body=%s", err, body)
	}
	if got["error"] == "" {
		t.Errorf("error field empty in %q", body)
	}
}

func TestContentType(t *testing.T) {
	fake := fakeusdx.New()

	cases := []struct {
		method, path string
	}{
		{http.MethodGet, "/songs"},
		{http.MethodGet, "/now-playing"},
		{http.MethodPost, "/songs"},         // 405
		{http.MethodGet, "/does-not-exist"}, // 404
	}
	for _, tc := range cases {
		rec, _ := doRequest(t, fake, tc.method, tc.path)
		ct := rec.Header().Get("Content-Type")
		if ct != "application/json" {
			t.Errorf("%s %s: Content-Type = %q, want %q", tc.method, tc.path, ct, "application/json")
		}
	}
}

func TestConcurrentReadsAndWrites(t *testing.T) {
	fake := fakeusdx.New()
	songs := []fakeusdx.Song{
		{Title: "A", Artist: "X", Duet: false},
		{Title: "B", Artist: "Y", Duet: false},
	}
	fake.LoadSongs(songs)
	idA := stableid.Compute("X", "A", false)
	idB := stableid.Compute("Y", "B", false)

	stop := make(chan struct{})
	var wg sync.WaitGroup

	wg.Go(func() {
		toggle := false
		for {
			select {
			case <-stop:
				return
			default:
			}
			if toggle {
				_ = fake.SetCurrentPlaying(idA, 10, 100)
			} else {
				_ = fake.SetCurrentPlaying(idB, 20, 200)
			}
			toggle = !toggle
		}
	})

	wg.Go(func() {
		for {
			select {
			case <-stop:
				return
			default:
			}
			fake.SetIdle()
		}
	})

	for range 10 {
		wg.Go(func() {
			for {
				select {
				case <-stop:
					return
				default:
				}
				doRequest(t, fake, http.MethodGet, "/songs")
				doRequest(t, fake, http.MethodGet, "/now-playing")
			}
		})
	}

	time.Sleep(100 * time.Millisecond)
	close(stop)
	wg.Wait()
}
