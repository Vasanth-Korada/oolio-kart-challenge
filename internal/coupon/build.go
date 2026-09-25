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
	for fileIndex, path := range paths {
		sources[fileIndex] = GzipFileSource{Path: path}
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

	keysPerFile := make([][]codeKey, len(sources))
	lineCounts := make([]int64, len(sources))

	// Sources are independent until the merge below, so they're read and
	// sorted concurrently. Every goroutine owns a distinct fileIndex into
	// keysPerFile/lineCounts, no shared state, no lock needed.
	readers := new(errgroup.Group)
	for fileIndex, source := range sources {
		readers.Go(func() error {
			keys, lines, err := collectKeys(source)
			if errors.Is(err, ErrPartialRead) {
				logger.Warn("coupon: source file ended early, continuing with partial data",
					slog.Int("file_index", fileIndex),
					slog.String("path", source.Name()),
					slog.Int64("lines_read", lines),
					slog.Any("error", err),
				)
			} else if err != nil {
				return err
			}
			keysPerFile[fileIndex] = keys
			lineCounts[fileIndex] = lines
			logger.Info("coupon: indexed source file",
				slog.Int("file_index", fileIndex),
				slog.String("path", source.Name()),
				slog.Int64("lines", lines),
				slog.Int("unique_candidates", len(keys)),
			)
			return nil
		})
	}
	if err := readers.Wait(); err != nil {
		return Stats{}, err
	}

	stats := Stats{
		PerFileLines:      lineCounts,
		PerFileCandidates: make([]int, len(sources)),
	}
	for fileIndex := range sources {
		stats.PerFileCandidates[fileIndex] = len(keysPerFile[fileIndex])
		stats.TotalLines += lineCounts[fileIndex]
	}

	valid := mergeAtLeastN(keysPerFile, requiredFileCount)
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
func collectKeys(source Source) ([]codeKey, int64, error) {
	var keys []codeKey
	lines, err := source.Scan(func(code string) {
		keys = append(keys, makeKey(code))
	})
	if err != nil && !errors.Is(err, ErrPartialRead) {
		return nil, 0, err
	}
	slices.SortFunc(keys, compareKeys)
	return dedupeSorted(keys), lines, err
}

func dedupeSorted(keys []codeKey) []codeKey {
	if len(keys) == 0 {
		return keys
	}
	lastUnique := 0
	for readAt := 1; readAt < len(keys); readAt++ {
		if keys[readAt] != keys[lastUnique] {
			lastUnique++
			keys[lastUnique] = keys[readAt]
		}
	}
	return keys[:lastUnique+1]
}

// mergeAtLeastN k-way merges each file's sorted, deduplicated keys and keeps
// every key present in at least minFiles of them. It advances each file's
// cursor at most one step per round, so filesWithKey counts files, not
// lines. The result comes out sorted, ready for binary search.
func mergeAtLeastN(keysPerFile [][]codeKey, minFiles int) []codeKey {
	cursors := make([]int, len(keysPerFile))
	var result []codeKey

	for {
		var smallest codeKey
		smallestFile := -1
		for fileIndex, keys := range keysPerFile {
			if cursors[fileIndex] >= len(keys) {
				continue
			}
			if smallestFile == -1 || compareKeys(keys[cursors[fileIndex]], smallest) < 0 {
				smallest = keys[cursors[fileIndex]]
				smallestFile = fileIndex
			}
		}
		if smallestFile == -1 {
			break
		}

		filesWithKey := 0
		for fileIndex, keys := range keysPerFile {
			if cursors[fileIndex] < len(keys) && keys[cursors[fileIndex]] == smallest {
				filesWithKey++
				cursors[fileIndex]++
			}
		}
		if filesWithKey >= minFiles {
			result = append(result, smallest)
		}
	}
	return result
}

func writeIndexFile(outPath string, keys []codeKey) (int64, error) {
	if err := os.MkdirAll(filepath.Dir(outPath), 0o750); err != nil {
		return 0, fmt.Errorf("coupon: mkdir for %s: %w", outPath, err)
	}
	file, err := os.Create(outPath) //nolint:gosec // outPath is an operator-supplied CLI flag (cmd/buildindex), not user input
	if err != nil {
		return 0, fmt.Errorf("coupon: create %s: %w", outPath, err)
	}
	defer func() { _ = file.Close() }() // safety net for early returns; the happy path closes and checks explicitly below

	if err := writeIndex(file, keys); err != nil {
		return 0, fmt.Errorf("coupon: write index: %w", err)
	}
	var size int64
	if info, err := file.Stat(); err == nil {
		size = info.Size()
	}
	if err := file.Close(); err != nil {
		return 0, fmt.Errorf("coupon: close %s: %w", outPath, err)
	}
	return size, nil
}
