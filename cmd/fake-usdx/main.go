// Command fake-usdx serves the fakeusdx package as an HTTP stand-in for the
// real USDX Deck API. Use it for end-to-end sound-stage development without a
// running Steam Deck.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joshsymonds/sound-stage/server/fakeusdx"
)

const shutdownTimeout = 5 * time.Second

func main() {
	addr := flag.String("addr", "127.0.0.1:9000",
		"TCP bind address; defaults to localhost. Pass :9000 to expose on LAN "+
			"(only on trusted networks — /refresh reads arbitrary .txt paths)")
	seedPath := flag.String("seed", "", "optional path to a JSON array of {title,artist,duet} songs")
	flag.Parse()

	if err := run(*addr, *seedPath); err != nil {
		log.Fatal(err)
	}
}

func run(addr, seedPath string) error {
	songs := defaultSeed()
	if seedPath != "" {
		loaded, err := loadSeed(seedPath)
		if err != nil {
			return fmt.Errorf("load seed: %w", err)
		}
		songs = loaded
	}

	fake := fakeusdx.New()
	fake.LoadSongs(songs)
	fake.StartClock()

	handler := loggingHandler(fake)
	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		log.Printf("shutting down")
		fake.StopClock()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("server shutdown: %v", err)
		}
	}()

	log.Printf("fake-usdx listening on %s with %d songs", addr, len(songs))
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("listen: %w", err)
	}
	return nil
}

// loadSeed reads a JSON file containing an array of {title, artist, duet}
// song entries and returns them as fakeusdx.Song values.
func loadSeed(path string) ([]fakeusdx.Song, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var entries []struct {
		Title  string `json:"title"`
		Artist string `json:"artist"`
		Duet   bool   `json:"duet"`
	}
	if jsonErr := json.Unmarshal(data, &entries); jsonErr != nil {
		return nil, fmt.Errorf("parse %s: %w", path, jsonErr)
	}
	songs := make([]fakeusdx.Song, len(entries))
	for i, e := range entries {
		songs[i] = fakeusdx.Song{Title: e.Title, Artist: e.Artist, Duet: e.Duet}
	}
	return songs, nil
}

// defaultSeed returns the hardcoded demo library used when --seed is unset.
func defaultSeed() []fakeusdx.Song {
	return []fakeusdx.Song{
		{Title: "Dancing Queen", Artist: "ABBA", Duet: false},
		{Title: "Take On Me", Artist: "a-ha", Duet: false},
		{Title: "Africa", Artist: "Toto", Duet: false},
	}
}

// loggingHandler wraps h with a stderr access log: method, path, status, elapsed.
func loggingHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		h.ServeHTTP(rec, r)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, rec.status, time.Since(start))
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}
