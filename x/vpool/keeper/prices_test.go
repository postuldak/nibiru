package keeper

import (
	"testing"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/NibiruChain/nibiru/x/common"
	pftypes "github.com/NibiruChain/nibiru/x/pricefeed/types"
	"github.com/NibiruChain/nibiru/x/testutil/mock"
	"github.com/NibiruChain/nibiru/x/vpool/types"
)

func TestGetUnderlyingPrice(t *testing.T) {
	tests := []struct {
		name           string
		pair           common.TokenPair
		pricefeedPrice sdk.Dec
	}{
		{
			name:           "correctly fetch underlying price",
			pair:           common.TokenPair("btc:nusd"),
			pricefeedPrice: sdk.NewDec(40000),
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			mockPricefeedKeeper := mock.NewMockPriceKeeper(gomock.NewController(t))
			vpoolKeeper, ctx := VpoolKeeper(t, mockPricefeedKeeper)

			mockPricefeedKeeper.
				EXPECT().
				GetCurrentPrice(
					gomock.Eq(ctx),
					gomock.Eq(tc.pair.GetBaseTokenDenom()),
					gomock.Eq(tc.pair.GetQuoteTokenDenom()),
				).
				Return(
					pftypes.CurrentPrice{
						PairID: tc.pair.String(),
						Price:  tc.pricefeedPrice,
					}, nil,
				)

			price, err := vpoolKeeper.GetUnderlyingPrice(ctx, tc.pair)
			require.NoError(t, err)
			require.EqualValues(t, tc.pricefeedPrice, price)
		})
	}
}

func TestGetSpotPrice(t *testing.T) {
	tests := []struct {
		name              string
		pair              common.TokenPair
		quoteAssetReserve sdk.Dec
		baseAssetReserve  sdk.Dec
		expectedPrice     sdk.Dec
	}{
		{
			name:              "correctly fetch underlying price",
			pair:              common.TokenPair("btc:nusd"),
			quoteAssetReserve: sdk.NewDec(40_000),
			baseAssetReserve:  sdk.NewDec(1),
			expectedPrice:     sdk.NewDec(40000),
		},
		{
			name:              "complex price",
			pair:              common.TokenPair("btc:nusd"),
			quoteAssetReserve: sdk.NewDec(2_489_723_947),
			baseAssetReserve:  sdk.NewDec(34_597_234),
			expectedPrice:     sdk.MustNewDecFromStr("71.963092396345904415"),
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			vpoolKeeper, ctx := VpoolKeeper(t,
				mock.NewMockPriceKeeper(gomock.NewController(t)))

			vpoolKeeper.CreatePool(
				ctx,
				tc.pair.String(),
				/*tradeLimitRatio=*/ sdk.OneDec(),
				tc.quoteAssetReserve,
				tc.baseAssetReserve,
				/*fluctuationLimitratio=*/ sdk.OneDec(),
			)

			price, err := vpoolKeeper.GetSpotPrice(ctx, tc.pair)
			require.NoError(t, err)
			require.EqualValues(t, tc.expectedPrice, price)
		})
	}
}

func TestGetBaseAssetPrice(t *testing.T) {
	tests := []struct {
		name                string
		pair                common.TokenPair
		quoteAssetReserve   sdk.Dec
		baseAssetReserve    sdk.Dec
		baseAmount          sdk.Dec
		direction           types.Direction
		expectedQuoteAmount sdk.Dec
		expectedErr         error
	}{
		{
			name:                "zero base asset means zero price",
			pair:                common.TokenPair("btc:nusd"),
			quoteAssetReserve:   sdk.NewDec(40_000),
			baseAssetReserve:    sdk.NewDec(10_000),
			baseAmount:          sdk.ZeroDec(),
			direction:           types.Direction_ADD_TO_POOL,
			expectedQuoteAmount: sdk.ZeroDec(),
		},
		{
			name:                "simple add base to pool",
			pair:                common.TokenPair("btc:nusd"),
			baseAssetReserve:    sdk.NewDec(1000),
			quoteAssetReserve:   sdk.NewDec(1000),
			baseAmount:          sdk.MustNewDecFromStr("500"),
			direction:           types.Direction_ADD_TO_POOL,
			expectedQuoteAmount: sdk.MustNewDecFromStr("333.333333333333333333"), // rounds down
		},
		{
			name:                "simple remove base from pool",
			pair:                common.TokenPair("btc:nusd"),
			baseAssetReserve:    sdk.NewDec(1000),
			quoteAssetReserve:   sdk.NewDec(1000),
			baseAmount:          sdk.MustNewDecFromStr("500"),
			direction:           types.Direction_REMOVE_FROM_POOL,
			expectedQuoteAmount: sdk.MustNewDecFromStr("1000"),
		},
		{
			name:              "too much base removed results in error",
			pair:              common.TokenPair("btc:nusd"),
			baseAssetReserve:  sdk.NewDec(1000),
			quoteAssetReserve: sdk.NewDec(1000),
			baseAmount:        sdk.MustNewDecFromStr("1000"),
			direction:         types.Direction_REMOVE_FROM_POOL,
			expectedErr:       types.ErrBaseReserveAtZero,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			vpoolKeeper, ctx := VpoolKeeper(t,
				mock.NewMockPriceKeeper(gomock.NewController(t)))

			vpoolKeeper.CreatePool(
				ctx,
				tc.pair.String(),
				/*tradeLimitRatio=*/ sdk.OneDec(),
				tc.quoteAssetReserve,
				tc.baseAssetReserve,
				/*fluctuationLimitRatio=*/ sdk.OneDec(),
			)

			quoteAmount, err := vpoolKeeper.GetBaseAssetPrice(ctx, tc.pair, tc.direction, tc.baseAmount)
			if tc.expectedErr != nil {
				require.ErrorIs(t, err, tc.expectedErr,
					"expected error: %w, got: %w", tc.expectedErr, err)
			} else {
				require.NoError(t, err)
				require.EqualValuesf(t, tc.expectedQuoteAmount, quoteAmount,
					"expected quote: %s, got: %s", tc.expectedQuoteAmount.String(), quoteAmount.String(),
				)
			}
		})
	}
}

func TestGetQuoteAssetPrice(t *testing.T) {
	tests := []struct {
		name               string
		pair               common.TokenPair
		quoteAssetReserve  sdk.Dec
		baseAssetReserve   sdk.Dec
		quoteAmount        sdk.Dec
		direction          types.Direction
		expectedBaseAmount sdk.Dec
		expectedErr        error
	}{
		{
			name:               "zero base asset means zero price",
			pair:               common.TokenPair("btc:nusd"),
			quoteAssetReserve:  sdk.NewDec(40_000),
			baseAssetReserve:   sdk.NewDec(10_000),
			quoteAmount:        sdk.ZeroDec(),
			direction:          types.Direction_ADD_TO_POOL,
			expectedBaseAmount: sdk.ZeroDec(),
		},
		{
			name:               "simple add base to pool",
			pair:               common.TokenPair("btc:nusd"),
			baseAssetReserve:   sdk.NewDec(1000),
			quoteAssetReserve:  sdk.NewDec(1000),
			quoteAmount:        sdk.NewDec(500),
			direction:          types.Direction_ADD_TO_POOL,
			expectedBaseAmount: sdk.MustNewDecFromStr("333.333333333333333333"), // rounds down
		},
		{
			name:               "simple remove base from pool",
			pair:               common.TokenPair("btc:nusd"),
			baseAssetReserve:   sdk.NewDec(1000),
			quoteAssetReserve:  sdk.NewDec(1000),
			quoteAmount:        sdk.NewDec(500),
			direction:          types.Direction_REMOVE_FROM_POOL,
			expectedBaseAmount: sdk.NewDec(1000),
		},
		{
			name:              "too much base removed results in error",
			pair:              common.TokenPair("btc:nusd"),
			baseAssetReserve:  sdk.NewDec(1000),
			quoteAssetReserve: sdk.NewDec(1000),
			quoteAmount:       sdk.NewDec(1000),
			direction:         types.Direction_REMOVE_FROM_POOL,
			expectedErr:       types.ErrQuoteReserveAtZero,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			vpoolKeeper, ctx := VpoolKeeper(t,
				mock.NewMockPriceKeeper(gomock.NewController(t)))

			vpoolKeeper.CreatePool(
				ctx,
				tc.pair.String(),
				/*tradeLimitRatio=*/ sdk.OneDec(),
				tc.quoteAssetReserve,
				tc.baseAssetReserve,
				/*fluctuationLimitRatio=*/ sdk.OneDec(),
			)

			baseAmount, err := vpoolKeeper.GetQuoteAssetPrice(ctx, tc.pair, tc.direction, tc.quoteAmount)
			if tc.expectedErr != nil {
				require.ErrorIs(t, err, tc.expectedErr,
					"expected error: %w, got: %w", tc.expectedErr, err)
			} else {
				require.NoError(t, err)
				require.EqualValuesf(t, tc.expectedBaseAmount, baseAmount,
					"expected quote: %s, got: %s", tc.expectedBaseAmount.String(), baseAmount.String(),
				)
			}
		})
	}
}

func TestCalcTwap(t *testing.T) {
	tests := []struct {
		name               string
		pair               common.TokenPair
		reserveSnapshots   []types.ReserveSnapshot
		currentBlocktime   time.Time
		currentBlockheight int64
		lookbackInterval   time.Duration
		twapCalcOption     types.TwapCalcOption
		direction          types.Direction
		assetAmount        sdk.Dec
		expectedPrice      sdk.Dec
		expectedErr        error
	}{
		{
			name: "spot price twap calc, t=[10,30]",
			pair: common.TokenPair("btc:nusd"),
			reserveSnapshots: []types.ReserveSnapshot{
				{
					QuoteAssetReserve: sdk.NewDec(90),
					BaseAssetReserve:  sdk.NewDec(10),
					TimestampMs:       10,
					BlockNumber:       1,
				},
				{
					QuoteAssetReserve: sdk.NewDec(85),
					BaseAssetReserve:  sdk.NewDec(10),
					TimestampMs:       20,
					BlockNumber:       2,
				},
				{
					QuoteAssetReserve: sdk.NewDec(95),
					BaseAssetReserve:  sdk.NewDec(10),
					TimestampMs:       30,
					BlockNumber:       3,
				},
			},
			currentBlocktime:   time.UnixMilli(30),
			currentBlockheight: 3,
			lookbackInterval:   20 * time.Millisecond,
			twapCalcOption:     types.TwapCalcOption_SPOT,
			expectedPrice:      sdk.MustNewDecFromStr("8.75"),
		},
		{
			name: "spot price twap calc, t=[11,35]",
			pair: common.TokenPair("btc:nusd"),
			reserveSnapshots: []types.ReserveSnapshot{
				{
					QuoteAssetReserve: sdk.NewDec(90),
					BaseAssetReserve:  sdk.NewDec(10),
					TimestampMs:       10,
					BlockNumber:       1,
				},
				{
					QuoteAssetReserve: sdk.NewDec(85),
					BaseAssetReserve:  sdk.NewDec(10),
					TimestampMs:       20,
					BlockNumber:       2,
				},
				{
					QuoteAssetReserve: sdk.NewDec(95),
					BaseAssetReserve:  sdk.NewDec(10),
					TimestampMs:       30,
					BlockNumber:       3,
				},
			},
			currentBlocktime:   time.UnixMilli(35),
			currentBlockheight: 4,
			lookbackInterval:   24 * time.Millisecond,
			twapCalcOption:     types.TwapCalcOption_SPOT,
			expectedPrice:      sdk.MustNewDecFromStr("8.895833333333333333"),
		},
		{
			name: "quote asset swap twap calc, add to pool, t=[10,30]",
			pair: common.TokenPair("btc:nusd"),
			reserveSnapshots: []types.ReserveSnapshot{
				{
					QuoteAssetReserve: sdk.NewDec(30),
					BaseAssetReserve:  sdk.NewDec(10),
					TimestampMs:       10,
					BlockNumber:       1,
				},
				{
					QuoteAssetReserve: sdk.NewDec(40),
					BaseAssetReserve:  sdk.MustNewDecFromStr("7.5"),
					TimestampMs:       20,
					BlockNumber:       2,
				},
			},
			currentBlocktime:   time.UnixMilli(30),
			currentBlockheight: 3,
			lookbackInterval:   20 * time.Millisecond,
			twapCalcOption:     types.TwapCalcOption_QUOTE_ASSET_SWAP,
			direction:          types.Direction_ADD_TO_POOL,
			assetAmount:        sdk.NewDec(10),
			expectedPrice:      sdk.NewDec(2),
		},
		{
			name: "quote asset swap twap calc, remove from pool, t=[10,30]",
			pair: common.TokenPair("btc:nusd"),
			reserveSnapshots: []types.ReserveSnapshot{
				{
					QuoteAssetReserve: sdk.NewDec(60),
					BaseAssetReserve:  sdk.NewDec(10),
					TimestampMs:       10,
					BlockNumber:       1,
				},
				{
					QuoteAssetReserve: sdk.NewDec(50),
					BaseAssetReserve:  sdk.NewDec(12),
					TimestampMs:       20,
					BlockNumber:       2,
				},
			},
			currentBlocktime:   time.UnixMilli(30),
			currentBlockheight: 3,
			lookbackInterval:   20 * time.Millisecond,
			twapCalcOption:     types.TwapCalcOption_QUOTE_ASSET_SWAP,
			direction:          types.Direction_REMOVE_FROM_POOL,
			assetAmount:        sdk.NewDec(10),
			expectedPrice:      sdk.MustNewDecFromStr("2.5"),
		},
		{
			name: "base asset swap twap calc, add to pool, t=[10,30]",
			pair: common.TokenPair("btc:nusd"),
			reserveSnapshots: []types.ReserveSnapshot{
				{
					QuoteAssetReserve: sdk.NewDec(60),
					BaseAssetReserve:  sdk.NewDec(10),
					TimestampMs:       10,
					BlockNumber:       1,
				},
				{
					QuoteAssetReserve: sdk.NewDec(30),
					BaseAssetReserve:  sdk.NewDec(20),
					TimestampMs:       20,
					BlockNumber:       2,
				},
			},
			currentBlocktime:   time.UnixMilli(30),
			currentBlockheight: 3,
			lookbackInterval:   20 * time.Millisecond,
			twapCalcOption:     types.TwapCalcOption_BASE_ASSET_SWAP,
			direction:          types.Direction_ADD_TO_POOL,
			assetAmount:        sdk.NewDec(10),
			expectedPrice:      sdk.NewDec(20),
		},
		{
			name: "base asset swap twap calc, remove from pool, t=[10,30]",
			pair: common.TokenPair("btc:nusd"),
			reserveSnapshots: []types.ReserveSnapshot{
				{
					QuoteAssetReserve: sdk.NewDec(60),
					BaseAssetReserve:  sdk.NewDec(10),
					TimestampMs:       10,
					BlockNumber:       1,
				},
				{
					QuoteAssetReserve: sdk.NewDec(75),
					BaseAssetReserve:  sdk.NewDec(8),
					TimestampMs:       20,
					BlockNumber:       2,
				},
			},
			currentBlocktime:   time.UnixMilli(30),
			currentBlockheight: 3,
			lookbackInterval:   20 * time.Millisecond,
			twapCalcOption:     types.TwapCalcOption_BASE_ASSET_SWAP,
			direction:          types.Direction_REMOVE_FROM_POOL,
			assetAmount:        sdk.NewDec(2),
			expectedPrice:      sdk.NewDec(20),
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			vpoolKeeper, ctx := VpoolKeeper(t,
				mock.NewMockPriceKeeper(gomock.NewController(t)))
			ctx = ctx.WithBlockTime(time.UnixMilli(0)).WithBlockHeight(0)

			t.Log("Create an empty pool for the first block, it's snapshot won't be used")
			vpoolKeeper.CreatePool(
				ctx,
				tc.pair.String(),
				sdk.ZeroDec(),
				sdk.ZeroDec(),
				sdk.ZeroDec(),
				sdk.ZeroDec(),
			)

			for i, snapshot := range tc.reserveSnapshots {
				vpoolKeeper.saveSnapshot(
					ctx,
					tc.pair,
					uint64(i+1),
					snapshot.QuoteAssetReserve,
					snapshot.BaseAssetReserve,
					time.UnixMilli(snapshot.TimestampMs),
					snapshot.BlockNumber,
				)
			}
			vpoolKeeper.saveSnapshotCounter(ctx, tc.pair, uint64(len(tc.reserveSnapshots)))
			ctx = ctx.WithBlockTime(tc.currentBlocktime).WithBlockHeight(tc.currentBlockheight)

			price, err := vpoolKeeper.calcTwap(ctx,
				tc.pair,
				tc.twapCalcOption,
				tc.direction,
				tc.assetAmount,
				tc.lookbackInterval,
			)
			require.NoError(t, err)
			require.EqualValuesf(t, tc.expectedPrice, price,
				"expected %s, got %s", tc.expectedPrice.String(), price.String())
		})
	}
}