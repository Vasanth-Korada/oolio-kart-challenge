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
	// many complete lines it read. An error wrapping ErrPartialRead means
	// the input ended early: every complete line before that point was
	// delivered, and a line cut off by the error was not.
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

	// Each line is held back until the next one is read. On a read error
	// the scanner still returns the bytes it had as a final "line", which
	// can be a real line cut short (the first 8 bytes of a 12-byte line
	// would pass ValidLength), so that last one is dropped, not indexed.
	var lines int64
	var pending string
	havePending := false
	emit := func() {
		lines++
		if ValidLength(pending) {
			onCode(pending)
		}
	}
	for scanner.Scan() {
		if havePending {
			emit()
		}
		pending, havePending = scanner.Text(), true
	}
	if err := scanner.Err(); err != nil {
		return lines, fmt.Errorf("%w: %s: %w", ErrPartialRead, s.Path, err)
	}
	if havePending {
		emit()
	}
	return lines, nil
}
