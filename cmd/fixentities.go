package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/joshsymonds/sound-stage/server"
	"github.com/joshsymonds/sound-stage/usdb"
)

// entityPattern matches a stale HTML entity such as &amp;, &quot;, or &#39;.
var entityPattern = regexp.MustCompile(`&[a-zA-Z#][a-zA-Z0-9]*;`)

// refreshTimeout bounds each Deck /refresh POST.
const refreshTimeout = 5 * time.Second

var (
	fixEntitiesApply          bool
	fixEntitiesDeckURL        string
	fixEntitiesDeckLibraryDir string
)

var fixEntitiesCmd = &cobra.Command{
	Use:   "fix-entities",
	Short: "Repair stale HTML entities in song.txt headers and directory names",
	Long: `Scan the library for song.txt header tags and song directory names that
still contain HTML entities (&amp;, &quot;, &#39;, ...) from before download-time
decoding existed, and repair them.

Dry-run by default: prints every would-be change without writing anything.
Pass --apply to perform the fixes. With --apply and --deck-url set, POSTs
/refresh to the Deck for each touched song so a running USDX picks up the
changes.`,
	RunE: runFixEntities,
}

func init() {
	fixEntitiesCmd.Flags().BoolVar(
		&fixEntitiesApply, "apply", false,
		"perform the fixes (default is a dry run that only prints them)",
	)
	fixEntitiesCmd.Flags().StringVar(
		&fixEntitiesDeckURL, "deck-url", "",
		"Steam Deck Pascal API base URL (e.g. http://172.31.0.39:9000); "+
			"with --apply, POSTs /refresh for each touched song",
	)
	fixEntitiesCmd.Flags().StringVar(
		&fixEntitiesDeckLibraryDir, "deck-library-dir", "",
		"library path as the Deck mounts it (e.g. /var/mnt/music/sound-stage); "+
			"used to translate /refresh paths — empty sends server-local paths",
	)
	rootCmd.AddCommand(fixEntitiesCmd)
}

func runFixEntities(cmd *cobra.Command, _ []string) error {
	_, err := fixEntities(cmd.Context(), fixEntitiesConfig{
		libraryDir:     outputDir,
		apply:          fixEntitiesApply,
		deckURL:        fixEntitiesDeckURL,
		deckLibraryDir: fixEntitiesDeckLibraryDir,
		out:            cmd.OutOrStdout(),
	})
	return err
}

// fixEntitiesConfig carries the settings for one fix-entities run.
type fixEntitiesConfig struct {
	libraryDir     string
	apply          bool
	deckURL        string
	deckLibraryDir string
	out            io.Writer
}

// fixEntitiesResult summarizes a run. In a dry run the counts report the
// changes that would be made.
type fixEntitiesResult struct {
	filesRewritten int
	dirsRenamed    int
	refreshOK      int
	refreshFailed  int
}

// fixEntities walks <libraryDir>/*/song.txt, repairs HTML entities in header
// tag values and song directory names, and (with apply + deckURL) notifies
// the Deck of each touched song. Returns an error only on I/O failures.
func fixEntities(ctx context.Context, cfg fixEntitiesConfig) (fixEntitiesResult, error) {
	var result fixEntitiesResult
	txtPaths, err := filepath.Glob(filepath.Join(cfg.libraryDir, "*", "song.txt"))
	if err != nil {
		return result, fmt.Errorf("scanning library %s: %w", cfg.libraryDir, err)
	}

	client := &http.Client{Timeout: refreshTimeout}
	for _, txtPath := range txtPaths {
		finalTxtPath, txtChanged, dirRenamed, songErr := fixSong(cfg, txtPath)
		if songErr != nil {
			return result, songErr
		}
		if txtChanged {
			result.filesRewritten++
		}
		if dirRenamed {
			result.dirsRenamed++
		}
		if cfg.apply && cfg.deckURL != "" && (txtChanged || dirRenamed) {
			if refreshDeck(ctx, client, cfg, finalTxtPath) {
				result.refreshOK++
			} else {
				result.refreshFailed++
			}
		}
	}

	fmt.Fprintf(cfg.out, "%d files rewritten, %d dirs renamed, %d refreshes ok, %d refreshes failed\n",
		result.filesRewritten, result.dirsRenamed, result.refreshOK, result.refreshFailed)
	return result, nil
}

// fixSong repairs one song: header tag values first, then the directory name.
// Returns the song.txt path after any rename, whether the txt content changed,
// and whether the directory was renamed. In a dry run it reports the would-be
// changes without writing.
func fixSong(cfg fixEntitiesConfig, txtPath string) (string, bool, bool, error) {
	data, err := os.ReadFile(txtPath)
	if err != nil {
		return txtPath, false, false, fmt.Errorf("reading %s: %w", txtPath, err)
	}

	lines := strings.Split(string(data), "\n")
	txtChanged := false
	for i, line := range lines {
		fixed, wasFixed := fixHeaderLine(line)
		if !wasFixed {
			continue
		}
		fmt.Fprintf(cfg.out, "%s: %s → %s\n", txtPath,
			strings.TrimSuffix(line, "\r"), strings.TrimSuffix(fixed, "\r"))
		lines[i] = fixed
		txtChanged = true
	}
	if txtChanged && cfg.apply {
		if writeErr := os.WriteFile(txtPath, []byte(strings.Join(lines, "\n")), 0o600); writeErr != nil {
			return txtPath, false, false, fmt.Errorf("writing %s: %w", txtPath, writeErr)
		}
	}

	finalTxtPath, dirRenamed, renameErr := maybeRenameDir(cfg, txtPath, lines)
	if renameErr != nil {
		return txtPath, txtChanged, false, renameErr
	}
	return finalTxtPath, txtChanged, dirRenamed, nil
}

// fixHeaderLine unescapes HTML entities in the VALUE of a "#KEY:VALUE" header
// line. Non-header lines, keys, and values without entities are left
// byte-identical; a trailing \r is preserved.
func fixHeaderLine(line string) (string, bool) {
	if !strings.HasPrefix(line, "#") {
		return line, false
	}
	colon := strings.Index(line, ":")
	if colon < 0 {
		return line, false
	}
	value := strings.TrimSuffix(line[colon+1:], "\r")
	if !entityPattern.MatchString(value) {
		return line, false
	}
	fixedValue := usdb.NormalizeText(value)
	if fixedValue == value {
		return line, false
	}
	fixed := line[:colon+1] + fixedValue
	if strings.HasSuffix(line, "\r") {
		fixed += "\r"
	}
	return fixed, true
}

// maybeRenameDir renames the song directory to the sanitized
// "ARTIST - TITLE" derived from the (fixed) header tags — but only when the
// current name still contains an HTML entity. A rename whose target already
// exists is skipped with a warning. Returns the song.txt path after any
// rename and whether a rename happened (or would happen, in a dry run).
func maybeRenameDir(cfg fixEntitiesConfig, txtPath string, lines []string) (string, bool, error) {
	dirPath := filepath.Dir(txtPath)
	dirName := filepath.Base(dirPath)
	if !entityPattern.MatchString(dirName) {
		return txtPath, false, nil
	}
	artist := headerTagValue(lines, "ARTIST")
	title := headerTagValue(lines, "TITLE")
	if artist == "" || title == "" {
		return txtPath, false, nil
	}
	expected := usdb.SanitizePath(artist + " - " + title)
	if expected == "" || expected == dirName {
		return txtPath, false, nil
	}
	target := filepath.Join(filepath.Dir(dirPath), expected)
	if _, statErr := os.Stat(target); statErr == nil {
		fmt.Fprintf(cfg.out, "warning: %s: rename target %q already exists; skipping\n", dirPath, expected)
		return txtPath, false, nil
	}
	fmt.Fprintf(cfg.out, "%s: %s → %s\n", dirPath, dirName, expected)
	if !cfg.apply {
		return txtPath, true, nil
	}
	if renameErr := os.Rename(dirPath, target); renameErr != nil {
		return txtPath, false, fmt.Errorf("renaming %s to %s: %w", dirPath, target, renameErr)
	}
	return filepath.Join(target, "song.txt"), true, nil
}

// headerTagValue returns the trimmed value of the first "#KEY:VALUE" header
// line matching key (case-insensitive), or "" when absent.
func headerTagValue(lines []string, key string) string {
	prefix := "#" + key + ":"
	for _, line := range lines {
		trimmed := strings.TrimSuffix(line, "\r")
		if strings.HasPrefix(strings.ToUpper(trimmed), prefix) {
			return strings.TrimSpace(trimmed[len(prefix):])
		}
	}
	return ""
}

// refreshDeck POSTs {"path": <deck-visible txt path>} to <deck-url>/refresh
// so a running USDX re-reads the touched song. The path is translated via
// server.DeckPath when --deck-library-dir is set; otherwise the server-local
// path is sent. Failures are logged per song and never abort the run.
func refreshDeck(ctx context.Context, client *http.Client, cfg fixEntitiesConfig, txtPath string) bool {
	refreshPath := txtPath
	if mapped, ok := server.DeckPath(txtPath, cfg.libraryDir, cfg.deckLibraryDir); ok {
		refreshPath = mapped
	}
	body, err := json.Marshal(map[string]string{"path": refreshPath})
	if err != nil {
		fmt.Fprintf(cfg.out, "%s: refresh failed: %v\n", txtPath, err)
		return false
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.deckURL+"/refresh", bytes.NewReader(body))
	if err != nil {
		fmt.Fprintf(cfg.out, "%s: refresh failed: %v\n", txtPath, err)
		return false
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintf(cfg.out, "%s: refresh failed: %v\n", txtPath, err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(cfg.out, "%s: refresh failed: HTTP %d\n", txtPath, resp.StatusCode)
		return false
	}
	fmt.Fprintf(cfg.out, "%s: refresh ok\n", txtPath)
	return true
}
