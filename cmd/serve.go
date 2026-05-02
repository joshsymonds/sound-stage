package cmd

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/signal"
	"time"

	"github.com/spf13/cobra"

	"github.com/joshsymonds/sound-stage/server"
	"github.com/joshsymonds/sound-stage/usdb"
	"github.com/joshsymonds/sound-stage/ytdlp"
)

const queueDriverInterval = 2 * time.Second

// staticFS holds the SPA assets to serve at /. Set by SetStaticFS, which
// main.go calls with the embed.FS subtree before invoking Execute.
// A nil staticFS disables the SPA route (API-only mode, primarily for tests).
var staticFS fs.FS

// SetStaticFS configures the SPA filesystem the serve command will mount.
// Intended to be called once from main before Execute.
func SetStaticFS(f fs.FS) {
	staticFS = f
}

var (
	servePort       string
	serveBindAddr   string
	serveDeckURL    string
	serveDelyricURL string
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the SoundStage web server",
	Long: `Start the SoundStage HTTP server that serves the web frontend
and provides API endpoints for the karaoke queue system.`,
	RunE: runServe,
}

func init() {
	serveCmd.Flags().StringVar(&servePort, "port", "8080", "HTTP server port")
	serveCmd.Flags().StringVar(
		&serveBindAddr, "bind", "127.0.0.1",
		"address to bind the listener (loopback by default; Caddy fronts on :443)",
	)
	serveCmd.Flags().StringVar(
		&serveDeckURL, "deck-url", "",
		"Steam Deck Pascal API base URL (e.g. http://172.31.0.39:9000)",
	)
	serveCmd.Flags().StringVar(
		&serveDelyricURL, "delyric-url", "",
		"Delyric worker base URL (e.g. http://172.31.0.98:9001)",
	)
	rootCmd.AddCommand(serveCmd)
}

func runServe(_ *cobra.Command, _ []string) error {
	cfg := server.Config{
		Port:        servePort,
		BindAddress: serveBindAddr,
		LibraryDir:  outputDir,
		StaticFS:    staticFS,
		DeckURL:     serveDeckURL,
		DelyricURL:  serveDelyricURL,
	}

	// Construct the USDB client up front. This is offline (no network), so
	// it never blocks the listener bind below. Login happens concurrently in
	// a goroutine; until it lands, USDB-gated handlers return HTTP 503.
	client, err := usdb.NewClient(username, password)
	if err != nil {
		return fmt.Errorf("creating USDB client: %w", err)
	}
	cfg.Searcher = client
	cfg.CoverFetcher = client
	cfg.Download = &server.DownloadConfig{
		Client:    client,
		YtDlp:     ytdlp.Downloader{Proxy: proxy, MaxHeight: maxHeight},
		OutputDir: outputDir,
		DeckURL:   serveDeckURL,
	}

	queue := server.NewQueue()

	driver := server.NewQueueDriver(cfg.DeckURL, queue, queueDriverInterval)
	if driver != nil {
		// Wire the driver as the deck-status reporter BEFORE building the
		// server, so /api/deck-status reflects probe state.
		cfg.DeckStatus = driver
		fmt.Fprintf(os.Stderr, "Deck queue driver: %s (tick %s)\n", cfg.DeckURL, queueDriverInterval)
		driver.Start()
	}

	srv := server.NewWithQueue(cfg, queue)

	if cfg.DelyricURL != "" {
		fmt.Fprintf(os.Stderr, "Delyric worker: %s\n", cfg.DelyricURL)
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)

	go func() {
		fmt.Fprintf(os.Stderr,
			"SoundStage server starting on %s:%s (library: %s)\n",
			cfg.BindAddress, cfg.Port, cfg.LibraryDir,
		)
		if err := srv.ListenAndServe(); err != nil {
			fmt.Fprintf(os.Stderr, "server error: %v\n", err)
			os.Exit(1)
		}
	}()

	// Run USDB login concurrently with serving. While it's in progress,
	// /api/usdb/search and /api/download return 503 with Retry-After. With
	// no credentials configured, LoginAsync is a no-op and those endpoints
	// return 503 indefinitely — the operator's signal that USDB isn't set up.
	loginCtx, loginCancel := context.WithCancel(context.Background())
	defer loginCancel()
	if username == "" || password == "" {
		fmt.Fprintln(os.Stderr,
			"USDB credentials not set — search and download will return 503")
	}
	go func() {
		if err := client.LoginAsync(loginCtx); err != nil && !errors.Is(err, context.Canceled) {
			fmt.Fprintf(os.Stderr, "USDB login: %v\n", err)
		}
	}()

	<-stop
	loginCancel()
	if driver != nil {
		driver.Stop()
	}
	fmt.Fprintln(os.Stderr, "shutting down")
	return nil
}
