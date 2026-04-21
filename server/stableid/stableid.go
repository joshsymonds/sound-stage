// Package stableid computes the 16-hex content hash USDX uses to identify
// songs. The formula is shared between USDX (Free Pascal) and SoundStage (Go);
// both sides must agree byte-for-byte or POST /queue will 404 on every song.
//
// Formula (per ~/Personal/sound-stage-usdx/docs/API.md):
//
//	id = lowercase(md5(
//	    asciiLower(asciiTrim(artist)) + "\0" +
//	    asciiLower(asciiTrim(title))  + "\0" +
//	    (duet ? "1" : "0")
//	)[0:16])
//
// The ASCII-only operations are load-bearing: Unicode case folding
// (strings.ToLower) or Unicode whitespace trim (strings.TrimSpace) would
// diverge from Free Pascal's behavior on non-ASCII input. We deliberately
// avoid importing the strings package to enforce this at review time.
package stableid

import (
	"crypto/md5" //nolint:gosec // not cryptographic; must match USDX's MD5 for ID parity
	"encoding/hex"
)

// Compute returns the 16-hex-character stable song ID for the given metadata.
// Matches USDX's identity formula byte-for-byte.
func Compute(artist, title string, duet bool) string {
	a := asciiLower(asciiTrim(artist))
	t := asciiLower(asciiTrim(title))

	buf := make([]byte, 0, len(a)+len(t)+3)
	buf = append(buf, a...)
	buf = append(buf, 0)
	buf = append(buf, t...)
	buf = append(buf, 0)
	if duet {
		buf = append(buf, '1')
	} else {
		buf = append(buf, '0')
	}

	sum := md5.Sum(buf) //nolint:gosec // not cryptographic; must match USDX's MD5 for ID parity
	return hex.EncodeToString(sum[:8])
}

// asciiLower folds 'A'..'Z' to 'a'..'z'. All other bytes pass through.
func asciiLower(input string) string {
	out := []byte(input)
	for i, c := range out {
		if c >= 'A' && c <= 'Z' {
			out[i] = c + 32
		}
	}
	return string(out)
}

// asciiTrim strips bytes < 0x21 from both ends of input. Matches Free Pascal's
// Trim (which strips bytes <= #32). Non-ASCII whitespace (e.g., NBSP) is left
// intact on purpose.
func asciiTrim(input string) string {
	start, end := 0, len(input)
	for start < end && input[start] < 0x21 {
		start++
	}
	for end > start && input[end-1] < 0x21 {
		end--
	}
	return input[start:end]
}
