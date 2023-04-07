package types

import (
	"testing"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/NibiruChain/nibiru/x/common"
	"github.com/NibiruChain/nibiru/x/common/asset"
	"github.com/NibiruChain/nibiru/x/common/denoms"
	"github.com/NibiruChain/nibiru/x/common/testutil"
)

func TestVpool_NewPool(t *testing.T) {
	pair := asset.Registry.Pair(denoms.BTC, denoms.NUSD)
	tests := []struct {
		name        string
		args        ArgsNewVpool
		shouldPanic bool
		sqrtDepth   sdk.Dec
	}{
		{name: "pass: normal",
			args: ArgsNewVpool{
				Pair:          pair,
				BaseReserves:  sdk.NewDec(10 * 10), // 10**2
				QuoteReserves: sdk.NewDec(15 * 15), // 15**2
			}, shouldPanic: false, sqrtDepth: sdk.NewDec(150), // 10 * 15
		},
		{name: "pass: zero reserves",
			args: ArgsNewVpool{
				Pair:          pair,
				BaseReserves:  sdk.NewDec(10),
				QuoteReserves: sdk.NewDec(0),
			}, shouldPanic: false, sqrtDepth: sdk.NewDec(0),
		},
		{name: "pass: custom config",
			args: ArgsNewVpool{
				Pair:          pair,
				BaseReserves:  sdk.NewDec(22 * 22), // 22**2
				QuoteReserves: sdk.NewDec(7 * 7),   // 7**2
				Config: &VpoolConfig{
					TradeLimitRatio:        sdk.MustNewDecFromStr("0.1"),
					FluctuationLimitRatio:  sdk.MustNewDecFromStr("0.1"),
					MaxOracleSpreadRatio:   sdk.OneDec(),
					MaintenanceMarginRatio: sdk.OneDec().Quo(sdk.NewDec(18)),
					MaxLeverage:            sdk.NewDec(12),
				},
			}, shouldPanic: false, sqrtDepth: sdk.NewDec(154), // 22 * 7
		},
		{name: "err: negative sqrt depth",
			args: ArgsNewVpool{
				Pair:          pair,
				BaseReserves:  sdk.NewDec(10),
				QuoteReserves: sdk.NewDec(-10),
			}, shouldPanic: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if tc.shouldPanic {
				require.Panics(t, func() {
					_ = NewVpool(tc.args)
				})
			} else {
				vpool := NewVpool(tc.args)
				assert.EqualValues(t, tc.args.Pair, vpool.Pair)
				assert.EqualValues(t, tc.args.BaseReserves, vpool.BaseAssetReserve)
				assert.EqualValues(t, tc.args.QuoteReserves, vpool.QuoteAssetReserve)

				sqrtDepth, err := common.SqrtDec(tc.args.BaseReserves.Mul(tc.args.QuoteReserves))
				assert.NoError(t, err)
				assert.EqualValues(t, sqrtDepth, vpool.SqrtDepth)

				var config VpoolConfig
				if tc.args.Config == nil {
					config = *DefaultVpoolConfig()
				} else {
					config = *tc.args.Config
				}
				assert.EqualValues(t, config, vpool.Config)
			}
		})
	}
}

func TestPoolHasEnoughQuoteReserve(t *testing.T) {
	pair := asset.MustNewPair("BTC:NUSD")

	pool := &Vpool{
		Pair:              pair,
		QuoteAssetReserve: sdk.NewDec(10 * common.TO_MICRO),
		BaseAssetReserve:  sdk.NewDec(10 * common.TO_MICRO),
		SqrtDepth:         common.MustSqrtDec(sdk.NewDec(10 * 10 * common.TO_MICRO)),
		Config: VpoolConfig{
			FluctuationLimitRatio:  sdk.MustNewDecFromStr("0.1"),
			MaxOracleSpreadRatio:   sdk.MustNewDecFromStr("0.1"),
			MaintenanceMarginRatio: sdk.MustNewDecFromStr("0.0625"),
			MaxLeverage:            sdk.NewDec(15),
			TradeLimitRatio:        sdk.MustNewDecFromStr("0.9"), // 0.9
		},
	}

	// less than max ratio
	require.True(t, pool.HasEnoughQuoteReserve(sdk.NewDec(8*common.TO_MICRO)))

	// equal to ratio limit
	require.True(t, pool.HasEnoughQuoteReserve(sdk.NewDec(9*common.TO_MICRO)))

	// more than ratio limit
	require.False(t, pool.HasEnoughQuoteReserve(sdk.NewDec(9_000_001)))
}

func TestSetMarginRatioAndLeverage(t *testing.T) {
	pair := asset.MustNewPair("BTC:NUSD")

	pool := &Vpool{
		Pair:              pair,
		QuoteAssetReserve: sdk.NewDec(10 * common.TO_MICRO),
		BaseAssetReserve:  sdk.NewDec(10 * common.TO_MICRO),
		SqrtDepth:         common.MustSqrtDec(sdk.NewDec(10 * 10 * common.TO_MICRO)),
		Config: VpoolConfig{
			FluctuationLimitRatio:  sdk.MustNewDecFromStr("0.1"),
			MaintenanceMarginRatio: sdk.MustNewDecFromStr("0.42"),
			MaxLeverage:            sdk.NewDec(15),
			MaxOracleSpreadRatio:   sdk.MustNewDecFromStr("0.1"),
			TradeLimitRatio:        sdk.MustNewDecFromStr("0.9"), // 0.9
		},
	}

	require.Equal(t, sdk.MustNewDecFromStr("0.42"), pool.Config.MaintenanceMarginRatio)
	require.Equal(t, sdk.MustNewDecFromStr("15"), pool.Config.MaxLeverage)
}

func TestGetBaseAmountByQuoteAmount(t *testing.T) {
	pair := asset.MustNewPair("BTC:NUSD")

	tests := []struct {
		name               string
		baseAssetReserve   sdk.Dec
		quoteAssetReserve  sdk.Dec
		quoteIn            sdk.Dec
		expectedBaseAmount sdk.Dec
		expectedErr        error
	}{
		{
			name:               "quote amount zero",
			baseAssetReserve:   sdk.NewDec(1000),
			quoteAssetReserve:  sdk.NewDec(1000),
			quoteIn:            sdk.ZeroDec(),
			expectedBaseAmount: sdk.ZeroDec(),
		},
		{
			name:              "simple add quote to pool",
			baseAssetReserve:  sdk.NewDec(1000),
			quoteAssetReserve: sdk.NewDec(1000), // swapInvariant = 1000 * 1000
			quoteIn:           sdk.NewDec(500),  // quoteReserves = 1000 + 500
			// swapInvariant / quoteReserves - baseReserves = 333.33
			expectedBaseAmount: sdk.MustNewDecFromStr("333.333333333333333333"),
		},
		{
			name:              "simple remove quote from pool",
			baseAssetReserve:  sdk.NewDec(1000),
			quoteAssetReserve: sdk.NewDec(1000), // swapInvariant = 1000 * 1000
			quoteIn:           sdk.NewDec(-500), // quoteReserves = 1000 - 500
			// swapInvariant / quoteReserves - baseReserves = 1000
			expectedBaseAmount: sdk.NewDec(1000),
		},
		{
			name:              "too much quote removed results in error",
			baseAssetReserve:  sdk.NewDec(1000),
			quoteAssetReserve: sdk.NewDec(1000),  // swapInvariant = 1000 * 1000
			quoteIn:           sdk.NewDec(-1000), // quoteReserves = 1000 - 1000
			expectedErr:       ErrQuoteReserveAtZero,
		},
		{
			name:              "attempt to remove more than the quote reserves",
			baseAssetReserve:  sdk.NewDec(1000),
			quoteAssetReserve: sdk.NewDec(1000),
			quoteIn:           sdk.NewDec(-9999),
			expectedErr:       ErrQuoteReserveAtZero,
		},
		{
			name:               "add large amount to the quote reserves",
			baseAssetReserve:   sdk.NewDec(1000),
			quoteAssetReserve:  sdk.NewDec(1000),        // swapInvariant = 1000 * 1000
			quoteIn:            sdk.NewDec(999_555_999), // quoteReserves = 1000 + 999_555_999
			expectedBaseAmount: sdk.MustNewDecFromStr("999.998999556802663137"),
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			pool := NewVpool(ArgsNewVpool{
				Pair:          pair,
				QuoteReserves: tc.quoteAssetReserve,
				BaseReserves:  tc.baseAssetReserve,
				Config: &VpoolConfig{
					FluctuationLimitRatio:  sdk.MustNewDecFromStr("0.1"),
					MaintenanceMarginRatio: sdk.MustNewDecFromStr("0.0625"),
					MaxLeverage:            sdk.NewDec(15),
					MaxOracleSpreadRatio:   sdk.MustNewDecFromStr("0.1"),
					TradeLimitRatio:        sdk.MustNewDecFromStr("0.9"), // 0.9
				},
			})

			amount, err := pool.GetBaseAmountByQuoteAmount(tc.quoteIn)
			if tc.expectedErr != nil {
				require.ErrorIs(t, err, tc.expectedErr,
					"expected error: %w, got: %w", tc.expectedErr, err)
			} else {
				require.NoError(t, err)
				require.EqualValuesf(t, tc.expectedBaseAmount, amount,
					"expected quote: %s, got: %s", tc.expectedBaseAmount.String(), amount.String(),
				)
			}
		})
	}
}

func TestGetQuoteAmountByBaseAmount(t *testing.T) {
	pair := asset.MustNewPair("BTC:NUSD")

	tests := []struct {
		name                string
		baseAssetReserve    sdk.Dec
		quoteAssetReserve   sdk.Dec
		baseIn              sdk.Dec
		expectedQuoteAmount sdk.Dec
		expectedErr         error
	}{
		{
			name:                "base amount zero",
			baseAssetReserve:    sdk.NewDec(1000),
			quoteAssetReserve:   sdk.NewDec(1000),
			baseIn:              sdk.ZeroDec(),
			expectedQuoteAmount: sdk.ZeroDec(),
		},
		{
			name:                "simple add base to pool",
			baseAssetReserve:    sdk.NewDec(1000),
			quoteAssetReserve:   sdk.NewDec(1000),
			baseIn:              sdk.NewDec(500),
			expectedQuoteAmount: sdk.MustNewDecFromStr("333.333333333333333333"),
		},
		{
			name:                "simple remove base from pool",
			baseAssetReserve:    sdk.NewDec(1000),
			quoteAssetReserve:   sdk.NewDec(1000),
			baseIn:              sdk.NewDec(-500),
			expectedQuoteAmount: sdk.NewDec(1000),
		},
		{
			name:              "too much base removed results in error",
			baseAssetReserve:  sdk.NewDec(1000),
			quoteAssetReserve: sdk.NewDec(1000),
			baseIn:            sdk.NewDec(-1000),
			expectedErr:       ErrBaseReserveAtZero,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			pool := &Vpool{
				Pair:              pair,
				QuoteAssetReserve: tc.quoteAssetReserve,
				BaseAssetReserve:  tc.baseAssetReserve,
				SqrtDepth:         common.MustSqrtDec(tc.quoteAssetReserve.Mul(tc.baseAssetReserve)),
				Config: VpoolConfig{
					FluctuationLimitRatio:  sdk.OneDec(),
					MaintenanceMarginRatio: sdk.MustNewDecFromStr("0.0625"),
					MaxLeverage:            sdk.NewDec(15),
					MaxOracleSpreadRatio:   sdk.OneDec(),
					TradeLimitRatio:        sdk.OneDec(),
				},
			}

			amount, err := pool.GetQuoteAmountByBaseAmount(tc.baseIn)
			if tc.expectedErr != nil {
				require.ErrorIs(t, err, tc.expectedErr,
					"expected error: %w, got: %w", tc.expectedErr, err)
			} else {
				require.NoError(t, err)
				require.EqualValuesf(t, tc.expectedQuoteAmount, amount,
					"expected quote: %s, got: %s", tc.expectedQuoteAmount.String(), amount.String(),
				)
			}
		})
	}
}

func TestIncreaseDecreaseReserves(t *testing.T) {
	pair := asset.MustNewPair("ATOM:NUSD")

	pool := NewVpool(ArgsNewVpool{
		Pair:          pair,
		QuoteReserves: sdk.NewDec(1 * common.TO_MICRO),
		BaseReserves:  sdk.NewDec(1 * common.TO_MICRO),
		Config: &VpoolConfig{
			FluctuationLimitRatio:  sdk.OneDec(),
			MaintenanceMarginRatio: sdk.MustNewDecFromStr("0.0625"),
			MaxLeverage:            sdk.NewDec(15),
			MaxOracleSpreadRatio:   sdk.OneDec(),
			TradeLimitRatio:        sdk.OneDec(),
		},
	})

	t.Log("decrease quote asset reserve")
	pool.AddToQuoteAssetReserve(sdk.NewDec(-100))
	require.Equal(t, sdk.NewDec(999_900), pool.QuoteAssetReserve)

	t.Log("increase quote asset reserve")
	pool.AddToQuoteAssetReserve(sdk.NewDec(100))
	require.Equal(t, sdk.NewDec(1*common.TO_MICRO), pool.QuoteAssetReserve)

	t.Log("decrease base asset reserve")
	pool.AddToBaseAssetReserve(sdk.NewDec(-100))
	require.Equal(t, sdk.NewDec(999_900), pool.BaseAssetReserve)

	t.Log("increase base asset reserve")
	pool.AddToBaseAssetReserve(sdk.NewDec(100))
	require.Equal(t, sdk.NewDec(1*common.TO_MICRO), pool.BaseAssetReserve)
}

func TestPool_Validate(t *testing.T) {
	type test struct {
		m         *Vpool
		expectErr bool
	}

	cases := map[string]test{
		"invalid pair": {
			m: &Vpool{
				Pair:              "",
				BaseAssetReserve:  sdk.OneDec(),
				QuoteAssetReserve: sdk.OneDec(),
				SqrtDepth:         common.MustSqrtDec(sdk.NewDec(1)),
				Config: VpoolConfig{
					TradeLimitRatio:        sdk.NewDec(-1),
					FluctuationLimitRatio:  sdk.OneDec(),
					MaxOracleSpreadRatio:   sdk.OneDec(),
					MaintenanceMarginRatio: sdk.OneDec(),
					MaxLeverage:            sdk.OneDec(),
				},
			},
			expectErr: true,
		},

		"invalid trade limit ratio < 0": {
			m: &Vpool{
				Pair:              asset.MustNewPair("btc:usd"),
				BaseAssetReserve:  sdk.OneDec(),
				QuoteAssetReserve: sdk.OneDec(),
				SqrtDepth:         common.MustSqrtDec(sdk.NewDec(1)),
				Config: VpoolConfig{
					TradeLimitRatio:        sdk.NewDec(-1),
					FluctuationLimitRatio:  sdk.OneDec(),
					MaxOracleSpreadRatio:   sdk.OneDec(),
					MaintenanceMarginRatio: sdk.OneDec(),
					MaxLeverage:            sdk.OneDec(),
				},
			},
			expectErr: true,
		},

		"invalid trade limit ratio > 1": {
			m: &Vpool{
				Pair:              asset.MustNewPair("btc:usd"),
				BaseAssetReserve:  sdk.OneDec(),
				QuoteAssetReserve: sdk.OneDec(),
				SqrtDepth:         common.MustSqrtDec(sdk.NewDec(1)),
				Config: VpoolConfig{
					TradeLimitRatio:        sdk.NewDec(2),
					FluctuationLimitRatio:  sdk.OneDec(),
					MaxOracleSpreadRatio:   sdk.OneDec(),
					MaintenanceMarginRatio: sdk.OneDec(),
					MaxLeverage:            sdk.OneDec(),
				},
			},
			expectErr: true,
		},

		"quote asset reserve 0": {
			m: &Vpool{
				Pair:              asset.MustNewPair("btc:usd"),
				BaseAssetReserve:  sdk.NewDec(999),
				QuoteAssetReserve: sdk.ZeroDec(),
				SqrtDepth:         common.MustSqrtDec(sdk.NewDec(999)),
				Config: VpoolConfig{
					TradeLimitRatio:        sdk.MustNewDecFromStr("0.10"),
					FluctuationLimitRatio:  sdk.OneDec(),
					MaxOracleSpreadRatio:   sdk.OneDec(),
					MaintenanceMarginRatio: sdk.OneDec(),
					MaxLeverage:            sdk.OneDec(),
				},
			},
			expectErr: true,
		},

		"base asset reserve 0": {
			m: &Vpool{
				Pair:              asset.MustNewPair("btc:usd"),
				QuoteAssetReserve: sdk.NewDec(1 * common.TO_MICRO),
				BaseAssetReserve:  sdk.ZeroDec(),
				SqrtDepth:         sdk.ZeroDec(),
				Config: VpoolConfig{
					TradeLimitRatio:        sdk.MustNewDecFromStr("0.10"),
					FluctuationLimitRatio:  sdk.OneDec(),
					MaxOracleSpreadRatio:   sdk.OneDec(),
					MaintenanceMarginRatio: sdk.OneDec(),
					MaxLeverage:            sdk.OneDec(),
				},
			},
			expectErr: true,
		},

		"fluctuation < 0": {
			m: &Vpool{
				Pair:              asset.MustNewPair("btc:usd"),
				QuoteAssetReserve: sdk.NewDec(1 * common.TO_MICRO),
				BaseAssetReserve:  sdk.NewDec(1 * common.TO_MICRO),
				SqrtDepth:         common.MustSqrtDec(sdk.NewDec(1 * common.TO_MICRO * common.TO_MICRO)),
				Config: VpoolConfig{
					TradeLimitRatio:        sdk.MustNewDecFromStr("0.10"),
					FluctuationLimitRatio:  sdk.NewDec(-1),
					MaxOracleSpreadRatio:   sdk.OneDec(),
					MaintenanceMarginRatio: sdk.OneDec(),
					MaxLeverage:            sdk.OneDec(),
				},
			},
			expectErr: true,
		},

		"fluctuation > 1": {
			m: &Vpool{
				Pair:              asset.MustNewPair("btc:usd"),
				QuoteAssetReserve: sdk.NewDec(1 * common.TO_MICRO),
				BaseAssetReserve:  sdk.NewDec(1 * common.TO_MICRO),
				SqrtDepth:         common.MustSqrtDec(sdk.NewDec(1 * common.TO_MICRO * common.TO_MICRO)),
				Config: VpoolConfig{
					TradeLimitRatio:        sdk.MustNewDecFromStr("0.10"),
					FluctuationLimitRatio:  sdk.NewDec(2),
					MaxOracleSpreadRatio:   sdk.OneDec(),
					MaintenanceMarginRatio: sdk.OneDec(),
					MaxLeverage:            sdk.OneDec(),
				},
			},
			expectErr: true,
		},

		"max oracle spread ratio < 0": {
			m: &Vpool{
				Pair:              asset.MustNewPair("btc:usd"),
				QuoteAssetReserve: sdk.NewDec(1 * common.TO_MICRO),
				BaseAssetReserve:  sdk.NewDec(1 * common.TO_MICRO),
				SqrtDepth:         common.MustSqrtDec(sdk.NewDec(1 * common.TO_MICRO * common.TO_MICRO)),
				Config: VpoolConfig{
					FluctuationLimitRatio:  sdk.MustNewDecFromStr("0.10"),
					MaintenanceMarginRatio: sdk.OneDec(),
					MaxLeverage:            sdk.OneDec(),
					MaxOracleSpreadRatio:   sdk.NewDec(-1),
					TradeLimitRatio:        sdk.MustNewDecFromStr("0.10"),
				},
			},
			expectErr: true,
		},

		"max oracle spread ratio > 1": {
			m: &Vpool{
				Pair:              asset.MustNewPair("btc:usd"),
				QuoteAssetReserve: sdk.NewDec(1 * common.TO_MICRO),
				BaseAssetReserve:  sdk.NewDec(1 * common.TO_MICRO),
				SqrtDepth:         common.MustSqrtDec(sdk.NewDec(1 * common.TO_MICRO * common.TO_MICRO)),
				Config: VpoolConfig{
					FluctuationLimitRatio:  sdk.MustNewDecFromStr("0.10"),
					MaintenanceMarginRatio: sdk.OneDec(),
					MaxLeverage:            sdk.OneDec(),
					MaxOracleSpreadRatio:   sdk.NewDec(2),
					TradeLimitRatio:        sdk.MustNewDecFromStr("0.10"),
				},
			},
			expectErr: true,
		},

		"maintenance ratio < 0": {
			m: &Vpool{
				Pair: asset.MustNewPair("btc:usd"),
				Config: VpoolConfig{
					FluctuationLimitRatio:  sdk.MustNewDecFromStr("0.10"),
					MaintenanceMarginRatio: sdk.NewDec(-1),
					MaxLeverage:            sdk.OneDec(),
					MaxOracleSpreadRatio:   sdk.MustNewDecFromStr("0.10"),
					TradeLimitRatio:        sdk.MustNewDecFromStr("0.10"),
				},
				QuoteAssetReserve: sdk.NewDec(1 * common.TO_MICRO),
				BaseAssetReserve:  sdk.NewDec(1 * common.TO_MICRO),
				SqrtDepth:         common.MustSqrtDec(sdk.NewDec(1 * common.TO_MICRO * common.TO_MICRO)),
			},
			expectErr: true,
		},

		"maintenance ratio > 1": {
			m: &Vpool{
				Pair:              asset.MustNewPair("btc:usd"),
				QuoteAssetReserve: sdk.NewDec(1 * common.TO_MICRO),
				BaseAssetReserve:  sdk.NewDec(1 * common.TO_MICRO),
				SqrtDepth:         common.MustSqrtDec(sdk.NewDec(1 * common.TO_MICRO * common.TO_MICRO)),
				Config: VpoolConfig{
					FluctuationLimitRatio:  sdk.MustNewDecFromStr("0.10"),
					MaintenanceMarginRatio: sdk.NewDec(2),
					MaxLeverage:            sdk.OneDec(),
					MaxOracleSpreadRatio:   sdk.MustNewDecFromStr("0.10"),
					TradeLimitRatio:        sdk.MustNewDecFromStr("0.10"),
				},
			},
			expectErr: true,
		},

		"max leverage < 0": {
			m: &Vpool{
				Pair:              asset.MustNewPair("btc:usd"),
				QuoteAssetReserve: sdk.NewDec(1 * common.TO_MICRO),
				BaseAssetReserve:  sdk.NewDec(1 * common.TO_MICRO),
				SqrtDepth:         common.MustSqrtDec(sdk.NewDec(1 * common.TO_MICRO * common.TO_MICRO)),
				Config: VpoolConfig{
					FluctuationLimitRatio:  sdk.MustNewDecFromStr("0.10"),
					MaintenanceMarginRatio: sdk.MustNewDecFromStr("0.10"),
					MaxLeverage:            sdk.MustNewDecFromStr("-0.10"),
					MaxOracleSpreadRatio:   sdk.MustNewDecFromStr("0.10"),
					TradeLimitRatio:        sdk.MustNewDecFromStr("0.10"),
				},
			},
			expectErr: true,
		},

		"max leverage too high for maintenance margin ratio": {
			m: &Vpool{
				Pair:              asset.MustNewPair("btc:usd"),
				QuoteAssetReserve: sdk.NewDec(1 * common.TO_MICRO),
				BaseAssetReserve:  sdk.NewDec(1 * common.TO_MICRO),
				SqrtDepth:         common.MustSqrtDec(sdk.NewDec(1 * common.TO_MICRO * common.TO_MICRO)),
				Config: VpoolConfig{
					FluctuationLimitRatio:  sdk.MustNewDecFromStr("0.10"),
					MaintenanceMarginRatio: sdk.MustNewDecFromStr("0.10"), // Equivalent to 10 leverage
					MaxLeverage:            sdk.MustNewDecFromStr("11"),
					MaxOracleSpreadRatio:   sdk.MustNewDecFromStr("0.10"),
					TradeLimitRatio:        sdk.MustNewDecFromStr("0.10"),
				},
			},
			expectErr: true,
		},

		"success": {
			m: &Vpool{
				Pair:              asset.MustNewPair("btc:usd"),
				QuoteAssetReserve: sdk.NewDec(1 * common.TO_MICRO),
				BaseAssetReserve:  sdk.NewDec(1 * common.TO_MICRO),
				SqrtDepth:         common.MustSqrtDec(sdk.NewDec(1 * common.TO_MICRO * common.TO_MICRO)),
				Config: VpoolConfig{
					FluctuationLimitRatio:  sdk.MustNewDecFromStr("0.10"),
					MaintenanceMarginRatio: sdk.MustNewDecFromStr("0.0625"),
					MaxLeverage:            sdk.MustNewDecFromStr("15"),
					MaxOracleSpreadRatio:   sdk.MustNewDecFromStr("0.10"),
					TradeLimitRatio:        sdk.MustNewDecFromStr("0.10"),
				},
			},
			expectErr: false,
		},
	}

	for name, tc := range cases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			err := tc.m.Validate()
			if err == nil && tc.expectErr {
				t.Fatal("error expected")
			} else if err != nil && !tc.expectErr {
				t.Fatal("unexpected error")
			}
		})
	}
}

func TestVpool_GetMarkPrice(t *testing.T) {
	tests := []struct {
		name          string
		pool          Vpool
		expectedValue sdk.Dec
	}{
		{
			"happy path",
			Vpool{
				Pair:              asset.Registry.Pair(denoms.BTC, denoms.NUSD),
				BaseAssetReserve:  sdk.MustNewDecFromStr("10"),
				QuoteAssetReserve: sdk.MustNewDecFromStr("10000"),
			},
			sdk.MustNewDecFromStr("1000"),
		},
		{
			"nil base",
			Vpool{
				Pair:              asset.Registry.Pair(denoms.BTC, denoms.NUSD),
				BaseAssetReserve:  sdk.Dec{},
				QuoteAssetReserve: sdk.MustNewDecFromStr("10000"),
			},
			sdk.ZeroDec(),
		},
		{
			"zero base",
			Vpool{
				Pair:              asset.Registry.Pair(denoms.BTC, denoms.NUSD),
				BaseAssetReserve:  sdk.ZeroDec(),
				QuoteAssetReserve: sdk.MustNewDecFromStr("10000"),
			},
			sdk.ZeroDec(),
		},
		{
			"nil quote",
			Vpool{
				Pair:              asset.Registry.Pair(denoms.BTC, denoms.NUSD),
				BaseAssetReserve:  sdk.MustNewDecFromStr("10"),
				QuoteAssetReserve: sdk.Dec{},
			},
			sdk.ZeroDec(),
		},
		{
			"zero quote",
			Vpool{
				Pair:              asset.Registry.Pair(denoms.BTC, denoms.NUSD),
				BaseAssetReserve:  sdk.MustNewDecFromStr("10"),
				QuoteAssetReserve: sdk.ZeroDec(),
			},
			sdk.ZeroDec(),
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			require.True(t, tc.expectedValue.Equal(tc.pool.GetMarkPrice()))
		})
	}
}

func TestVpool_IsOverFluctuationLimit(t *testing.T) {
	tests := []struct {
		name string
		pool Vpool

		isOverLimit bool
	}{
		{
			name: "zero fluctuation limit ratio",
			pool: Vpool{
				Pair:              asset.Registry.Pair(denoms.BTC, denoms.NUSD),
				QuoteAssetReserve: sdk.OneDec(),
				BaseAssetReserve:  sdk.OneDec(),
				Config: VpoolConfig{
					FluctuationLimitRatio:  sdk.ZeroDec(),
					MaintenanceMarginRatio: sdk.MustNewDecFromStr("0.0625"),
					MaxLeverage:            sdk.MustNewDecFromStr("15"),
					MaxOracleSpreadRatio:   sdk.OneDec(),
					TradeLimitRatio:        sdk.OneDec(),
				},
			},
			isOverLimit: false,
		},
		{
			name: "lower limit of fluctuation limit",
			pool: Vpool{
				Pair:              asset.Registry.Pair(denoms.BTC, denoms.NUSD),
				QuoteAssetReserve: sdk.NewDec(999),
				BaseAssetReserve:  sdk.OneDec(),
				Config: VpoolConfig{
					FluctuationLimitRatio:  sdk.ZeroDec(),
					MaintenanceMarginRatio: sdk.MustNewDecFromStr("0.0625"),
					MaxLeverage:            sdk.MustNewDecFromStr("15"),
					MaxOracleSpreadRatio:   sdk.OneDec(),
					TradeLimitRatio:        sdk.OneDec(),
				},
			},
			isOverLimit: false,
		},
		{
			name: "upper limit of fluctuation limit",
			pool: Vpool{
				Pair:              asset.Registry.Pair(denoms.BTC, denoms.NUSD),
				QuoteAssetReserve: sdk.NewDec(1001),
				BaseAssetReserve:  sdk.OneDec(),
				Config: VpoolConfig{
					FluctuationLimitRatio:  sdk.MustNewDecFromStr("0.001"),
					MaintenanceMarginRatio: sdk.MustNewDecFromStr("0.0625"),
					MaxLeverage:            sdk.MustNewDecFromStr("15"),
					MaxOracleSpreadRatio:   sdk.OneDec(),
					TradeLimitRatio:        sdk.OneDec(),
				},
			},
			isOverLimit: false,
		},
		{
			name: "under fluctuation limit",
			pool: Vpool{
				Pair:              asset.Registry.Pair(denoms.BTC, denoms.NUSD),
				QuoteAssetReserve: sdk.NewDec(998),
				BaseAssetReserve:  sdk.OneDec(),
				Config: VpoolConfig{
					FluctuationLimitRatio:  sdk.MustNewDecFromStr("0.001"),
					MaintenanceMarginRatio: sdk.MustNewDecFromStr("0.0625"),
					MaxLeverage:            sdk.MustNewDecFromStr("15"),
					MaxOracleSpreadRatio:   sdk.OneDec(),
					TradeLimitRatio:        sdk.OneDec(),
				},
			},
			isOverLimit: true,
		},
		{
			name: "over fluctuation limit",
			pool: Vpool{
				Pair:              asset.Registry.Pair(denoms.BTC, denoms.NUSD),
				QuoteAssetReserve: sdk.NewDec(1002),
				BaseAssetReserve:  sdk.OneDec(),
				Config: VpoolConfig{
					FluctuationLimitRatio:  sdk.MustNewDecFromStr("0.001"),
					MaintenanceMarginRatio: sdk.MustNewDecFromStr("0.0625"),
					MaxLeverage:            sdk.MustNewDecFromStr("15"),
					MaxOracleSpreadRatio:   sdk.OneDec(),
					TradeLimitRatio:        sdk.OneDec(),
				},
			},
			isOverLimit: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			snapshot := NewReserveSnapshot(
				asset.Registry.Pair(denoms.BTC, denoms.NUSD),
				sdk.OneDec(),
				sdk.NewDec(1000),
				time.Now(),
			)
			assert.EqualValues(t, tc.isOverLimit, tc.pool.IsOverFluctuationLimitInRelationWithSnapshot(snapshot))
		})
	}
}

func TestVpool_ToSnapshot(t *testing.T) {
	tests := []struct {
		name       string
		vpool      Vpool
		expectFail bool
	}{
		{
			name: "happy path",
			vpool: Vpool{
				Pair:              asset.Registry.Pair(denoms.BTC, denoms.NUSD),
				BaseAssetReserve:  sdk.NewDec(10),
				QuoteAssetReserve: sdk.NewDec(10_000),
			},
			expectFail: false,
		},
		{
			name: "err invalid base",
			vpool: Vpool{
				Pair:              asset.Registry.Pair(denoms.BTC, denoms.NUSD),
				BaseAssetReserve:  sdk.Dec{},
				QuoteAssetReserve: sdk.NewDec(500),
			},
			expectFail: true,
		},
		{
			name: "err invalid quote",
			vpool: Vpool{
				Pair:              asset.Registry.Pair(denoms.BTC, denoms.NUSD),
				BaseAssetReserve:  sdk.NewDec(500),
				QuoteAssetReserve: sdk.Dec{},
			},
			expectFail: true,
		},
		{
			name: "err negative quote",
			vpool: Vpool{
				Pair:              asset.Registry.Pair(denoms.BTC, denoms.NUSD),
				BaseAssetReserve:  sdk.NewDec(500),
				QuoteAssetReserve: sdk.NewDec(-500),
			},
			expectFail: true,
		},
		{
			name: "err negative base",
			vpool: Vpool{
				Pair:              asset.Registry.Pair(denoms.BTC, denoms.NUSD),
				BaseAssetReserve:  sdk.NewDec(-500),
				QuoteAssetReserve: sdk.NewDec(500),
			},
			expectFail: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctx := testutil.BlankContext(StoreKey)
			if tc.expectFail {
				require.Panics(t, func() {
					_ = tc.vpool.ToSnapshot(ctx)
				})
			} else {
				snapshot := tc.vpool.ToSnapshot(ctx)
				assert.EqualValues(t, tc.vpool.Pair, snapshot.Pair)
				assert.EqualValues(t, tc.vpool.BaseAssetReserve, snapshot.BaseAssetReserve)
				assert.EqualValues(t, tc.vpool.QuoteAssetReserve, snapshot.QuoteAssetReserve)
				assert.EqualValues(t, ctx.BlockTime().UnixMilli(), snapshot.TimestampMs)
			}
		})
	}
}

func TestDefaultVpoolConfig(t *testing.T) {
	vpoolCfg := DefaultVpoolConfig()
	err := vpoolCfg.Validate()
	require.NoError(t, err)
}

func TestVpoolConfigWith(t *testing.T) {
	vpoolCfg := DefaultVpoolConfig()

	vpoolCfgUpdates := VpoolConfig{
		TradeLimitRatio:        sdk.NewDec(12),
		FluctuationLimitRatio:  sdk.NewDec(34),
		MaxOracleSpreadRatio:   sdk.NewDec(56),
		MaintenanceMarginRatio: sdk.NewDec(78),
		MaxLeverage:            sdk.NewDec(910),
	}

	var newVpoolCfg VpoolConfig

	testCases := testutil.FunctionTestCases{
		{Name: "WithTradeLimitRatio", Test: func() {
			assert.NotEqualValues(t, vpoolCfgUpdates.TradeLimitRatio, vpoolCfg.TradeLimitRatio)
			newVpoolCfg = *vpoolCfg.WithTradeLimitRatio(vpoolCfgUpdates.TradeLimitRatio)
			assert.EqualValues(t, vpoolCfgUpdates.TradeLimitRatio, newVpoolCfg.TradeLimitRatio)
		}},
		{Name: "WithFluctuationLimitRatio", Test: func() {
			assert.NotEqualValues(t, vpoolCfgUpdates.FluctuationLimitRatio, vpoolCfg.FluctuationLimitRatio)
			newVpoolCfg = *vpoolCfg.WithFluctuationLimitRatio(vpoolCfgUpdates.FluctuationLimitRatio)
			assert.EqualValues(t, vpoolCfgUpdates.FluctuationLimitRatio, newVpoolCfg.FluctuationLimitRatio)
		}},
		{Name: "WithMaxOracleSpreadRatio", Test: func() {
			assert.NotEqualValues(t, vpoolCfgUpdates.MaxOracleSpreadRatio, vpoolCfg.MaxOracleSpreadRatio)
			newVpoolCfg = *vpoolCfg.WithMaxOracleSpreadRatio(vpoolCfgUpdates.MaxOracleSpreadRatio)
			assert.EqualValues(t, vpoolCfgUpdates.MaxOracleSpreadRatio, newVpoolCfg.MaxOracleSpreadRatio)
		}},
		{Name: "WithMaintenanceMarginRatio", Test: func() {
			assert.NotEqualValues(t, vpoolCfgUpdates.MaintenanceMarginRatio, vpoolCfg.MaintenanceMarginRatio)
			newVpoolCfg = *vpoolCfg.WithMaintenanceMarginRatio(vpoolCfgUpdates.MaintenanceMarginRatio)
			assert.EqualValues(t, vpoolCfgUpdates.MaintenanceMarginRatio, newVpoolCfg.MaintenanceMarginRatio)
		}},
		{Name: "WithMaxLeverage", Test: func() {
			assert.NotEqualValues(t, vpoolCfgUpdates.MaxLeverage, vpoolCfg.MaxLeverage)
			newVpoolCfg = *vpoolCfg.WithMaxLeverage(vpoolCfgUpdates.MaxLeverage)
			assert.EqualValues(t, vpoolCfgUpdates.MaxLeverage, newVpoolCfg.MaxLeverage)
		}},
	}

	testutil.RunFunctionTests(t, testCases)
}