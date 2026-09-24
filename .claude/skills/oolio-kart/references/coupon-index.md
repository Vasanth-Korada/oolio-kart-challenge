# Coupon index

## The rule

A code is valid when it is 8 to 10 bytes long **and** appears in at least 2 of
the 3 source files (`couponbase1/2/3.gz`, public S3, ~2.1 GB compressed).

| File | Lines | Unique 8-10 byte codes |
| --- | --- | --- |
| couponbase1.gz | 107,260,777 | 107,258,700 |
| couponbase2.gz | 107,260,776 | 107,260,726 |
| couponbase3.gz | 98,566,152 | 98,566,151 |

Result: **8 valid codes**, index of **96 bytes**. `HAPPYHRS` and `FIFTYOFF` are
valid, `SUPER100` is not (`index_real_data_test.go`). Don't write the other valid
codes into docs or tests; the committed index is the source of truth.

## Files (`internal/coupon`)

| File | Holds |
| --- | --- |
| `validator.go` | `Validator` interface, `ValidLength`, `unavailableValidator` (fail closed) |
| `key.go` | `codeKey [11]byte` = length byte + code zero-padded; `makeKey`, `compareKeys` |
| `index.go` | `Index` (sorted keys), `IsValid` = length check → `makeKey` → `sort.Search` → `==` |
| `source.go` | `Source` interface, `GzipFileSource`, `ErrPartialRead` |
| `build.go` | `BuildIndex(paths)` wraps paths in `GzipFileSource` → `Build(sources)`; `collectKeys`, `dedupeSorted`, `mergeAtLeastN`, `writeIndexFile` |
| `format.go` | `CPX2` layout, `writeIndex(io.Writer)`, `parseIndex`, `LoadIndex` |

Interfaces exist only at real seams: `Validator` (what `order.Service` needs),
`Source` (where codes come from; tests use an in-memory `fakeSource`), and
`io.Writer`. Don't add interfaces for single-implementation types.

## Build pipeline

1. One goroutine per source (`errgroup`); each writes only its own slot, so no
   lock. Loop variables are per-iteration (Go 1.22+), so capturing them is safe.
2. `Scan` streams gzip → `bufio.Scanner` (64 KB buffer, 1 MB max line), keeps
   lines passing `ValidLength`.
3. Each code → 11-byte key; `slices.SortFunc(keys, compareKeys)`; `dedupeSorted`.
4. `mergeAtLeastN(sets, 2)`: k-way merge with one cursor per sorted set; each
   round takes the smallest head, counts equal heads, advances them, keeps the key
   if count ≥ 2. Output comes out sorted, ready for binary search.
5. `writeIndex`: header `"CPX2"` + big-endian `uint32` count, then 11 bytes per key.

`LoadIndex` rejects a wrong magic (including old `CPX1`) or a size that isn't
exactly `8 + 11·count`.

## Things that are subtle (and were gotten wrong before)

- **What dedupe really does.** The merge advances each set at most one step per
  round, so a code repeated inside one file never counts twice: the merge itself
  enforces "files, not lines". Dedupe keeps output keys unique (X twice in two
  files would otherwise be emitted twice) and shrinks memory. A `map[code]++`
  design would need dedupe for correctness; this one doesn't.
- **Why the length byte.** Without it, `"ABCDEFGH"` and `"ABCDEFGH\x00"` pad to
  the same key. `TestMixedLengthsAndPrefixes` fails if it is removed. Keys sort by
  length first, so a 9-char code sorts after every 8-char code.
- **Truncated input.** On a read error `bufio.Scanner` returns the buffered bytes
  as a final token: a real line cut short. An 8-10 byte fragment would pass
  `ValidLength` and be indexed. `GzipFileSource.Scan` holds each line until the
  next reads cleanly and drops the held one on error; the build logs a warning and
  keeps every complete line (`ErrPartialRead`).
- **Plain text.** v1.1.0 stores raw codes, so `strings coupons.idx` shows the
  valid codes. Accepted trade-off (the source files are public); production would
  build the index in CI from a private source.

## History

- v1.0.0 (`main`): SHA-256 truncated to 16 bytes, format `CPX1`, 136 bytes. The
  old README's "64-bit collision odds ~1-in-4000" figure is wrong; cross-file
  collision risk at 64 bits is about 0.2% (~1 in 500).
- v1.1.0 (`submission-v2`): raw 11-byte keys, `CPX2`, 96 bytes, exact matching.

## Numbers for design questions

- Build memory: ~11 bytes × ~107M keys ≈ 1.2 GB per file, three in parallel,
  plus `append` growth slack.
- Build time: ~5 minutes on the dev Mac when nothing else runs; much slower if
  tests or lint run at the same time (disk and CPU contention).
- Runtime: 8 × 11 bytes in memory; one lookup is a length check plus binary
  search, no I/O.

## Rebuilding

`make fetch-coupons` (resumable, checks `Content-Length`) then
`make build-coupon-index`. It overwrites `coupons/coupons.idx`; with unchanged
code the output must be byte-identical to the committed file (compare with
`md5` or `cmp`). Raw files live in `coupons/raw/` and are gitignored.
