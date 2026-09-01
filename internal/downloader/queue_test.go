package downloader

import (
	"net/http"
	"testing"
)

func TestResolveFilename(t *testing.T) {
	tests := []struct {
		name     string
		rawName  string
		headerCD string
		expected string
	}{
		{
			name:     "Combined filename and filename* header",
			rawName:  "default.zip",
			headerCD: `attachment; filename="Kittew_2025-08_Pack_50_SpecialKiss.zip"; filename*=UTF-8''Kittew_2025-08_Pack_50_SpecialKiss.zip`,
			expected: "Kittew_2025-08_Pack_50_SpecialKiss.zip",
		},
		{
			name:     "Simple quoted filename",
			rawName:  "default.mp4",
			headerCD: `attachment; filename="my_movie_2026.mp4"`,
			expected: "my_movie_2026.mp4",
		},
		{
			name:     "URL encoded filename*",
			rawName:  "default.mkv",
			headerCD: `attachment; filename*=UTF-8''my%20cool%20video.mkv`,
			expected: "my cool video.mkv",
		},
		{
			name:     "Unquoted filename with params",
			rawName:  "default.bin",
			headerCD: `attachment; filename=file_name_123.bin; size=1024`,
			expected: "file_name_123.bin",
		},
		{
			name:     "No header fallback to raw name",
			rawName:  "fallback_archive.tar.gz",
			headerCD: "",
			expected: "fallback_archive.tar.gz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{
				Header: make(http.Header),
			}
			if tt.headerCD != "" {
				resp.Header.Set("Content-Disposition", tt.headerCD)
			}
			got := resolveFilename(tt.rawName, "task1", resp)
			if got != tt.expected {
				t.Errorf("resolveFilename() = %q, want %q", got, tt.expected)
			}
		})
	}
}
