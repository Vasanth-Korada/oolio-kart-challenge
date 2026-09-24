package coupon_test

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/coupon"
)

// fakeSource is an in-memory coupon.Source: every entry in lines is one
// input line, and err (if set) is returned after all of them are read.
type fakeSource struct {
	lines []string
	err   error
}

func (f fakeSource) Name() string { return "fake" }

func (f fakeSource) Scan(onCode func(code string)) (int64, error) {
	for _, l := range f.lines {
		if coupon.ValidLength(l) {
			onCode(l)
		}
	}
	return int64(len(f.lines)), f.err
}

func src(lines ...string) fakeSource { return fakeSource{lines: lines} }

type BuildSuite struct {
	suite.Suite
	out  string
	logs *bytes.Buffer
}

func TestBuildSuite(t *testing.T) {
	suite.Run(t, new(BuildSuite))
}

func (s *BuildSuite) SetupTest() {
	s.out = filepath.Join(s.T().TempDir(), "out.idx")
	s.logs = &bytes.Buffer{}
}

func (s *BuildSuite) build(sources ...coupon.Source) (coupon.Stats, *coupon.Index) {
	stats, err := coupon.Build(sources, s.out, slog.New(slog.NewTextHandler(s.logs, nil)))
	s.Require().NoError(err)
	idx, err := coupon.LoadIndex(s.out)
	s.Require().NoError(err)
	return stats, idx
}

func (s *BuildSuite) assertValid(idx *coupon.Index, want map[string]bool) {
	for code, valid := range want {
		s.Equal(valid, idx.IsValid(code), "IsValid(%q)", code)
	}
}

func (s *BuildSuite) TestKeepsCodesFoundInAtLeastTwoSources() {
	_, idx := s.build(
		src("AAAAAAAA", "BBBBBBBB", "CCCCCCCC", "short", "toolongtoolongxx"),
		src("AAAAAAAA", "BBBBBBBB", "short", "toolongtoolongxx"),
		src("BBBBBBBB"),
	)

	s.Equal(2, idx.Len())
	s.assertValid(idx, map[string]bool{
		"AAAAAAAA": true,  // 2 sources
		"BBBBBBBB": true,  // 3 sources
		"CCCCCCCC": false, // 1 source
		"short":    false, // wrong length
	})
}

func (s *BuildSuite) TestMixedLengthsAndPrefixes() {
	_, idx := s.build(
		src("ABCDEFGH", "ABCDEFGHI", "ABCDEFGHIJ", "ZZZZZZZZ"),
		src("ABCDEFGH", "ABCDEFGHIJ", "ZZZZZZZZZ"),
	)

	s.assertValid(idx, map[string]bool{
		"ABCDEFGH":     true,
		"ABCDEFGHIJ":   true,
		"ABCDEFGHI":    false, // extends a valid code, one source only
		"ABCDEFGH\x00": false, // fails if the key's length byte is removed
		"ZZZZZZZZ":     false,
		"ZZZZZZZZZ":    false,
		"ABCDEFGHIJK":  false,
	})
}

func (s *BuildSuite) TestRepeatsInsideOneSourceDoNotCount() {
	stats, idx := s.build(src("XXXXXXXX", "XXXXXXXX"), src("YYYYYYYY"))

	s.Zero(stats.ValidCodes)
	s.False(idx.IsValid("XXXXXXXX"))
}

func (s *BuildSuite) TestRepeatsAcrossSourcesAreStoredOnce() {
	stats, idx := s.build(src("XXXXXXXX", "XXXXXXXX"), src("XXXXXXXX", "XXXXXXXX"))

	s.Equal(1, stats.ValidCodes)
	s.Equal(int64(8+11), stats.IndexBytes, "header plus exactly one key")
	s.True(idx.IsValid("XXXXXXXX"))
}

func (s *BuildSuite) TestStats() {
	stats, _ := s.build(
		src("AAAAAAAA", "short", "BBBBBBBB"),
		src("AAAAAAAA"),
	)

	s.Equal([]int64{3, 1}, stats.PerFileLines)
	s.Equal([]int{2, 1}, stats.PerFileCandidates)
	s.Equal(int64(4), stats.TotalLines)
	s.Equal(1, stats.ValidCodes)
	s.Equal(int64(8+11), stats.IndexBytes)
}

func (s *BuildSuite) TestPartialSourceKeepsWhatWasRead() {
	partial := fakeSource{
		lines: []string{"AAAAAAAA"},
		err:   fmt.Errorf("%w: unexpected EOF", coupon.ErrPartialRead),
	}

	_, idx := s.build(partial, src("AAAAAAAA"))

	s.True(idx.IsValid("AAAAAAAA"))
	s.Contains(s.logs.String(), "ended early")
}

func (s *BuildSuite) TestFailingSourceFailsTheBuild() {
	boom := errors.New("boom")

	_, err := coupon.Build([]coupon.Source{fakeSource{err: boom}, src("AAAAAAAA")}, s.out, slog.New(slog.NewTextHandler(s.logs, nil)))

	s.Require().ErrorIs(err, boom)
	s.NoFileExists(s.out)
}

func (s *BuildSuite) TestNeedsAtLeastTwoSources() {
	_, err := coupon.Build([]coupon.Source{src("AAAAAAAA")}, s.out, slog.New(slog.NewTextHandler(s.logs, nil)))

	s.Require().Error(err)
}

func (s *BuildSuite) TestNoOverlapWritesAnEmptyIndex() {
	stats, idx := s.build(src("AAAAAAAA"), src("BBBBBBBB"))

	s.Zero(stats.ValidCodes)
	s.Zero(idx.Len())
	s.False(idx.IsValid("AAAAAAAA"))
}

func (s *BuildSuite) TestBuildIndexReadsGzipFiles() {
	dir := s.T().TempDir()
	f1, f2 := filepath.Join(dir, "f1.gz"), filepath.Join(dir, "f2.gz")
	writeGzipLines(s.T(), f1, []string{"AAAAAAAA", "BBBBBBBB"})
	writeGzipLines(s.T(), f2, []string{"AAAAAAAA"})

	stats, err := coupon.BuildIndex([]string{f1, f2}, s.out, slog.New(slog.NewTextHandler(s.logs, nil)))
	s.Require().NoError(err)
	idx, err := coupon.LoadIndex(s.out)
	s.Require().NoError(err)

	s.Equal(1, stats.ValidCodes)
	s.True(idx.IsValid("AAAAAAAA"))
	s.False(idx.IsValid("BBBBBBBB"))
}
