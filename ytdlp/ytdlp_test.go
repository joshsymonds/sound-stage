package ytdlp

import (
	"testing"
)

func TestBuildAudioArgs_ContainsRetryConfig(t *testing.T) {
	t.Parallel()
	dl := Downloader{}
	args := dl.buildAudioArgs("https://youtube.com/watch?v=test", "/tmp", "audio.webm")

	assertContains(t, args, "--retry-sleep")
	assertContains(t, args, "--retries")
}

func TestBuildVideoArgs_ContainsRetryConfig(t *testing.T) {
	t.Parallel()
	dl := Downloader{}
	args := dl.buildVideoArgs("https://youtube.com/watch?v=test", "/tmp", "video.webm")

	assertContains(t, args, "--retry-sleep")
	assertContains(t, args, "--retries")
}

func TestBuildVideoArgs_ContainsVP9Format(t *testing.T) {
	t.Parallel()
	dl := Downloader{}
	args := dl.buildVideoArgs("https://youtube.com/watch?v=test", "/tmp", "video.webm")

	assertContains(t, args, "-f")
	assertContainsValue(
		t, args, "-f",
		"bestvideo[vcodec^=vp9][height<=1080]/bestvideo[height<=1080]",
	)
}

func TestBuildVideoArgs_CustomMaxHeight(t *testing.T) {
	t.Parallel()
	dl := Downloader{MaxHeight: 720}
	args := dl.buildVideoArgs("https://youtube.com/watch?v=test", "/tmp", "video.webm")

	assertContainsValue(
		t, args, "-f",
		"bestvideo[vcodec^=vp9][height<=720]/bestvideo[height<=720]",
	)
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
