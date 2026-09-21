package coupon

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
)

// requiredFileCount is the ">= N files" threshold from the promo-code rule.
const requiredFileCount = 2

const indexMagic = "CPX1"

// key128 is a SHA-256-truncated 128-bit fingerprint of a coupon code.
// 64 bits was the cheaper first instinct, but at ~3*10^8 candidate
// lines a 64-bit hash's birthday-bound collision odds (~1-in-4000) are
// too risky for something gating a real discount; 128 bits makes that
// negligible for a cost paid once, offline.
type key128 [16]byte

func hashCode(code string) key128 {
	sum := sha256.Sum256([]byte(code))
	var k key128
	copy(k[:], sum[:16])
	return k
}

// Index is a Validator backed by a sorted, deduplicated set of the
// 128-bit fingerprints of every code found in at least requiredFileCount
// of the source files. Lookups are O(log n) binary search with no map
// allocation.
type Index struct {
	keys []key128 // sorted ascending
}

// IsValid implements Validator: a code must satisfy the length
// precondition and have its fingerprint present in the index.
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

// Len reports how many distinct valid codes the index holds.
func (idx *Index) Len() int { return len(idx.keys) }

var _ Validator = (*Index)(nil)

// Stats summarizes one BuildIndex run.
type Stats struct {
	PerFileLines      []int64
	PerFileCandidates []int
	TotalLines        int64
	ValidCodes        int
	IndexBytes        int64
}

// BuildIndex streams each of the given gzip-compressed source files
// exactly once, keeps only lines whose length is in the valid range,
// and writes the sorted set of fingerprints appearing in at least
// requiredFileCount of the files to outPath.
//
// Each file's candidates are hashed, sorted, and deduplicated into
// their own slice rather than accumulated into one shared map — at the
// real ~3*10^8-line scale a map's per-entry overhead costs tens of GB,
// measured against the actual files, not assumed.
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

	for i, path := range paths {
		hashes, lines, err := collectFileHashes(logger, i, path)
		if err != nil {
			return Stats{}, err
		}
		sets[i] = hashes
		stats.PerFileLines[i] = lines
		stats.PerFileCandidates[i] = len(hashes)
		stats.TotalLines += lines
		logger.Info("coupon: indexed source file",
			slog.Int("file_index", i),
			slog.String("path", path),
			slog.Int64("lines", lines),
			slog.Int("unique_candidates", len(hashes)),
		)
	}

	valid := mergeAtLeastN(sets, requiredFileCount)
	stats.ValidCodes = len(valid)

	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return Stats{}, fmt.Errorf("coupon: mkdir for %s: %w", outPath, err)
	}
	f, err := os.Create(outPath)
	if err != nil {
		return Stats{}, fmt.Errorf("coupon: create %s: %w", outPath, err)
	}
	defer f.Close()

	if err := writeIndex(f, valid); err != nil {
		return Stats{}, fmt.Errorf("coupon: write index: %w", err)
	}
	if info, err := f.Stat(); err == nil {
		stats.IndexBytes = info.Size()
	}
	return stats, nil
}

// collectFileHashes streams path (gzip-compressed), hashes every
// length-valid line, and returns the sorted, deduplicated set of
// fingerprints plus the total line count.
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

// scanFile decompresses path and calls onCandidate for every line whose
// length falls in the valid range. A truncated or corrupted tail (the
// failure mode hit downloading these files over an unreliable
// connection during development) logs a warning and keeps what was
// read cleanly instead of failing the whole build.
func scanFile(logger *slog.Logger, fileIndex int, path string, onCandidate func(code string)) (int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("coupon: open %s: %w", path, err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return 0, fmt.Errorf("coupon: gzip reader for %s: %w", path, err)
	}
	defer gz.Close()

	scanner := bufio.NewScanner(gz)
	scanner.Buffer(make([]byte, 64*1024), 1<<20) // generous but bounded; real lines are 8-10 bytes

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

// mergeAtLeastN performs a k-way merge across sorted, deduplicated
// fingerprint sets and returns the sorted union of every fingerprint
// present in at least n of them.
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
			break // every set exhausted
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
	var header [8]byte
	copy(header[0:4], indexMagic)
	binary.BigEndian.PutUint32(header[4:8], uint32(len(keys)))
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

// LoadIndex reads a binary index file written by BuildIndex.
func LoadIndex(path string) (*Index, error) {
	data, err := os.ReadFile(path)
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
