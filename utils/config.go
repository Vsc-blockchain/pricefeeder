package utils

import (
	"sync"

	sdk "github.com/cosmos/cosmos-sdk/types"
	vsctypes "github.com/vsc-blockchain/core/types"
	coreutils "github.com/vsc-blockchain/core/utils"
)

// InitSDKConfig sets the chain's bech32 prefixes on the global sdk config and
// seals it. Sealing is one-way, so the work is done once: a second call would
// otherwise panic, which is a problem for any test binary that initialises more
// than once.
func InitSDKConfig() {
	initSDKConfigOnce.Do(initSDKConfig)
}

var initSDKConfigOnce sync.Once

func initSDKConfig() {
	// Set prefixes
	accountPubKeyPrefix := coreutils.AccountAddressPrefix + "pub"
	validatorAddressPrefix := coreutils.AccountAddressPrefix + "valoper"
	validatorPubKeyPrefix := coreutils.AccountAddressPrefix + "valoperpub"
	consNodeAddressPrefix := coreutils.AccountAddressPrefix + "valcons"
	consNodePubKeyPrefix := coreutils.AccountAddressPrefix + "valconspub"

	sdk.DefaultPowerReduction = vsctypes.PowerReduction

	// Set and seal config
	config := sdk.GetConfig()
	config.SetBech32PrefixForAccount(coreutils.AccountAddressPrefix, accountPubKeyPrefix)
	config.SetBech32PrefixForValidator(validatorAddressPrefix, validatorPubKeyPrefix)
	config.SetBech32PrefixForConsensusNode(consNodeAddressPrefix, consNodePubKeyPrefix)
	config.Seal()
}
