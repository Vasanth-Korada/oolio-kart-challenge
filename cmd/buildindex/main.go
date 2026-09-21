// Command buildindex turns Oolio's three raw coupon files into the
// compact coupons.idx used by the running server.
package main

import (
	"flag"
	"log/slog"
	"os"
	"time"

	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/coupon"
)

func main() {
	var (
		out   = flag.String("out", "coupons/coupons.idx", "output path for the built index")
		file1 = flag.String("file1", "coupons/raw/couponbase1.gz", "path to couponbase1.gz")
		file2 = flag.String("file2", "coupons/raw/couponbase2.gz", "path to couponbase2.gz")
		file3 = flag.String("file3", "coupons/raw/couponbase3.gz", "path to couponbase3.gz")
	)
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	paths := []string{*file1, *file2, *file3}

	logger.Info("buildindex: starting", slog.Any("sources", paths), slog.String("out", *out))
	start := time.Now()

	stats, err := coupon.BuildIndex(paths, *out, logger)
	if err != nil {
		logger.Error("buildindex: failed", slog.Any("error", err))
		os.Exit(1)
	}

	elapsed := time.Since(start)
	logger.Info("buildindex: complete",
		slog.Int64("total_lines_scanned", stats.TotalLines),
		slog.Any("per_file_lines", stats.PerFileLines),
		slog.Any("per_file_unique_candidates", stats.PerFileCandidates),
		slog.Int("valid_codes", stats.ValidCodes),
		slog.Int64("index_bytes", stats.IndexBytes),
		slog.Duration("elapsed", elapsed),
	)
}
