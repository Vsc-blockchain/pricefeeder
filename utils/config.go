package utils

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	vsctypes "github.com/vsc-blockchain/core/types"
	coreutils "github.com/vsc-blockchain/core/utils"
)

func InitSDKConfig() {
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
