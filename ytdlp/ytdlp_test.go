package ytdlp

import (
	"testing"
)

func TestBuildAudioArgs_ContainsRateLimiting(t *testing.T) {
	t.Parallel()
	dl := Downloader{}
	args := dl.buildAudioArgs("https://youtube.com/watch?v=test", "/tmp", "audio.webm")

	assertContains(t, args, "--sleep-interval")
	assertContains(t, args, "--max-sleep-interval")
	assertContains(t, args, "--limit-rate")
	assertContains(t, args, "--retry-sleep")
}

func TestBuildVideoArgs_ContainsRateLimiting(t *testing.T) {
	t.Parallel()
	dl := Downloader{}
	args := dl.buildVideoArgs("https://youtube.com/watch?v=test", "/tmp", "video.webm")

	assertContains(t, args, "--sleep-interval")
	assertContains(t, args, "--max-sleep-interval")
	assertContains(t, args, "--limit-rate")
	assertContains(t, args, "--retry-sleep")
}

func TestBuildVideoArgs_ContainsVP9Format(t *testing.T) {
	t.Parallel()
	dl := Downloader{}
	args := dl.buildVideoArgs("https://youtube.com/watch?v=test", "/tmp", "video.webm")

	assertContains(t, args, "-f")
	assertContainsValue(t, args, "-f", "bestvideo[vcodec^=vp9]+bestaudio[acodec=opus]/bestvideo+bestaudio/best")
	assertContains(t, args, "--merge-output-format")
}

func TestBuildAudioArgs_ContainsOpusFormat(t *testing.T) {
	t.Parallel()
	dl := Downloader{}
	args := dl.buildAudioArgs("https://youtube.com/watch?v=test", "/tmp", "audio.webm")

	assertContains(t, args, "-f")
	assertContainsValue(t, args, "-f", "bestaudio[acodec=opus]/bestaudio")
}

func TestBuildVideoArgs_WithProxy(t *testing.T) {
	t.Parallel()
	dl := Downloader{Proxy: "socks5://10.64.0.1:1080"}
	args := dl.buildVideoArgs("https://youtube.com/watch?v=test", "/tmp", "video.webm")

	assertContains(t, args, "--proxy")
	assertContainsValue(t, args, "--proxy", "socks5://10.64.0.1:1080")
}

func TestBuildVideoArgs_NoProxyByDefault(t *testing.T) {
	t.Parallel()
	dl := Downloader{}
	args := dl.buildVideoArgs("https://youtube.com/watch?v=test", "/tmp", "video.webm")

	for _, arg := range args {
		if arg == "--proxy" {
			t.Error("--proxy should not be present when Proxy is empty")
		}
	}
}

func assertContains(t *testing.T, args []string, flag string) {
	t.Helper()
	for _, arg := range args {
		if arg == flag {
			return
		}
	}
	t.Errorf("expected args to contain %q, got %v", flag, args)
}

func assertContainsValue(t *testing.T, args []string, flag, value string) {
	t.Helper()
	for i, arg := range args {
		if arg == flag && i+1 < len(args) && args[i+1] == value {
			return
		}
	}
	t.Errorf("expected args to contain %q %q", flag, value)
}
