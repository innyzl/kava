package types

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/params"

	cdptypes "github.com/kava-labs/kava/x/cdp/types"
)

// Parameter keys and default values
var (
	KeyActive                 = []byte("Active")
	KeyMoneyMarkets           = []byte("MoneyMarkets")
	KeyCheckLtvIndexCount     = []byte("CheckLtvIndexCount")
	DefaultActive             = true
	DefaultMoneyMarkets       = MoneyMarkets{}
	DefaultCheckLtvIndexCount = 10
	GovDenom                  = cdptypes.DefaultGovDenom
)

// Params governance parameters for hard module
type Params struct {
	Active             bool         `json:"active" yaml:"active"`
	MoneyMarkets       MoneyMarkets `json:"money_markets" yaml:"money_markets"`
	CheckLtvIndexCount int          `json:"check_ltv_index_count" yaml:"check_ltv_index_count"`
}

// Multiplier amount the claim rewards get increased by, along with how long the claim rewards are locked
type Multiplier struct {
	Name         MultiplierName `json:"name" yaml:"name"`
	MonthsLockup int64          `json:"months_lockup" yaml:"months_lockup"`
	Factor       sdk.Dec        `json:"factor" yaml:"factor"`
}

// NewMultiplier returns a new Multiplier
func NewMultiplier(name MultiplierName, lockup int64, factor sdk.Dec) Multiplier {
	return Multiplier{
		Name:         name,
		MonthsLockup: lockup,
		Factor:       factor,
	}
}

// Validate multiplier param
func (m Multiplier) Validate() error {
	if err := m.Name.IsValid(); err != nil {
		return err
	}
	if m.MonthsLockup < 0 {
		return fmt.Errorf("expected non-negative lockup, got %d", m.MonthsLockup)
	}
	if m.Factor.IsNegative() {
		return fmt.Errorf("expected non-negative factor, got %s", m.Factor.String())
	}

	return nil
}

// Multipliers slice of Multiplier
type Multipliers []Multiplier

// BorrowLimit enforces restrictions on a money market
type BorrowLimit struct {
	HasMaxLimit  bool    `json:"has_max_limit" yaml:"has_max_limit"`
	MaximumLimit sdk.Dec `json:"maximum_limit" yaml:"maximum_limit"`
	LoanToValue  sdk.Dec `json:"loan_to_value" yaml:"loan_to_value"`
}

// NewBorrowLimit returns a new BorrowLimit
func NewBorrowLimit(hasMaxLimit bool, maximumLimit, loanToValue sdk.Dec) BorrowLimit {
	return BorrowLimit{
		HasMaxLimit:  hasMaxLimit,
		MaximumLimit: maximumLimit,
		LoanToValue:  loanToValue,
	}
}

// Validate BorrowLimit
func (bl BorrowLimit) Validate() error {
	if bl.MaximumLimit.IsNegative() {
		return fmt.Errorf("maximum limit USD cannot be negative: %s", bl.MaximumLimit)
	}
	if !bl.LoanToValue.IsPositive() {
		return fmt.Errorf("loan-to-value must be a positive integer: %s", bl.LoanToValue)
	}
	if bl.LoanToValue.GT(sdk.OneDec()) {
		return fmt.Errorf("loan-to-value cannot be greater than 1.0: %s", bl.LoanToValue)
	}
	return nil
}

// Equal returns a boolean indicating if an BorrowLimit is equal to another BorrowLimit
func (bl BorrowLimit) Equal(blCompareTo BorrowLimit) bool {
	if bl.HasMaxLimit != blCompareTo.HasMaxLimit {
		return false
	}
	if !bl.MaximumLimit.Equal(blCompareTo.MaximumLimit) {
		return false
	}
	if !bl.LoanToValue.Equal(blCompareTo.LoanToValue) {
		return false
	}
	return true
}

// MoneyMarket is a money market for an individual asset
type MoneyMarket struct {
	Denom                  string            `json:"denom" yaml:"denom"`
	BorrowLimit            BorrowLimit       `json:"borrow_limit" yaml:"borrow_limit"`
	SpotMarketID           string            `json:"spot_market_id" yaml:"spot_market_id"`
	ConversionFactor       sdk.Int           `json:"conversion_factor" yaml:"conversion_factor"`
	InterestRateModel      InterestRateModel `json:"interest_rate_model" yaml:"interest_rate_model"`
	ReserveFactor          sdk.Dec           `json:"reserve_factor" yaml:"reserve_factor"`
	AuctionSize            sdk.Int           `json:"auction_size" yaml:"auction_size"`
	KeeperRewardPercentage sdk.Dec           `json:"keeper_reward_percentage" yaml:"keeper_reward_percentages"`
}

// NewMoneyMarket returns a new MoneyMarket
func NewMoneyMarket(denom string, borrowLimit BorrowLimit, spotMarketID string, conversionFactor,
	auctionSize sdk.Int, interestRateModel InterestRateModel, reserveFactor, keeperRewardPercentage sdk.Dec) MoneyMarket {
	return MoneyMarket{
		Denom:                  denom,
		BorrowLimit:            borrowLimit,
		SpotMarketID:           spotMarketID,
		ConversionFactor:       conversionFactor,
		AuctionSize:            auctionSize,
		InterestRateModel:      interestRateModel,
		ReserveFactor:          reserveFactor,
		KeeperRewardPercentage: keeperRewardPercentage,
	}
}

// Validate MoneyMarket param
func (mm MoneyMarket) Validate() error {
	if err := sdk.ValidateDenom(mm.Denom); err != nil {
		return err
	}

	if err := mm.BorrowLimit.Validate(); err != nil {
		return err
	}

	if err := mm.InterestRateModel.Validate(); err != nil {
		return err
	}

	if mm.ReserveFactor.IsNegative() || mm.ReserveFactor.GT(sdk.OneDec()) {
		return fmt.Errorf("Reserve factor must be between 0.0-1.0")
	}

	if !mm.AuctionSize.IsPositive() {
		return fmt.Errorf("Auction size must be a positive integer")
	}

	if mm.KeeperRewardPercentage.IsNegative() || mm.KeeperRewardPercentage.GT(sdk.OneDec()) {
		return fmt.Errorf("Keeper reward percentage must be between 0.0-1.0")
	}

	return nil
}

// Equal returns a boolean indicating if a MoneyMarket is equal to another MoneyMarket
func (mm MoneyMarket) Equal(mmCompareTo MoneyMarket) bool {
	if mm.Denom != mmCompareTo.Denom {
		return false
	}
	if !mm.BorrowLimit.Equal(mmCompareTo.BorrowLimit) {
		return false
	}
	if mm.SpotMarketID != mmCompareTo.SpotMarketID {
		return false
	}
	if !mm.ConversionFactor.Equal(mmCompareTo.ConversionFactor) {
		return false
	}
	if !mm.InterestRateModel.Equal(mmCompareTo.InterestRateModel) {
		return false
	}
	if !mm.ReserveFactor.Equal(mmCompareTo.ReserveFactor) {
		return false
	}
	if !mm.AuctionSize.Equal(mmCompareTo.AuctionSize) {
		return false
	}
	if !mm.KeeperRewardPercentage.Equal(mmCompareTo.KeeperRewardPercentage) {
		return false
	}
	return true
}

// MoneyMarkets slice of MoneyMarket
type MoneyMarkets []MoneyMarket

// Validate borrow limits
func (mms MoneyMarkets) Validate() error {
	for _, moneyMarket := range mms {
		if err := moneyMarket.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// InterestRateModel contains information about an asset's interest rate
type InterestRateModel struct {
	BaseRateAPY    sdk.Dec `json:"base_rate_apy" yaml:"base_rate_apy"`
	BaseMultiplier sdk.Dec `json:"base_multiplier" yaml:"base_multiplier"`
	Kink           sdk.Dec `json:"kink" yaml:"kink"`
	JumpMultiplier sdk.Dec `json:"jump_multiplier" yaml:"jump_multiplier"`
}

// NewInterestRateModel returns a new InterestRateModel
func NewInterestRateModel(baseRateAPY, baseMultiplier, kink, jumpMultiplier sdk.Dec) InterestRateModel {
	return InterestRateModel{
		BaseRateAPY:    baseRateAPY,
		BaseMultiplier: baseMultiplier,
		Kink:           kink,
		JumpMultiplier: jumpMultiplier,
	}
}

// Validate InterestRateModel param
func (irm InterestRateModel) Validate() error {
	if irm.BaseRateAPY.IsNegative() || irm.BaseRateAPY.GT(sdk.OneDec()) {
		return fmt.Errorf("Base rate APY must be between 0.0-1.0")
	}

	if irm.BaseMultiplier.IsNegative() {
		return fmt.Errorf("Base multiplier must be positive")
	}

	if irm.Kink.IsNegative() || irm.Kink.GT(sdk.OneDec()) {
		return fmt.Errorf("Kink must be between 0.0-1.0")
	}

	if irm.JumpMultiplier.IsNegative() {
		return fmt.Errorf("Jump multiplier must be positive")
	}

	return nil
}

// Equal returns a boolean indicating if an InterestRateModel is equal to another InterestRateModel
func (irm InterestRateModel) Equal(irmCompareTo InterestRateModel) bool {
	if !irm.BaseRateAPY.Equal(irmCompareTo.BaseRateAPY) {
		return false
	}
	if !irm.BaseMultiplier.Equal(irmCompareTo.BaseMultiplier) {
		return false
	}
	if !irm.Kink.Equal(irmCompareTo.Kink) {
		return false
	}
	if !irm.JumpMultiplier.Equal(irmCompareTo.JumpMultiplier) {
		return false
	}
	return true
}

// InterestRateModels slice of InterestRateModel
type InterestRateModels []InterestRateModel

// NewParams returns a new params object
func NewParams(active bool, moneyMarkets MoneyMarkets, checkLtvIndexCount int) Params {
	return Params{
		Active:             active,
		MoneyMarkets:       moneyMarkets,
		CheckLtvIndexCount: checkLtvIndexCount,
	}
}

// DefaultParams returns default params for hard module
func DefaultParams() Params {
	return NewParams(DefaultActive, DefaultMoneyMarkets, DefaultCheckLtvIndexCount)
}

// String implements fmt.Stringer
func (p Params) String() string {
	return fmt.Sprintf(`Params:
	Active: %t
	Money Markets %v
	Check LTV Index Count: %v`,
		p.Active, p.MoneyMarkets, p.CheckLtvIndexCount)
}

// ParamKeyTable Key declaration for parameters
func ParamKeyTable() params.KeyTable {
	return params.NewKeyTable().RegisterParamSet(&Params{})
}

// ParamSetPairs implements the ParamSet interface and returns all the key/value pairs
func (p *Params) ParamSetPairs() params.ParamSetPairs {
	return params.ParamSetPairs{
		params.NewParamSetPair(KeyActive, &p.Active, validateActiveParam),
		params.NewParamSetPair(KeyMoneyMarkets, &p.MoneyMarkets, validateMoneyMarketParams),
		params.NewParamSetPair(KeyCheckLtvIndexCount, &p.CheckLtvIndexCount, validateCheckLtvIndexCount),
	}
}

// Validate checks that the parameters have valid values.
func (p Params) Validate() error {
	if err := validateActiveParam(p.Active); err != nil {
		return err
	}

	if err := validateMoneyMarketParams(p.MoneyMarkets); err != nil {
		return err
	}

	return validateCheckLtvIndexCount(p.CheckLtvIndexCount)
}

func validateActiveParam(i interface{}) error {
	_, ok := i.(bool)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}

	return nil
}

func validateMoneyMarketParams(i interface{}) error {
	mm, ok := i.(MoneyMarkets)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}

	return mm.Validate()
}

func validateCheckLtvIndexCount(i interface{}) error {
	ltvCheckCount, ok := i.(int)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}

	if ltvCheckCount < 0 {
		return fmt.Errorf("CheckLtvIndexCount param must be positive, got: %d", ltvCheckCount)
	}

	return nil
}
