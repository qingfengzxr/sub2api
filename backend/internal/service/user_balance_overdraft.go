package service

import "math"

// IsBalanceEligible reports whether a cached balance may start another request.
func (u *User) IsBalanceEligible(balance, minimumReserve float64) bool {
	limit := 0.0
	if u != nil && u.OverdraftLimit >= 0 && !math.IsNaN(u.OverdraftLimit) && !math.IsInf(u.OverdraftLimit, 0) {
		limit = u.OverdraftLimit
	}
	if minimumReserve > 0 && !math.IsNaN(minimumReserve) && !math.IsInf(minimumReserve, 0) {
		return balance >= minimumReserve-limit
	}
	return balance > -limit
}
