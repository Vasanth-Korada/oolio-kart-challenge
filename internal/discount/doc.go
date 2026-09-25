// Package discount decides how much a valid coupon takes off an order. The
// coupon index decides whether a code is valid; this package only prices it,
// from the config file: a default percent for every valid code, plus
// optional per-code percents. Policy implements order.DiscountPolicy.
package discount
