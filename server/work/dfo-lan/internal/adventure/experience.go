package adventure

import (
	"fmt"
	"math"
	"math/big"
	"strconv"
)

// ExperienceGain consumes the source float32's decimal value (0.3, not its
// binary approximation) without losing small kills or overflowing gain*rate.
// Old saves store tenths and have no scale field. A different source rate can
// change the denominator while retaining that existing fractional experience.
func (r *Rules) ExperienceGain(gain, remainder, scale uint64) (earned, nextRemainder, nextScale uint64, err error) {
	if r == nil || r.ExpRate <= 0 || math.IsNaN(float64(r.ExpRate)) || math.IsInf(float64(r.ExpRate), 0) {
		return 0, 0, 0, fmt.Errorf("冒险团经验比例无效")
	}
	if scale == 0 {
		scale = 10
	}
	if remainder >= scale {
		return 0, 0, 0, fmt.Errorf("冒险团经验余数无效")
	}
	rate, ok := new(big.Rat).SetString(strconv.FormatFloat(float64(r.ExpRate), 'f', -1, 32))
	if !ok || !rate.Denom().IsUint64() {
		return 0, 0, 0, fmt.Errorf("冒险团经验比例超出精度范围")
	}
	oldScale := new(big.Int).SetUint64(scale)
	gcd := new(big.Int).GCD(nil, nil, oldScale, rate.Denom())
	denominator := new(big.Int).Mul(new(big.Int).Quo(oldScale, gcd), rate.Denom())
	if !denominator.IsUint64() {
		return 0, 0, 0, fmt.Errorf("冒险团经验余数精度溢出")
	}
	total := new(big.Int).Mul(new(big.Int).SetUint64(gain), rate.Num())
	total.Mul(total, new(big.Int).Quo(new(big.Int).Set(denominator), rate.Denom()))
	carry := new(big.Int).Mul(new(big.Int).SetUint64(remainder), new(big.Int).Quo(new(big.Int).Set(denominator), oldScale))
	total.Add(total, carry)
	whole, fraction := new(big.Int), new(big.Int)
	whole.QuoRem(total, denominator, fraction)
	if !whole.IsUint64() {
		return 0, 0, 0, fmt.Errorf("冒险团经验增量溢出")
	}
	return whole.Uint64(), fraction.Uint64(), denominator.Uint64(), nil
}
