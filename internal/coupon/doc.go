// Package coupon validates promo codes. A code is valid when it is 8 to 10
// characters long and appears in at least two of the source files.
//
// The work is split in two. BuildIndex runs offline and writes the valid codes
// to a small sorted index file. At runtime, LoadIndex reads that file into an
// Index, which answers IsValid with a binary search.
package coupon
