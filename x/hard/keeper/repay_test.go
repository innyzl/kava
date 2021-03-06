package keeper_test

import (
	"strings"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto"
	tmtime "github.com/tendermint/tendermint/types/time"

	"github.com/kava-labs/kava/app"
	"github.com/kava-labs/kava/x/hard"
	"github.com/kava-labs/kava/x/hard/types"
	"github.com/kava-labs/kava/x/pricefeed"
)

func (suite *KeeperTestSuite) TestRepay() {
	type args struct {
		borrower                  sdk.AccAddress
		initialBorrowerCoins      sdk.Coins
		initialModuleCoins        sdk.Coins
		depositCoins              []sdk.Coin
		borrowCoins               sdk.Coins
		repayCoins                sdk.Coins
		expectedAccountBalance    sdk.Coins
		expectedModAccountBalance sdk.Coins
	}

	type errArgs struct {
		expectPass   bool
		expectDelete bool
		contains     string
	}

	type borrowTest struct {
		name    string
		args    args
		errArgs errArgs
	}

	model := types.NewInterestRateModel(sdk.MustNewDecFromStr("0.05"), sdk.MustNewDecFromStr("2"), sdk.MustNewDecFromStr("0.8"), sdk.MustNewDecFromStr("10"))

	testCases := []borrowTest{
		{
			"valid: partial repay",
			args{
				borrower:             sdk.AccAddress(crypto.AddressHash([]byte("test"))),
				initialBorrowerCoins: sdk.NewCoins(sdk.NewCoin("ukava", sdk.NewInt(100*KAVA_CF))),
				initialModuleCoins:   sdk.NewCoins(sdk.NewCoin("ukava", sdk.NewInt(1000*KAVA_CF)), sdk.NewCoin("usdx", sdk.NewInt(1000*USDX_CF))),
				depositCoins:         []sdk.Coin{sdk.NewCoin("ukava", sdk.NewInt(100*KAVA_CF))},
				borrowCoins:          sdk.NewCoins(sdk.NewCoin("ukava", sdk.NewInt(50*KAVA_CF))),
				repayCoins:           sdk.NewCoins(sdk.NewCoin("ukava", sdk.NewInt(10*KAVA_CF))),
			},
			errArgs{
				expectPass:   true,
				expectDelete: false,
				contains:     "",
			},
		},
		{
			"valid: repay in full",
			args{
				borrower:             sdk.AccAddress(crypto.AddressHash([]byte("test"))),
				initialBorrowerCoins: sdk.NewCoins(sdk.NewCoin("ukava", sdk.NewInt(100*KAVA_CF))),
				initialModuleCoins:   sdk.NewCoins(sdk.NewCoin("ukava", sdk.NewInt(1000*KAVA_CF)), sdk.NewCoin("usdx", sdk.NewInt(1000*USDX_CF))),
				depositCoins:         []sdk.Coin{sdk.NewCoin("ukava", sdk.NewInt(100*KAVA_CF))},
				borrowCoins:          sdk.NewCoins(sdk.NewCoin("ukava", sdk.NewInt(50*KAVA_CF))),
				repayCoins:           sdk.NewCoins(sdk.NewCoin("ukava", sdk.NewInt(50*KAVA_CF))),
			},
			errArgs{
				expectPass:   true,
				expectDelete: true,
				contains:     "",
			},
		},
		{
			"valid: overpayment is adjusted",
			args{
				borrower:             sdk.AccAddress(crypto.AddressHash([]byte("test"))),
				initialBorrowerCoins: sdk.NewCoins(sdk.NewCoin("ukava", sdk.NewInt(100*KAVA_CF))),
				initialModuleCoins:   sdk.NewCoins(sdk.NewCoin("ukava", sdk.NewInt(1000*KAVA_CF)), sdk.NewCoin("usdx", sdk.NewInt(1000*USDX_CF))),
				depositCoins:         []sdk.Coin{sdk.NewCoin("ukava", sdk.NewInt(80*KAVA_CF))}, // Deposit less so user still has some KAVA
				borrowCoins:          sdk.NewCoins(sdk.NewCoin("ukava", sdk.NewInt(50*KAVA_CF))),
				repayCoins:           sdk.NewCoins(sdk.NewCoin("ukava", sdk.NewInt(60*KAVA_CF))), // Exceeds borrowed coins but not user's balance
			},
			errArgs{
				expectPass:   true,
				expectDelete: true,
				contains:     "",
			},
		},
		{
			"invalid: attempt to repay non-supplied coin",
			args{
				borrower:             sdk.AccAddress(crypto.AddressHash([]byte("test"))),
				initialBorrowerCoins: sdk.NewCoins(sdk.NewCoin("ukava", sdk.NewInt(100*KAVA_CF))),
				initialModuleCoins:   sdk.NewCoins(sdk.NewCoin("ukava", sdk.NewInt(1000*KAVA_CF)), sdk.NewCoin("usdx", sdk.NewInt(1000*USDX_CF))),
				depositCoins:         []sdk.Coin{sdk.NewCoin("ukava", sdk.NewInt(100*KAVA_CF))},
				borrowCoins:          sdk.NewCoins(sdk.NewCoin("ukava", sdk.NewInt(50*KAVA_CF))),
				repayCoins:           sdk.NewCoins(sdk.NewCoin("ukava", sdk.NewInt(10*KAVA_CF)), sdk.NewCoin("bnb", sdk.NewInt(10*KAVA_CF))),
			},
			errArgs{
				expectPass:   false,
				expectDelete: false,
				contains:     "account can only repay up to 0bnb",
			},
		},
		{
			"invalid: insufficent balance for repay",
			args{
				borrower:             sdk.AccAddress(crypto.AddressHash([]byte("test"))),
				initialBorrowerCoins: sdk.NewCoins(sdk.NewCoin("ukava", sdk.NewInt(100*KAVA_CF))),
				initialModuleCoins:   sdk.NewCoins(sdk.NewCoin("ukava", sdk.NewInt(1000*KAVA_CF)), sdk.NewCoin("usdx", sdk.NewInt(1000*USDX_CF))),
				depositCoins:         []sdk.Coin{sdk.NewCoin("ukava", sdk.NewInt(100*KAVA_CF))},
				borrowCoins:          sdk.NewCoins(sdk.NewCoin("ukava", sdk.NewInt(50*KAVA_CF))),
				repayCoins:           sdk.NewCoins(sdk.NewCoin("ukava", sdk.NewInt(51*KAVA_CF))), // Exceeds user's KAVA balance
			},
			errArgs{
				expectPass:   false,
				expectDelete: false,
				contains:     "account can only repay up to 50000000ukava",
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Initialize test app and set context
			tApp := app.NewTestApp()
			ctx := tApp.NewContext(true, abci.Header{Height: 1, Time: tmtime.Now()})

			// Auth module genesis state
			authGS := app.NewAuthGenState(
				[]sdk.AccAddress{tc.args.borrower},
				[]sdk.Coins{tc.args.initialBorrowerCoins})

			// Hard module genesis state
			hardGS := types.NewGenesisState(types.NewParams(
				types.MoneyMarkets{
					types.NewMoneyMarket("usdx",
						types.NewBorrowLimit(false, sdk.NewDec(100000000*USDX_CF), sdk.MustNewDecFromStr("1")), // Borrow Limit
						"usdx:usd",                     // Market ID
						sdk.NewInt(USDX_CF),            // Conversion Factor
						model,                          // Interest Rate Model
						sdk.MustNewDecFromStr("0.05"),  // Reserve Factor
						sdk.MustNewDecFromStr("0.05")), // Keeper Reward Percent
					types.NewMoneyMarket("ukava",
						types.NewBorrowLimit(false, sdk.NewDec(100000000*KAVA_CF), sdk.MustNewDecFromStr("0.8")), // Borrow Limit
						"kava:usd",                     // Market ID
						sdk.NewInt(KAVA_CF),            // Conversion Factor
						model,                          // Interest Rate Model
						sdk.MustNewDecFromStr("0.05"),  // Reserve Factor
						sdk.MustNewDecFromStr("0.05")), // Keeper Reward Percent
				},
			), types.DefaultAccumulationTimes, types.DefaultDeposits, types.DefaultBorrows,
				types.DefaultTotalSupplied, types.DefaultTotalBorrowed, types.DefaultTotalReserves,
			)

			// Pricefeed module genesis state
			pricefeedGS := pricefeed.GenesisState{
				Params: pricefeed.Params{
					Markets: []pricefeed.Market{
						{MarketID: "usdx:usd", BaseAsset: "usdx", QuoteAsset: "usd", Oracles: []sdk.AccAddress{}, Active: true},
						{MarketID: "kava:usd", BaseAsset: "kava", QuoteAsset: "usd", Oracles: []sdk.AccAddress{}, Active: true},
					},
				},
				PostedPrices: []pricefeed.PostedPrice{
					{
						MarketID:      "usdx:usd",
						OracleAddress: sdk.AccAddress{},
						Price:         sdk.MustNewDecFromStr("1.00"),
						Expiry:        time.Now().Add(1 * time.Hour),
					},
					{
						MarketID:      "kava:usd",
						OracleAddress: sdk.AccAddress{},
						Price:         sdk.MustNewDecFromStr("2.00"),
						Expiry:        time.Now().Add(1 * time.Hour),
					},
				},
			}

			// Initialize test application
			tApp.InitializeFromGenesisStates(authGS,
				app.GenesisState{pricefeed.ModuleName: pricefeed.ModuleCdc.MustMarshalJSON(pricefeedGS)},
				app.GenesisState{types.ModuleName: types.ModuleCdc.MustMarshalJSON(hardGS)},
			)

			// Mint coins to Hard module account
			supplyKeeper := tApp.GetSupplyKeeper()
			supplyKeeper.MintCoins(ctx, types.ModuleAccountName, tc.args.initialModuleCoins)

			keeper := tApp.GetHardKeeper()
			suite.app = tApp
			suite.ctx = ctx
			suite.keeper = keeper

			var err error

			// Run BeginBlocker once to transition MoneyMarkets
			hard.BeginBlocker(suite.ctx, suite.keeper)

			// Deposit coins to hard
			err = suite.keeper.Deposit(suite.ctx, tc.args.borrower, tc.args.depositCoins)
			suite.Require().NoError(err)

			// Borrow coins from hard
			err = suite.keeper.Borrow(suite.ctx, tc.args.borrower, tc.args.borrowCoins)
			suite.Require().NoError(err)

			err = suite.keeper.Repay(suite.ctx, tc.args.borrower, tc.args.borrower, tc.args.repayCoins)
			if tc.errArgs.expectPass {
				suite.Require().NoError(err)
				// If we overpaid expect an adjustment
				repaymentCoins, err := suite.keeper.CalculatePaymentAmount(tc.args.borrowCoins, tc.args.repayCoins)
				suite.Require().NoError(err)

				// Check borrower balance
				expectedBorrowerCoins := tc.args.initialBorrowerCoins.Sub(tc.args.depositCoins).Add(tc.args.borrowCoins...).Sub(repaymentCoins)
				acc := suite.getAccount(tc.args.borrower)
				suite.Require().Equal(expectedBorrowerCoins, acc.GetCoins())

				// Check module account balance
				expectedModuleCoins := tc.args.initialModuleCoins.Add(tc.args.depositCoins...).Sub(tc.args.borrowCoins).Add(repaymentCoins...)
				mAcc := suite.getModuleAccount(types.ModuleAccountName)
				suite.Require().Equal(expectedModuleCoins, mAcc.GetCoins())

				// Check user's borrow object
				borrow, foundBorrow := suite.keeper.GetBorrow(suite.ctx, tc.args.borrower)
				expectedBorrowCoins := tc.args.borrowCoins.Sub(repaymentCoins)

				if tc.errArgs.expectDelete {
					suite.Require().False(foundBorrow)
				} else {
					suite.Require().True(foundBorrow)
					suite.Require().Equal(expectedBorrowCoins, borrow.Amount)
				}
			} else {
				suite.Require().Error(err)
				suite.Require().True(strings.Contains(err.Error(), tc.errArgs.contains))

				// Check borrower balance (no repay coins)
				expectedBorrowerCoins := tc.args.initialBorrowerCoins.Sub(tc.args.depositCoins).Add(tc.args.borrowCoins...)
				acc := suite.getAccount(tc.args.borrower)
				suite.Require().Equal(expectedBorrowerCoins, acc.GetCoins())

				// Check module account balance (no repay coins)
				expectedModuleCoins := tc.args.initialModuleCoins.Add(tc.args.depositCoins...).Sub(tc.args.borrowCoins)
				mAcc := suite.getModuleAccount(types.ModuleAccountName)
				suite.Require().Equal(expectedModuleCoins, mAcc.GetCoins())

				// Check user's borrow object (no repay coins)
				borrow, foundBorrow := suite.keeper.GetBorrow(suite.ctx, tc.args.borrower)
				suite.Require().True(foundBorrow)
				suite.Require().Equal(tc.args.borrowCoins, borrow.Amount)
			}
		})
	}
}
