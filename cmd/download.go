// Package cmd implements the sound-stage CLI commands.
package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/joshsymonds/sound-stage/archive"
	"github.com/joshsymonds/sound-stage/usdb"
	"github.com/joshsymonds/sound-stage/ytdlp"
)

var downloadCmd = &cobra.Command{
	Use:   "download [song IDs...]",
	Short: "Download songs by USDB ID",
	Long: `Download one or more songs from USDB by their song IDs.
Fetches the UltraStar txt, cover art, and audio/video from YouTube.
Use --from to read IDs from a file (one per line).`,
	RunE: runDownload,
}

var (
	downloadVideo bool
	downloadAudio bool
	downloadFrom  string
)

func init() {
	downloadCmd.Flags().BoolVar(&downloadVideo, "video", true, "download video from YouTube")
	downloadCmd.Flags().BoolVar(&downloadAudio, "audio", true, "download audio from YouTube")
	downloadCmd.Flags().StringVar(&downloadFrom, "from", "", "read song IDs from file (one per line)")
	rootCmd.AddCommand(downloadCmd)
}

func runDownload(cmd *cobra.Command, args []string) error {
	if err := requireCredentials(); err != nil {
		return err
	}

	ids, err := collectIDs(args, downloadFrom)
	if err != nil {
		return err
	}

	if len(ids) == 0 {
		return fmt.Errorf("no song IDs provided (pass IDs as arguments or use --from)")
	}

	client, err := usdb.NewClient(username, password)
	if err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	dl := ytdlp.Downloader{Proxy: proxy}

	for _, idStr := range ids {
		if err := downloadSong(client, dl, idStr); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "error downloading %s: %v\n", idStr, err)
			continue
		}
	}

	return nil
}

func collectIDs(args []string, fromFile string) ([]string, error) {
	ids := append([]string{}, args...)

	if fromFile == "" {
		return ids, nil
	}

	file, err := os.Open(fromFile)
	if err != nil {
		return nil, fmt.Errorf("opening ID file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			ids = append(ids, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading ID file: %w", err)
	}

	return ids, nil
}

func downloadSong(client *usdb.Client, dl ytdlp.Downloader, idStr string) error {
	var id int
	if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil {
		return fmt.Errorf("invalid song ID %q", idStr)
	}

	// Check download archive
	if archive.IsDownloaded(outputDir, id) {
		fmt.Printf("Skipping song #%d (already downloaded)\n", id)
		return nil
	}

	fmt.Printf("Fetching song #%d from USDB...\n", id)

	details, err := client.GetSongDetails(id)
	if err != nil {
		return fmt.Errorf("fetching song details: %w", err)
	}

	txt, err := client.GetSongTxt(id)
	if err != nil {
		return fmt.Errorf("fetching song txt: %w", err)
	}

	// Build output directory
	dirName := sanitizePath(fmt.Sprintf("%s - %s", details.Artist, details.Title))
	songDir := filepath.Join(outputDir, dirName)

	fmt.Printf("Downloading: %s - %s → %s\n", details.Artist, details.Title, songDir)

	// Write the txt file, download cover, and fetch media
	song, err := usdb.PrepareSong(txt, details, songDir)
	if err != nil {
		return fmt.Errorf("preparing song: %w", err)
	}

	// Download cover art
	if details.HasCover {
		fmt.Printf("  Downloading cover art...\n")
		if err := client.DownloadCover(id, songDir); err != nil {
			fmt.Printf("  Warning: cover download failed: %v\n", err)
		}
	}

	// Download media from YouTube
	if song.YouTubeURL == "" {
		fmt.Printf("  Warning: no YouTube URL found, skipping media download\n")
		return markAndFinish(id)
	}

	if downloadAudio {
		fmt.Printf("  Downloading audio...\n")
		if err := dl.DownloadAudio(song.YouTubeURL, songDir, song.AudioFile); err != nil {
			return fmt.Errorf("downloading audio: %w", err)
		}
	}

	if downloadVideo {
		fmt.Printf("  Downloading video...\n")
		if err := dl.DownloadVideo(song.YouTubeURL, songDir, song.VideoFile); err != nil {
			fmt.Printf("  Warning: video download failed: %v\n", err)
		}
	}

	return markAndFinish(id)
}

func markAndFinish(songID int) error {
	if err := archive.MarkDownloaded(outputDir, songID); err != nil {
		return fmt.Errorf("updating download archive: %w", err)
	}

	fmt.Printf("  Done!\n")

	return nil
}

func sanitizePath(input string) string {
	replacer := strings.NewReplacer(
		"/", "-",
		"\\", "-",
		":", "-",
		"*", "",
		"?", "",
		"\"", "",
		"<", "",
		">", "",
		"|", "",
	)
	return strings.TrimSpace(replacer.Replace(input))
}
