package usdb

import (
	"strings"
	"unicode/utf8"
)

const (
	cp1252ExtrasStart = 0x80 // bottom of the range where CP1252 diverges from Latin-1
)

// decodeUSDB converts a raw USDB response body to a valid UTF-8 string.
// USDB usually serves UTF-8, but as a legacy PHP site it can emit
// Windows-1252. Invalid UTF-8 is re-decoded as Windows-1252 — a total
// decoding, every byte maps — so downstream code never writes invalid
// bytes into directory names or song.txt. That matters on the USDX side:
// its UTF-8 filesystem layer skips songs whose paths aren't valid UTF-8.
// A leading UTF-8 BOM is stripped either way.
func decodeUSDB(body []byte) string {
	decoded := string(body)
	if !utf8.ValidString(decoded) {
		decoded = decodeCP1252(body)
	}
	return strings.TrimPrefix(decoded, "\uFEFF")
}

// decodeCP1252 maps every byte of body to its Windows-1252 code point.
func decodeCP1252(body []byte) string {
	table := cp1252Table()
	var builder strings.Builder
	builder.Grow(len(body) + len(body)/8)
	for _, octet := range body {
		builder.WriteRune(table[octet])
	}
	return builder.String()
}

// cp1252Table returns the full byte→rune mapping for Windows-1252. Every
// byte matches its Latin-1 (identity) code point except 0x80–0x9F, which
// map to the CP1252-specific punctuation block (the five unassigned slots
// become U+FFFD).
func cp1252Table() [256]rune {
	extras := [...]rune{
		0x20AC, 0xFFFD, 0x201A, 0x0192, 0x201E, 0x2026, 0x2020, 0x2021,
		0x02C6, 0x2030, 0x0160, 0x2039, 0x0152, 0xFFFD, 0x017D, 0xFFFD,
		0xFFFD, 0x2018, 0x2019, 0x201C, 0x201D, 0x2022, 0x2013, 0x2014,
		0x02DC, 0x2122, 0x0161, 0x203A, 0x0153, 0xFFFD, 0x017E, 0x0178,
	}

	var table [256]rune
	for octet := range table {
		table[octet] = rune(octet)
	}
	for offset, mapped := range extras {
		table[cp1252ExtrasStart+offset] = mapped
	}
	return table
}
