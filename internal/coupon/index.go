package coupon

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"sort"

	"golang.org/x/sync/errgroup"
)

const requiredFileCount = 2
const indexMagic = "CPX1"

type key128 [16]byte

func hashCode(code string) key128 {
	sum := sha256.Sum256([]byte(code))
	var k key128
	copy(k[:], sum[:16])
	return k
}

type Index struct {
	keys []key128
}

func (idx *Index) IsValid(code string) bool {
	if !ValidLength(code) {
		return false
	}
	k := hashCode(code)
	i := sort.Search(len(idx.keys), func(i int) bool {
		return bytes.Compare(idx.keys[i][:], k[:]) >= 0
	})
	return i < len(idx.keys) && idx.keys[i] == k
}

func (idx *Index) Len() int { return len(idx.keys) }

var _ Validator = (*Index)(nil)

type Stats struct {
	PerFileLines      []int64
	PerFileCandidates []int
	TotalLines        int64
	ValidCodes        int
	IndexBytes        int64
}

// Each file's candidates are hashed, sorted, and deduplicated into
// their own slice and merged, rather than accumulated into one shared
// map: at ~3*10^8 lines a map's per-entry overhead costs tens of GB.
func BuildIndex(paths []string, outPath string, logger *slog.Logger) (Stats, error) {
	if len(paths) < requiredFileCount {
		return Stats{}, fmt.Errorf("coupon: need at least %d source files, got %d", requiredFileCount, len(paths))
	}
	if len(paths) > 8 {
		return Stats{}, fmt.Errorf("coupon: at most 8 source files supported, got %d", len(paths))
	}

	stats := Stats{
		PerFileLines:      make([]int64, len(paths)),
		PerFileCandidates: make([]int, len(paths)),
	}
	sets := make([][]key128, len(paths))
	lineCounts := make([]int64, len(paths))

	// Each file is independent until the merge below, so they're hashed
	// and sorted concurrently. Every goroutine owns a distinct index
	// into sets/lineCounts, no shared state, no lock needed.
	g := new(errgroup.Group)
	for i, path := range paths {
		g.Go(func() error {
			hashes, lines, err := collectFileHashes(logger, i, path)
			if err != nil {
				return err
			}
			sets[i] = hashes
			lineCounts[i] = lines
			logger.Info("coupon: indexed source file",
				slog.Int("file_index", i),
				slog.String("path", path),
				slog.Int64("lines", lines),
				slog.Int("unique_candidates", len(hashes)),
			)
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return Stats{}, err
	}

	for i := range paths {
		stats.PerFileLines[i] = lineCounts[i]
		stats.PerFileCandidates[i] = len(sets[i])
		stats.TotalLines += lineCounts[i]
	}

	valid := mergeAtLeastN(sets, requiredFileCount)
	stats.ValidCodes = len(valid)

	if err := os.MkdirAll(filepath.Dir(outPath), 0o750); err != nil {
		return Stats{}, fmt.Errorf("coupon: mkdir for %s: %w", outPath, err)
	}
	f, err := os.Create(outPath) //nolint:gosec // outPath is an operator-supplied CLI flag (cmd/buildindex), not user input
	if err != nil {
		return Stats{}, fmt.Errorf("coupon: create %s: %w", outPath, err)
	}
	defer func() { _ = f.Close() }() // safety net for early returns; the happy path closes and checks explicitly below

	if err := writeIndex(f, valid); err != nil {
		return Stats{}, fmt.Errorf("coupon: write index: %w", err)
	}
	if info, err := f.Stat(); err == nil {
		stats.IndexBytes = info.Size()
	}
	if err := f.Close(); err != nil {
		return Stats{}, fmt.Errorf("coupon: close %s: %w", outPath, err)
	}
	return stats, nil
}

func collectFileHashes(logger *slog.Logger, fileIndex int, path string) ([]key128, int64, error) {
	var hashes []key128
	lines, err := scanFile(logger, fileIndex, path, func(code string) {
		hashes = append(hashes, hashCode(code))
	})
	if err != nil {
		return nil, 0, err
	}

	sort.Slice(hashes, func(i, j int) bool { return bytes.Compare(hashes[i][:], hashes[j][:]) < 0 })
	return dedupeSorted(hashes), lines, nil
}

func dedupeSorted(s []key128) []key128 {
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

// A truncated or corrupted tail logs a warning and keeps what was read
// cleanly instead of failing the whole build.
func scanFile(logger *slog.Logger, fileIndex int, path string, onCandidate func(code string)) (int64, error) {
	f, err := os.Open(path) //nolint:gosec // path is an operator-supplied CLI flag (cmd/buildindex), not user input
	if err != nil {
		return 0, fmt.Errorf("coupon: open %s: %w", path, err)
	}
	defer func() { _ = f.Close() }() // read-only handle; nothing buffered to lose on close error

	gz, err := gzip.NewReader(f)
	if err != nil {
		return 0, fmt.Errorf("coupon: gzip reader for %s: %w", path, err)
	}
	defer func() { _ = gz.Close() }()

	scanner := bufio.NewScanner(gz)
	scanner.Buffer(make([]byte, 64*1024), 1<<20)

	var lines int64
	for scanner.Scan() {
		line := scanner.Text()
		lines++
		if ValidLength(line) {
			onCandidate(line)
		}
	}
	if serr := scanner.Err(); serr != nil {
		logger.Warn("coupon: source file ended early, continuing with partial data",
			slog.Int("file_index", fileIndex),
			slog.String("path", path),
			slog.Int64("lines_read", lines),
			slog.Any("error", serr),
		)
	}
	return lines, nil
}

func mergeAtLeastN(sets [][]key128, n int) []key128 {
	idxs := make([]int, len(sets))
	var result []key128

	for {
		var min key128
		minSet := -1
		for i, s := range sets {
			if idxs[i] >= len(s) {
				continue
			}
			if minSet == -1 || bytes.Compare(s[idxs[i]][:], min[:]) < 0 {
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

func writeIndex(w *os.File, keys []key128) error {
	if len(keys) > math.MaxUint32 {
		return fmt.Errorf("coupon: %d valid codes exceeds the uint32 index header limit", len(keys))
	}

	var header [8]byte
	copy(header[0:4], indexMagic)
	binary.BigEndian.PutUint32(header[4:8], uint32(len(keys))) //nolint:gosec // bounds-checked above
	if _, err := w.Write(header[:]); err != nil {
		return err
	}
	bw := bufio.NewWriter(w)
	for _, k := range keys {
		if _, err := bw.Write(k[:]); err != nil {
			return err
		}
	}
	return bw.Flush()
}

func LoadIndex(path string) (*Index, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path is server config (COUPON_INDEX_PATH), not user input
	if err != nil {
		return nil, fmt.Errorf("coupon: read index %s: %w", path, err)
	}
	if len(data) < 8 || string(data[0:4]) != indexMagic {
		return nil, fmt.Errorf("coupon: %s is not a valid coupon index file", path)
	}
	count := binary.BigEndian.Uint32(data[4:8])
	want := 8 + int(count)*16
	if len(data) != want {
		return nil, fmt.Errorf("coupon: index %s is corrupt: size %d, expected %d for %d entries", path, len(data), want, count)
	}

	keys := make([]key128, count)
	for i := range keys {
		off := 8 + i*16
		copy(keys[i][:], data[off:off+16])
	}
	return &Index{keys: keys}, nil
}
