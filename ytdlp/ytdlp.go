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

// DefaultMaxHeight is the default video resolution cap (1080p).
const DefaultMaxHeight = 1080

// Downloader wraps yt-dlp invocations.
type Downloader struct {
	Proxy     string // SOCKS5 proxy URL, e.g. "socks5://10.64.0.1:1080"
	MaxHeight int    // Max video height in pixels (0 uses DefaultMaxHeight)
}

// retryArgs returns yt-dlp flags for retry behavior on transient errors.
func retryArgs() []string {
	return []string{
		"--retry-sleep", "http:10",
		"--retries", "5",
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
	args = append(args, retryArgs()...)
	args = d.appendProxy(args)
	args = append(args, "--", videoURL)
	return args
}

// buildVideoArgs constructs yt-dlp arguments for video-only download (VP9 WebM, no audio track).
func (d Downloader) buildVideoArgs(videoURL, destDir, filename string) []string {
	maxH := d.MaxHeight
	if maxH <= 0 {
		maxH = DefaultMaxHeight
	}
	formatSpec := fmt.Sprintf(
		"bestvideo[vcodec^=vp9][height<=%d]/bestvideo[height<=%d]",
		maxH, maxH,
	)
	outPath := filepath.Join(destDir, filename)
	args := []string{
		"-f", formatSpec,
		"--merge-output-format", "webm",
		"-o", outPath,
		"--no-playlist",
		"--no-warnings",
	}
	args = append(args, retryArgs()...)
	args = d.appendProxy(args)
	args = append(args, "--", videoURL)
	return args
}

// DownloadAudio downloads audio as Opus/WebM from a YouTube URL (YouTube's native format).
func (d Downloader) DownloadAudio(videoURL, destDir, filename string) error {
	return d.run(d.buildAudioArgs(videoURL, destDir, filename))
}

// DownloadVideo downloads video-only as VP9 WebM from a YouTube URL (no audio track).
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
