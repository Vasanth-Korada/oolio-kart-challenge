package coupon_test

import (
	"compress/gzip"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/coupon"
)

func writeGzipLines(t *testing.T, path string, lines []string) {
	t.Helper()
	f, err := os.Create(path) //nolint:gosec // path is t.TempDir()-derived, test-only
	if err != nil {
		t.Fatalf("create %s: %v", path, err)
	}
	defer func() { _ = f.Close() }()

	gw := gzip.NewWriter(f)
	defer func() { _ = gw.Close() }()
	for _, l := range lines {
		if _, err := io.WriteString(gw, l+"\n"); err != nil {
			t.Fatalf("write line: %v", err)
		}
	}
}

func TestBuildIndex_SyntheticFixture(t *testing.T) {
	dir := t.TempDir()
	file1 := filepath.Join(dir, "f1.gz")
	file2 := filepath.Join(dir, "f2.gz")
	file3 := filepath.Join(dir, "f3.gz")
	out := filepath.Join(dir, "out.idx")

	writeGzipLines(t, file1, []string{"AAAAAAAA", "BBBBBBBB", "CCCCCCCC", "short", "toolongtoolongxx"})
	writeGzipLines(t, file2, []string{"AAAAAAAA", "BBBBBBBB", "short", "toolongtoolongxx"})
	writeGzipLines(t, file3, []string{"BBBBBBBB"})

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stats, err := coupon.BuildIndex([]string{file1, file2, file3}, out, logger)
	if err != nil {
		t.Fatalf("BuildIndex: %v", err)
	}
	if stats.ValidCodes != 2 {
		t.Fatalf("ValidCodes = %d, want 2", stats.ValidCodes)
	}

	idx, err := coupon.LoadIndex(out)
	if err != nil {
		t.Fatalf("LoadIndex: %v", err)
	}
	cases := map[string]bool{
		"AAAAAAAA": true,
		"BBBBBBBB": true,
		"CCCCCCCC": false,
		"short":    false,
	}
	for code, want := range cases {
		t.Run(code, func(t *testing.T) {
			if got := idx.IsValid(code); got != want {
				t.Errorf("IsValid(%q) = %v, want %v", code, got, want)
			}
		})
	}
}

func TestBuildIndex_TooFewFiles(t *testing.T) {
	dir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	_, err := coupon.BuildIndex([]string{filepath.Join(dir, "only-one.gz")}, filepath.Join(dir, "out.idx"), logger)
	if err == nil {
		t.Fatal("expected an error for fewer than requiredFileCount source files")
	}
}

func TestBuildIndex_MixedLengthsAndPrefixes(t *testing.T) {
	dir := t.TempDir()
	file1 := filepath.Join(dir, "f1.gz")
	file2 := filepath.Join(dir, "f2.gz")
	out := filepath.Join(dir, "out.idx")

	writeGzipLines(t, file1, []string{"ABCDEFGH", "ABCDEFGHI", "ABCDEFGHIJ", "ZZZZZZZZ"})
	writeGzipLines(t, file2, []string{"ABCDEFGH", "ABCDEFGHIJ", "ZZZZZZZZZ"})

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	if _, err := coupon.BuildIndex([]string{file1, file2}, out, logger); err != nil {
		t.Fatalf("BuildIndex: %v", err)
	}
	idx, err := coupon.LoadIndex(out)
	if err != nil {
		t.Fatalf("LoadIndex: %v", err)
	}

	tests := []struct {
		name string
		code string
		want bool
	}{
		{name: "8 chars in both files", code: "ABCDEFGH", want: true},
		{name: "10 chars in both files", code: "ABCDEFGHIJ", want: true},
		{name: "9-char extension of a valid code, one file only", code: "ABCDEFGHI", want: false},
		{name: "valid code plus a NUL byte is a different code", code: "ABCDEFGH\x00", want: false},
		{name: "8 chars, one file only", code: "ZZZZZZZZ", want: false},
		{name: "9 chars, other file only", code: "ZZZZZZZZZ", want: false},
		{name: "11 chars", code: "ABCDEFGHIJK", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := idx.IsValid(tt.code); got != tt.want {
				t.Errorf("IsValid(%q) = %v, want %v", tt.code, got, tt.want)
			}
		})
	}
}

func TestLoadIndex_RejectsBadFiles(t *testing.T) {
	header := func(magic string, count byte) []byte {
		return []byte{magic[0], magic[1], magic[2], magic[3], 0, 0, 0, count}
	}
	tests := []struct {
		name string
		data []byte
	}{
		{name: "old CPX1 format", data: append(header("CPX1", 1), make([]byte, 16)...)},
		{name: "size does not match count", data: append(header("CPX2", 2), make([]byte, 11)...)},
		{name: "shorter than the header", data: []byte("CPX2")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "bad.idx")
			if err := os.WriteFile(path, tt.data, 0o600); err != nil {
				t.Fatalf("write: %v", err)
			}
			if _, err := coupon.LoadIndex(path); err == nil {
				t.Fatal("LoadIndex succeeded, want an error")
			}
		})
	}
}
