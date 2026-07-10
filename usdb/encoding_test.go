package usdb

import "testing"

func TestDecodeUSDB(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		body []byte
		want string
	}{
		{
			name: "valid utf-8 passes through",
			body: []byte("Beyoncé — A★Teens"),
			want: "Beyoncé — A★Teens",
		},
		{
			name: "cp1252 latin letters decode",
			body: []byte{'C', 0xE9, 'l', 'i', 'n', 'e'}, // Céline in CP1252
			want: "Céline",
		},
		{
			name: "cp1252 punctuation range decodes",
			body: []byte{0x93, 'H', 'i', 0x94, ' ', 0x96, ' ', 0x80}, // “Hi” – €
			want: "“Hi” – €",
		},
		{
			name: "utf-8 BOM stripped",
			body: []byte("\xEF\xBB\xBF#ARTIST:ABBA"),
			want: "#ARTIST:ABBA",
		},
		{
			name: "pure ascii unchanged",
			body: []byte("#ARTIST:Queen"),
			want: "#ARTIST:Queen",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := decodeUSDB(tt.body); got != tt.want {
				t.Errorf("decodeUSDB(%q) = %q, want %q", tt.body, got, tt.want)
			}
		})
	}
}
