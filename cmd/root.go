// Package cmd implements the sound-stage CLI commands.
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/joshsymonds/sound-stage/ytdlp"
)

var (
	outputDir string
	proxy     string
	username  string
	password  string
	maxHeight int
)

var rootCmd = &cobra.Command{
	Use:   "sound-stage",
	Short: "Download UltraStar karaoke songs from USDB",
	Long:  `sound-stage searches usdb.animux.de for karaoke songs, downloads UltraStar txt files, and fetches audio/video from YouTube via yt-dlp.`,
}

// Execute runs the root command and returns any error.
func Execute() error {
	if err := rootCmd.Execute(); err != nil {
		return fmt.Errorf("executing root command: %w", err)
	}
	return nil
}

func init() {
	rootCmd.PersistentFlags().
		StringVarP(&outputDir, "output", "o", defaultOutputDir(), "output directory for downloaded songs")
	rootCmd.PersistentFlags().
		StringVar(&proxy, "proxy", envOrDefault("SOUND_STAGE_PROXY", ""), "SOCKS5 proxy for yt-dlp (e.g. socks5://10.64.0.1:1080)")
	rootCmd.PersistentFlags().StringVarP(&username, "username", "u", os.Getenv("USDB_USERNAME"), "USDB username")
	rootCmd.PersistentFlags().StringVarP(&password, "password", "p", os.Getenv("USDB_PASSWORD"), "USDB password")
	rootCmd.PersistentFlags().IntVar(
		&maxHeight, "max-height", ytdlp.DefaultMaxHeight,
		"max video resolution height in pixels (e.g. 720, 1080, 2160)",
	)
}

func defaultOutputDir() string {
	if dir := os.Getenv("SOUND_STAGE_OUTPUT"); dir != "" {
		return dir
	}
	return "/mnt/music/sound-stage"
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func requireCredentials() error {
	if username == "" || password == "" {
		return fmt.Errorf(
			"USDB credentials required: set --username/--password or USDB_USERNAME/USDB_PASSWORD env vars",
		)
	}
	return nil
}
