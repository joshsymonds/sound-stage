package ytdlp

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Downloader wraps yt-dlp invocations.
type Downloader struct {
	Proxy string // SOCKS5 proxy URL, e.g. "socks5://10.64.0.1:1080"
}

// DownloadAudio downloads audio as Opus/WebM from a YouTube URL (YouTube's native format).
func (d Downloader) DownloadAudio(videoURL, destDir, filename string) error {
	outPath := filepath.Join(destDir, filename)
	args := []string{
		"-f", "bestaudio[acodec=opus]/bestaudio",
		"--merge-output-format", "webm",
		"-o", outPath,
		"--no-playlist",
		"--no-warnings",
	}
	args = d.appendProxy(args)
	args = append(args, "--", videoURL)
	return d.run(args)
}

// DownloadVideo downloads video as VP9/Opus in WebM from a YouTube URL (YouTube's native format).
func (d Downloader) DownloadVideo(videoURL, destDir, filename string) error {
	outPath := filepath.Join(destDir, filename)
	args := []string{
		"-f", "bestvideo[vcodec^=vp9]+bestaudio[acodec=opus]/bestvideo+bestaudio/best",
		"--merge-output-format", "webm",
		"-o", outPath,
		"--no-playlist",
		"--no-warnings",
	}
	args = d.appendProxy(args)
	args = append(args, "--", videoURL)
	return d.run(args)
}

// Search uses yt-dlp to search YouTube and return the first matching video URL.
func (d Downloader) Search(query string) (string, error) {
	args := []string{
		fmt.Sprintf("ytsearch1:%s", query),
		"--flat-playlist",
		"-j",
		"--no-warnings",
	}
	args = d.appendProxy(args)

	//nolint:gosec // yt-dlp args are constructed internally
	cmd := exec.CommandContext(context.Background(), "yt-dlp", args...)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("yt-dlp search: %w", err)
	}

	// The JSON output contains a "url" or "webpage_url" field
	// For simplicity, extract the ID and build the URL
	// yt-dlp --flat-playlist outputs JSON with "id" field
	return extractURLFromJSON(out)
}

func (d Downloader) appendProxy(args []string) []string {
	if d.Proxy != "" {
		return append(args, "--proxy", d.Proxy)
	}
	return args
}

func (d Downloader) run(args []string) error {
	//nolint:gosec // yt-dlp args are constructed internally
	cmd := exec.CommandContext(context.Background(), "yt-dlp", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("yt-dlp: %w", err)
	}
	return nil
}
