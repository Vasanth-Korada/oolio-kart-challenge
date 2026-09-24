package coupon_test

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/coupon"
)

type GzipFileSourceSuite struct {
	suite.Suite
	dir string
}

func TestGzipFileSourceSuite(t *testing.T) {
	suite.Run(t, new(GzipFileSourceSuite))
}

func (s *GzipFileSourceSuite) SetupTest() {
	s.dir = s.T().TempDir()
}

func (s *GzipFileSourceSuite) scan(path string) ([]string, int64, error) {
	var codes []string
	lines, err := coupon.GzipFileSource{Path: path}.Scan(func(code string) {
		codes = append(codes, code)
	})
	return codes, lines, err
}

func (s *GzipFileSourceSuite) TestCountsEveryLineButYieldsOnlyValidLengths() {
	path := filepath.Join(s.dir, "codes.gz")
	writeGzipLines(s.T(), path, []string{"AAAAAAAA", "short", "BBBBBBBBBB", "toolongtoolongxx"})

	codes, lines, err := s.scan(path)

	s.Require().NoError(err)
	s.Equal(int64(4), lines)
	s.Equal([]string{"AAAAAAAA", "BBBBBBBBBB"}, codes)
}

func (s *GzipFileSourceSuite) TestTruncatedFileIsAPartialRead() {
	const total = 5000
	lines := make([]string, total)
	for i := range lines {
		lines[i] = fmt.Sprintf("CODE%05d", i)
	}
	path := filepath.Join(s.dir, "truncated.gz")
	writeGzipLines(s.T(), path, lines)
	info, err := os.Stat(path)
	s.Require().NoError(err)
	s.Require().NoError(os.Truncate(path, info.Size()-32))

	codes, read, err := s.scan(path)

	s.Require().ErrorIs(err, coupon.ErrPartialRead)
	s.Positive(read, "lines before the cut should still be delivered")
	s.Less(read, int64(total))
	s.Equal(lines[:read], codes, "only complete, original lines are yielded")
}

// A file cut mid-line must not yield the cut-off fragment as a code:
// "BBBBBBBB" below is only the first 8 bytes of a 12-byte line, which
// would otherwise pass the length check and be indexed.
func (s *GzipFileSourceSuite) TestLineCutByTruncationIsDropped() {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	_, err := io.WriteString(gw, "AAAAAAAA\nBBBBBBBB")
	s.Require().NoError(err)
	s.Require().NoError(gw.Flush()) // everything up to here is decodable on its own
	cut := buf.Len()
	_, err = io.WriteString(gw, "BBBB\nCCCCCCCC\n")
	s.Require().NoError(err)
	s.Require().NoError(gw.Close())

	path := filepath.Join(s.dir, "cut.gz")
	s.Require().NoError(os.WriteFile(path, buf.Bytes()[:cut], 0o600))

	codes, lines, err := s.scan(path)

	s.Require().ErrorIs(err, coupon.ErrPartialRead)
	s.Equal([]string{"AAAAAAAA"}, codes)
	s.Equal(int64(1), lines, "only complete lines count")
}

func (s *GzipFileSourceSuite) TestLastLineWithoutNewlineIsKept() {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	_, err := io.WriteString(gw, "AAAAAAAA\nBBBBBBBB")
	s.Require().NoError(err)
	s.Require().NoError(gw.Close())
	path := filepath.Join(s.dir, "no-newline.gz")
	s.Require().NoError(os.WriteFile(path, buf.Bytes(), 0o600))

	codes, lines, err := s.scan(path)

	s.Require().NoError(err)
	s.Equal([]string{"AAAAAAAA", "BBBBBBBB"}, codes)
	s.Equal(int64(2), lines)
}

func (s *GzipFileSourceSuite) TestMissingFileIsAHardError() {
	_, _, err := s.scan(filepath.Join(s.dir, "missing.gz"))

	s.Require().ErrorIs(err, os.ErrNotExist)
	s.NotErrorIs(err, coupon.ErrPartialRead)
}

func (s *GzipFileSourceSuite) TestNonGzipFileIsAHardError() {
	path := filepath.Join(s.dir, "plain.txt")
	s.Require().NoError(os.WriteFile(path, []byte("AAAAAAAA\n"), 0o600))

	_, _, err := s.scan(path)

	s.Require().Error(err)
	s.NotErrorIs(err, coupon.ErrPartialRead)
}

func writeGzipLines(t *testing.T, path string, lines []string) {
	t.Helper()
	f, err := os.Create(path) //nolint:gosec // path is t.TempDir()-derived, test-only
	if err != nil {
		t.Fatalf("create %s: %v", path, err)
	}
	defer func() { _ = f.Close() }()

	gw := gzip.NewWriter(f)
	for _, l := range lines {
		if _, err := io.WriteString(gw, l+"\n"); err != nil {
			t.Fatalf("write line: %v", err)
		}
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("close gzip writer: %v", err)
	}
}
