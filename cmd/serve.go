package cmd

import (
	"fmt"
	"os"
	"os/signal"

	"github.com/spf13/cobra"

	"github.com/joshsymonds/sound-stage/server"
	"github.com/joshsymonds/sound-stage/usdb"
	"github.com/joshsymonds/sound-stage/ytdlp"
)

var (
	servePort       string
	serveStaticDir  string
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
	serveCmd.Flags().StringVar(&serveStaticDir, "static", "", "directory to serve static files from (SPA)")
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
		Port:       servePort,
		LibraryDir: outputDir,
		StaticDir:  serveStaticDir,
		DeckURL:    serveDeckURL,
		DelyricURL: serveDelyricURL,
	}

	// Set up USDB search and download if credentials are available.
	if username != "" && password != "" {
		client, err := usdb.NewClient(username, password)
		if err != nil {
			fmt.Fprintf(os.Stderr, "USDB login failed (search/download disabled): %v\n", err)
		} else {
			cfg.Searcher = client
			cfg.Download = &server.DownloadConfig{
				Client:    client,
				YtDlp:     ytdlp.Downloader{Proxy: proxy, MaxHeight: maxHeight},
				OutputDir: outputDir,
			}
			fmt.Fprintln(os.Stderr, "USDB search and download enabled")
		}
	}

	queue := server.NewQueue()
	srv := server.NewWithQueue(cfg, queue)

	if cfg.DeckURL != "" {
		fmt.Fprintf(os.Stderr, "Deck API: %s\n", cfg.DeckURL)
	}
	if cfg.DelyricURL != "" {
		fmt.Fprintf(os.Stderr, "Delyric worker: %s\n", cfg.DelyricURL)
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)

	go func() {
		fmt.Fprintf(os.Stderr, "SoundStage server starting on :%s (library: %s)\n", cfg.Port, cfg.LibraryDir)
		if err := srv.ListenAndServe(); err != nil {
			fmt.Fprintf(os.Stderr, "server error: %v\n", err)
			os.Exit(1)
		}
	}()

	<-stop
	fmt.Fprintln(os.Stderr, "shutting down")
	return nil
}
