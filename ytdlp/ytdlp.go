// Package ytdlp wraps yt-dlp for downloading audio and video from YouTube.
package ytdlp

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// downloadTimeout is the maximum duration for a single yt-dlp download.
const downloadTimeout = 30 * time.Minute

// Downloader wraps yt-dlp invocations.
type Downloader struct {
	Proxy string // SOCKS5 proxy URL, e.g. "socks5://10.64.0.1:1080"
}

// rateLimitArgs returns yt-dlp flags for responsible rate limiting.
func rateLimitArgs() []string {
	return []string{
		"--sleep-interval", "10",
		"--max-sleep-interval", "30",
		"--sleep-requests", "1.5",
		"--limit-rate", "5M",
		"--retry-sleep", "http:30",
		"--retries", "10",
	}
}

// buildAudioArgs constructs yt-dlp arguments for audio-only download (Opus/WebM).
func (d Downloader) buildAudioArgs(videoURL, destDir, filename string) []string {
	outPath := filepath.Join(destDir, filename)
	args := []string{
		"-f", "bestaudio[acodec=opus]/bestaudio",
		"--merge-output-format", "webm",
		"-o", outPath,
		"--no-playlist",
		"--no-warnings",
	}
	args = append(args, rateLimitArgs()...)
	args = d.appendProxy(args)
	args = append(args, "--", videoURL)
	return args
}

// buildVideoArgs constructs yt-dlp arguments for video download (VP9/Opus WebM).
func (d Downloader) buildVideoArgs(videoURL, destDir, filename string) []string {
	outPath := filepath.Join(destDir, filename)
	args := []string{
		"-f", "bestvideo[vcodec^=vp9]+bestaudio[acodec=opus]/bestvideo+bestaudio/best",
		"--merge-output-format", "webm",
		"-o", outPath,
		"--no-playlist",
		"--no-warnings",
	}
	args = append(args, rateLimitArgs()...)
	args = d.appendProxy(args)
	args = append(args, "--", videoURL)
	return args
}

// DownloadAudio downloads audio as Opus/WebM from a YouTube URL (YouTube's native format).
func (d Downloader) DownloadAudio(videoURL, destDir, filename string) error {
	return d.run(d.buildAudioArgs(videoURL, destDir, filename))
}

// DownloadVideo downloads video as VP9/Opus in WebM from a YouTube URL (YouTube's native format).
func (d Downloader) DownloadVideo(videoURL, destDir, filename string) error {
	return d.run(d.buildVideoArgs(videoURL, destDir, filename))
}

func (d Downloader) appendProxy(args []string) []string {
	if d.Proxy != "" {
		return append(args, "--proxy", d.Proxy)
	}
	return args
}

func (d Downloader) run(args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), downloadTimeout)
	defer cancel()

	//nolint:gosec // yt-dlp args are constructed internally
	cmd := exec.CommandContext(ctx, "yt-dlp", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("yt-dlp: %w", err)
	}

	return nil
}
