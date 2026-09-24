package coupon

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"

	"golang.org/x/sync/errgroup"
)

// A code is valid when it appears in at least this many sources.
const requiredFileCount = 2

// Stats reports what a build read and wrote, for logging.
type Stats struct {
	PerFileLines      []int64
	PerFileCandidates []int
	TotalLines        int64
	ValidCodes        int
	IndexBytes        int64
}

// BuildIndex builds the index from gzip files on disk; see Build.
func BuildIndex(paths []string, outPath string, logger *slog.Logger) (Stats, error) {
	sources := make([]Source, len(paths))
	for i, p := range paths {
		sources[i] = GzipFileSource{Path: p}
	}
	return Build(sources, outPath, logger)
}

// Build reads every source, keeps the codes found in at least
// requiredFileCount of them, and writes the sorted result to outPath.
//
// Each source's codes are sorted and deduplicated into their own slice
// and merged, rather than accumulated into one shared map: at ~3*10^8
// lines a map's per-entry overhead costs tens of GB.
func Build(sources []Source, outPath string, logger *slog.Logger) (Stats, error) {
	if len(sources) < requiredFileCount {
		return Stats{}, fmt.Errorf("coupon: need at least %d source files, got %d", requiredFileCount, len(sources))
	}

	sets := make([][]codeKey, len(sources))
	lineCounts := make([]int64, len(sources))

	// Sources are independent until the merge below, so they're read and
	// sorted concurrently. Every goroutine owns a distinct index into
	// sets/lineCounts, no shared state, no lock needed.
	g := new(errgroup.Group)
	for i, src := range sources {
		g.Go(func() error {
			keys, lines, err := collectKeys(src)
			if errors.Is(err, ErrPartialRead) {
				logger.Warn("coupon: source file ended early, continuing with partial data",
					slog.Int("file_index", i),
					slog.String("path", src.Name()),
					slog.Int64("lines_read", lines),
					slog.Any("error", err),
				)
			} else if err != nil {
				return err
			}
			sets[i] = keys
			lineCounts[i] = lines
			logger.Info("coupon: indexed source file",
				slog.Int("file_index", i),
				slog.String("path", src.Name()),
				slog.Int64("lines", lines),
				slog.Int("unique_candidates", len(keys)),
			)
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return Stats{}, err
	}

	stats := Stats{
		PerFileLines:      lineCounts,
		PerFileCandidates: make([]int, len(sources)),
	}
	for i := range sources {
		stats.PerFileCandidates[i] = len(sets[i])
		stats.TotalLines += lineCounts[i]
	}

	valid := mergeAtLeastN(sets, requiredFileCount)
	stats.ValidCodes = len(valid)

	size, err := writeIndexFile(outPath, valid)
	if err != nil {
		return Stats{}, err
	}
	stats.IndexBytes = size
	return stats, nil
}

// collectKeys returns a source's candidate codes, sorted and deduplicated.
// On ErrPartialRead it still returns everything read before the error.
func collectKeys(src Source) ([]codeKey, int64, error) {
	var keys []codeKey
	lines, err := src.Scan(func(code string) {
		keys = append(keys, makeKey(code))
	})
	if err != nil && !errors.Is(err, ErrPartialRead) {
		return nil, 0, err
	}
	slices.SortFunc(keys, compareKeys)
	return dedupeSorted(keys), lines, err
}

func dedupeSorted(s []codeKey) []codeKey {
	if len(s) == 0 {
		return s
	}
	j := 0
	for i := 1; i < len(s); i++ {
		if s[i] != s[j] {
			j++
			s[j] = s[i]
		}
	}
	return s[:j+1]
}

// mergeAtLeastN k-way merges sorted, deduplicated sets and keeps each key
// present in at least n of them. It advances each set at most one step
// per round, so a key's count is the number of sets containing it. The
// result comes out sorted, ready for binary search.
func mergeAtLeastN(sets [][]codeKey, n int) []codeKey {
	idxs := make([]int, len(sets))
	var result []codeKey

	for {
		var min codeKey
		minSet := -1
		for i, s := range sets {
			if idxs[i] >= len(s) {
				continue
			}
			if minSet == -1 || compareKeys(s[idxs[i]], min) < 0 {
				min = s[idxs[i]]
				minSet = i
			}
		}
		if minSet == -1 {
			break
		}

		count := 0
		for i, s := range sets {
			if idxs[i] < len(s) && s[idxs[i]] == min {
				count++
				idxs[i]++
			}
		}
		if count >= n {
			result = append(result, min)
		}
	}
	return result
}

func writeIndexFile(outPath string, keys []codeKey) (int64, error) {
	if err := os.MkdirAll(filepath.Dir(outPath), 0o750); err != nil {
		return 0, fmt.Errorf("coupon: mkdir for %s: %w", outPath, err)
	}
	f, err := os.Create(outPath) //nolint:gosec // outPath is an operator-supplied CLI flag (cmd/buildindex), not user input
	if err != nil {
		return 0, fmt.Errorf("coupon: create %s: %w", outPath, err)
	}
	defer func() { _ = f.Close() }() // safety net for early returns; the happy path closes and checks explicitly below

	if err := writeIndex(f, keys); err != nil {
		return 0, fmt.Errorf("coupon: write index: %w", err)
	}
	var size int64
	if info, err := f.Stat(); err == nil {
		size = info.Size()
	}
	if err := f.Close(); err != nil {
		return 0, fmt.Errorf("coupon: close %s: %w", outPath, err)
	}
	return size, nil
}
