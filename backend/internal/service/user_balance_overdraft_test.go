package service

import (
	"math"
	"testing"
)

func TestUserIsBalanceEligible(t *testing.T) {
	tests := []struct {
		name           string
		balance        float64
		limit          float64
		minimumReserve float64
		want           bool
	}{
		{name: "legacy positive balance", balance: 0.01, want: true},
		{name: "legacy zero balance", balance: 0, want: false},
		{name: "legacy negative balance", balance: -0.01, want: false},
		{name: "at minimum reserve", balance: 0.01, minimumReserve: 0.01, want: true},
		{name: "below minimum reserve", balance: 0.009, minimumReserve: 0.01, want: false},
		{name: "inside overdraft", balance: -80, limit: 100, want: true},
		{name: "at overdraft limit", balance: -100, limit: 100, want: false},
		{name: "beyond overdraft limit", balance: -101, limit: 100, want: false},
		{name: "overdraft with reserve at boundary", balance: -99.99, limit: 100, minimumReserve: 0.01, want: true},
		{name: "overdraft with reserve below boundary", balance: -99.991, limit: 100, minimumReserve: 0.01, want: false},
		{name: "invalid negative limit is zero", balance: -1, limit: -100, want: false},
		{name: "invalid nan limit is zero", balance: -1, limit: math.NaN(), want: false},
		{name: "invalid infinite limit is zero", balance: -1, limit: math.Inf(1), want: false},
		{name: "nan balance is rejected", balance: math.NaN(), limit: 100, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := &User{OverdraftLimit: tt.limit}
			if got := user.IsBalanceEligible(tt.balance, tt.minimumReserve); got != tt.want {
				t.Fatalf("IsBalanceEligible(%v, %v) = %v, want %v", tt.balance, tt.minimumReserve, got, tt.want)
			}
		})
	}
}
