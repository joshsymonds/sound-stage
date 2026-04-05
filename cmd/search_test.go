package cmd

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/joshsymonds/sound-stage/usdb"
)

func TestOutputJSON(t *testing.T) {
	// Not parallel: mutates package-level osStdout.

	songs := []usdb.Song{
		{ID: 1, Artist: "Queen", Title: "Bohemian Rhapsody", Language: "English"},
		{ID: 2, Artist: "Daft Punk", Title: "Get Lucky", Language: "English"},
	}

	var buf bytes.Buffer
	oldStdout := osStdout
	osStdout = &buf
	t.Cleanup(func() { osStdout = oldStdout })

	if err := outputJSON(songs); err != nil {
		t.Fatalf("outputJSON: %v", err)
	}

	// Verify the output is valid JSON
	var result []usdb.Song
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("outputJSON produced invalid JSON: %v\nOutput: %s", err, buf.String())
	}

	if len(result) != 2 {
		t.Fatalf("expected 2 songs, got %d", len(result))
	}
	if result[0].ID != 1 || result[0].Artist != "Queen" {
		t.Errorf("first song = %+v, want ID=1 Artist=Queen", result[0])
	}
	if result[1].ID != 2 || result[1].Title != "Get Lucky" {
		t.Errorf("second song = %+v, want ID=2 Title=Get Lucky", result[1])
	}
}
