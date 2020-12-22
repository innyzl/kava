package keeper

import (
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	supplyExported "github.com/cosmos/cosmos-sdk/x/supply/exported"

	"github.com/kava-labs/kava/x/hard/types"
)

// Deposit deposit
func (k Keeper) Deposit(ctx sdk.Context, depositor sdk.AccAddress, coins sdk.Coins) error {
	// Set any new denoms' global supply index to 1.0
	for _, coin := range coins {
		_, foundInterestFactor := k.GetSupplyInterestFactor(ctx, coin.Denom)
		if !foundInterestFactor {
			_, foundMm := k.GetMoneyMarket(ctx, coin.Denom)
			if foundMm {
				k.SetSupplyInterestFactor(ctx, coin.Denom, sdk.OneDec())
			}
		}
	}

	// Get current stored LTV based on stored borrows/deposits
	prevLtv, shouldRemoveIndex, err := k.GetStoreLTV(ctx, depositor)
	if err != nil {
		return err
	}

	// Sync any outstanding interest
	k.SyncBorrowInterest(ctx, depositor)
	k.SyncSupplyInterest(ctx, depositor)

	err = k.ValidateDeposit(ctx, coins)
	if err != nil {
		return err
	}

	err = k.supplyKeeper.SendCoinsFromAccountToModule(ctx, depositor, types.ModuleAccountName, coins)
	if err != nil {
		if strings.Contains(err.Error(), "insufficient account funds") {
			accCoins := k.accountKeeper.GetAccount(ctx, depositor).SpendableCoins(ctx.BlockTime())
			for _, coin := range coins {
				_, isNegative := accCoins.SafeSub(sdk.NewCoins(coin))
				if isNegative {
					return sdkerrors.Wrapf(types.ErrBorrowExceedsAvailableBalance,
						"insufficient funds: the requested deposit amount of %s exceeds the total available account funds of %s%s",
						coin, accCoins.AmountOf(coin.Denom), coin.Denom,
					)
				}
			}
		}
	}
	if err != nil {
		return err
	}

	// The first time a user deposits a denom we add it the user's supply interest factor index
	var supplyInterestFactors types.SupplyInterestFactors
	currDeposit, foundDeposit := k.GetDeposit(ctx, depositor)
	// On user's first deposit, build deposit index list containing denoms and current global deposit index value
	if foundDeposit {
		// If the coin denom to be deposited is not in the user's existing deposit, we add it deposit index
		for _, coin := range coins {
			if !sdk.NewCoins(coin).DenomsSubsetOf(currDeposit.Amount) {
				supplyInterestFactorValue, _ := k.GetSupplyInterestFactor(ctx, coin.Denom)
				supplyInterestFactor := types.NewSupplyInterestFactor(coin.Denom, supplyInterestFactorValue)
				supplyInterestFactors = append(supplyInterestFactors, supplyInterestFactor)
			}
		}
		// Concatenate new deposit interest factors to existing deposit interest factors
		supplyInterestFactors = append(supplyInterestFactors, currDeposit.Index...)
	} else {
		for _, coin := range coins {
			supplyInterestFactorValue, _ := k.GetSupplyInterestFactor(ctx, coin.Denom)
			supplyInterestFactor := types.NewSupplyInterestFactor(coin.Denom, supplyInterestFactorValue)
			supplyInterestFactors = append(supplyInterestFactors, supplyInterestFactor)
		}
	}

	// Calculate new deposit amount
	var amount sdk.Coins
	if foundDeposit {
		amount = currDeposit.Amount.Add(coins...)
	} else {
		amount = coins
	}

	// Update the depositer's amount and supply interest factors in the store
	deposit := types.NewDeposit(depositor, amount, supplyInterestFactors)
	k.SetDeposit(ctx, deposit)

	k.UpdateItemInLtvIndex(ctx, prevLtv, shouldRemoveIndex, depositor)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			types.EventTypeHardDeposit,
			sdk.NewAttribute(sdk.AttributeKeyAmount, coins.String()),
			sdk.NewAttribute(types.AttributeKeyDepositor, deposit.Depositor.String()),
		),
	)

	return nil
}

// ValidateDeposit validates a deposit
func (k Keeper) ValidateDeposit(ctx sdk.Context, coins sdk.Coins) error {
	params := k.GetParams(ctx)
	for _, depCoin := range coins {
		found := false
		for _, lps := range params.LiquidityProviderSchedules {
			if lps.DepositDenom == depCoin.Denom {
				found = true
			}
		}
		if !found {
			return sdkerrors.Wrapf(types.ErrInvalidDepositDenom, "liquidity provider denom %s not found", depCoin.Denom)
		}
	}

	return nil
}

// SyncSupplyInterest updates the user's earned interest on newly deposited coins to the latest global state
func (k Keeper) SyncSupplyInterest(ctx sdk.Context, addr sdk.AccAddress) {
	totalNewInterest := sdk.Coins{}

	// Update user's supply index list for each asset in the 'coins' array.
	// We use a list of SupplyInterestFactors here because Amino doesn't support marshaling maps.
	deposit, found := k.GetDeposit(ctx, addr)
	if !found {
		return
	}

	for _, coin := range deposit.Amount {
		// Locate the deposit index item by coin denom in the user's list of deposit indexes
		foundAtIndex := -1
		for i := range deposit.Index {
			if deposit.Index[i].Denom == coin.Denom {
				foundAtIndex = i
				break
			}
		}

		interestFactorValue, _ := k.GetSupplyInterestFactor(ctx, coin.Denom)
		if foundAtIndex == -1 { // First time user has supplied this denom
			deposit.Index = append(deposit.Index, types.NewSupplyInterestFactor(coin.Denom, interestFactorValue))
		} else { // User has an existing supply index for this denom
			// Calculate interest earned by user since asset's last deposit index update
			storedAmount := sdk.NewDecFromInt(deposit.Amount.AmountOf(coin.Denom))
			userLastInterestFactor := deposit.Index[foundAtIndex].Value
			interest := (storedAmount.Quo(userLastInterestFactor).Mul(interestFactorValue)).Sub(storedAmount)
			totalNewInterest = totalNewInterest.Add(sdk.NewCoin(coin.Denom, interest.TruncateInt()))
			// We're synced up, so update user's deposit index value to match the current global deposit index value
			deposit.Index[foundAtIndex].Value = interestFactorValue
		}
	}
	// Add all pending interest to user's deposit
	deposit.Amount = deposit.Amount.Add(totalNewInterest...)

	// Update user's deposit in the store
	k.SetDeposit(ctx, deposit)
}

// Withdraw returns some or all of a deposit back to original depositor
func (k Keeper) Withdraw(ctx sdk.Context, depositor sdk.AccAddress, coins sdk.Coins) error {
	// Get current stored LTV based on stored borrows/deposits
	prevLtv, shouldRemoveIndex, err := k.GetStoreLTV(ctx, depositor)
	if err != nil {
		return err
	}

	k.SyncBorrowInterest(ctx, depositor)
	k.SyncSupplyInterest(ctx, depositor)

	deposit, found := k.GetDeposit(ctx, depositor)
	if !found {
		return sdkerrors.Wrapf(types.ErrDepositNotFound, "no deposit found for %s", depositor)
	}

	borrow, found := k.GetBorrow(ctx, depositor)
	if !found {
		borrow = types.Borrow{}
	}

	proposedDepositAmount, isNegative := deposit.Amount.SafeSub(coins)
	if isNegative {
		return types.ErrNegativeBorrowedCoins
	}
	proposedDeposit := types.NewDeposit(deposit.Depositor, proposedDepositAmount, types.SupplyInterestFactors{})

	valid, err := k.IsWithinValidLtvRange(ctx, proposedDeposit, borrow)
	if err != nil {
		return err
	}

	if !valid {
		return sdkerrors.Wrapf(types.ErrInvalidWithdrawAmount, "proposed withdraw outside loan-to-value range")
	}

	err = k.supplyKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleAccountName, depositor, coins)
	if err != nil {
		return err
	}

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			types.EventTypeHardWithdrawal,
			sdk.NewAttribute(sdk.AttributeKeyAmount, coins.String()),
			sdk.NewAttribute(types.AttributeKeyDepositor, depositor.String()),
		),
	)

	if deposit.Amount.IsEqual(coins) {
		ctx.EventManager().EmitEvent(
			sdk.NewEvent(
				types.EventTypeDeleteHardDeposit,
				sdk.NewAttribute(types.AttributeKeyDepositor, depositor.String()),
			),
		)
		k.DeleteDeposit(ctx, deposit)
		return nil
	}

	deposit.Amount = deposit.Amount.Sub(coins)
	k.SetDeposit(ctx, deposit)

	k.UpdateItemInLtvIndex(ctx, prevLtv, shouldRemoveIndex, depositor)

	return nil
}

// GetTotalDeposited returns the total amount deposited for the input deposit type and deposit denom
func (k Keeper) GetTotalDeposited(ctx sdk.Context, depositDenom string) (total sdk.Int) {
	var macc supplyExported.ModuleAccountI
	macc = k.supplyKeeper.GetModuleAccount(ctx, types.ModuleAccountName)
	return macc.GetCoins().AmountOf(depositDenom)
}
