package stableid

import "testing"

// Test vectors derived by computing:
//
//	md5(asciiLower(asciiTrim(artist)) + "\x00" +
//	    asciiLower(asciiTrim(title))  + "\x00" +
//	    (duet ? "1" : "0"))[0:16]
//
// where asciiLower folds only bytes 'A'..'Z' to 'a'..'z' (no Unicode folding),
// and asciiTrim strips only bytes < 0x21 (no Unicode whitespace like NBSP).
//
// Each pinned hex was computed in Python as:
//
//	hashlib.md5(a + b'\0' + t + b'\0' + (b'1' if duet else b'0')).hexdigest()[:16]
//
// with a and t already normalized via the ASCII-only helpers above.
func TestCompute(t *testing.T) {
	tests := []struct {
		name   string
		artist string
		title  string
		duet   bool
		want   string
		reason string
	}{
		{
			name:   "basic",
			artist: "ABBA",
			title:  "Dancing Queen",
			duet:   false,
			want:   "c06f1e4c7ac6a67e",
			reason: "canonical baseline vector",
		},
		{
			name:   "duet_true_differs",
			artist: "ABBA",
			title:  "Dancing Queen",
			duet:   true,
			want:   "1c4b529e65c90c35",
			reason: "duet flag must change the hash (appends '1' vs '0')",
		},
		{
			name:   "ascii_case_insensitive_lower",
			artist: "abba",
			title:  "dancing queen",
			duet:   false,
			want:   "c06f1e4c7ac6a67e",
			reason: "ASCII letters fold to lowercase; matches basic",
		},
		{
			name:   "ascii_case_insensitive_mixed",
			artist: "AbBa",
			title:  "DaNcInG qUeEn",
			duet:   false,
			want:   "c06f1e4c7ac6a67e",
			reason: "mixed-case ASCII folds to lowercase; matches basic",
		},
		{
			name:   "ascii_whitespace_trimmed",
			artist: "  ABBA\t",
			title:  "Dancing Queen ",
			duet:   false,
			want:   "c06f1e4c7ac6a67e",
			reason: "leading/trailing ASCII whitespace (space, tab) stripped; matches basic",
		},
		{
			name:   "non_ascii_upper_preserved",
			artist: "\u00d6mer", // "Ömer" — U+00D6 (UTF-8: 0xC3 0x96)
			title:  "X",
			duet:   false,
			want:   "07fe983d01cad1bb",
			reason: "non-ASCII uppercase (Ö) does NOT fold; must differ from lowercase variant",
		},
		{
			name:   "non_ascii_lower_distinct",
			artist: "\u00f6mer", // "ömer" — U+00F6 (UTF-8: 0xC3 0xB6)
			title:  "X",
			duet:   false,
			want:   "a2df4c341cc1d226",
			reason: "Ö and ö differ in their second UTF-8 byte; ASCII folding doesn't touch either, so hashes must differ",
		},
		{
			name:   "nbsp_not_trimmed",
			artist: "\u00a0ABBA\u00a0", // U+00A0 NBSP (UTF-8: 0xC2 0xA0)
			title:  "Dancing Queen",
			duet:   false,
			want:   "bae84adfb799c88a",
			reason: "NBSP (0xC2 0xA0) must NOT be trimmed; differs from basic. This test fails if impl uses strings.TrimSpace (which DOES trim NBSP)",
		},
		{
			name:   "empty",
			artist: "",
			title:  "",
			duet:   false,
			want:   "f36a7035d685f7a5",
			reason: "empty inputs produce a well-defined hash of \"\\x00\\x000\"",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Compute(tc.artist, tc.title, tc.duet)
			if got != tc.want {
				t.Errorf("Compute(%q, %q, %v) = %q, want %q\n  reason: %s",
					tc.artist, tc.title, tc.duet, got, tc.want, tc.reason)
			}
		})
	}
}

func TestComputeShape(t *testing.T) {
	got := Compute("anything", "anything", false)
	if len(got) != 16 {
		t.Errorf("Compute returned length %d, want 16", len(got))
	}
	for i, r := range got {
		isHex := (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')
		if !isHex {
			t.Errorf("Compute returned non-lowercase-hex char %q at index %d", r, i)
		}
	}
}
