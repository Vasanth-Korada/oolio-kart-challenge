package coupon

import (
	"bufio"
	"compress/gzip"
	"errors"
	"fmt"
	"os"
)

// Source yields candidate codes from one input, e.g. one couponbaseN.gz
// file. The build treats every Source the same, so tests and future
// inputs (plain text, object storage) plug in without touching it.
type Source interface {
	Name() string
	// Scan calls onCode for every line of valid length and returns how
	// many lines it read. An error wrapping ErrPartialRead means the
	// input ended early; every line before that point was delivered.
	Scan(onCode func(code string)) (lines int64, err error)
}

// ErrPartialRead marks an input that was cut short (truncated or
// corrupted tail). The build logs it and keeps what was read cleanly.
var ErrPartialRead = errors.New("coupon: source ended early")

// GzipFileSource reads one gzip-compressed file of codes, one per line.
type GzipFileSource struct {
	Path string
}

var _ Source = GzipFileSource{}

func (s GzipFileSource) Name() string { return s.Path }

func (s GzipFileSource) Scan(onCode func(code string)) (int64, error) {
	f, err := os.Open(s.Path) //nolint:gosec // path is an operator-supplied CLI flag (cmd/buildindex), not user input
	if err != nil {
		return 0, fmt.Errorf("coupon: open %s: %w", s.Path, err)
	}
	defer func() { _ = f.Close() }() // read-only handle; nothing buffered to lose on close error

	gz, err := gzip.NewReader(f)
	if err != nil {
		return 0, fmt.Errorf("coupon: gzip reader for %s: %w", s.Path, err)
	}
	defer func() { _ = gz.Close() }()

	scanner := bufio.NewScanner(gz)
	scanner.Buffer(make([]byte, 64*1024), 1<<20)

	var lines int64
	for scanner.Scan() {
		line := scanner.Text()
		lines++
		if ValidLength(line) {
			onCode(line)
		}
	}
	if err := scanner.Err(); err != nil {
		return lines, fmt.Errorf("%w: %s: %w", ErrPartialRead, s.Path, err)
	}
	return lines, nil
}
