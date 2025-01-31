package types

import (
	"github.com/vsc-blockchain/core/x/common/asset"
	oracletypes "github.com/vsc-blockchain/core/x/oracle/types"
)

// ParamsFromOracleParams converts oracletypes.Params into
// Params. Panics on invalid whitelist pairs.
func ParamsFromOracleParams(p oracletypes.Params) Params {
	pairs := make([]asset.Pair, len(p.Whitelist))
	for i, pair := range p.Whitelist {
		pair := pair
		pairs[i] = pair
	}
	return Params{
		Pairs:            pairs,
		VotePeriodBlocks: p.VotePeriod,
	}
}

// Params is the x/oracle specific subset of parameters required for price feeding.
type Params struct {
	// Pairs are the symbols we need to provide prices for.
	Pairs []asset.Pair
	// VotePeriodBlocks is how
	VotePeriodBlocks uint64
}

func (p Params) Equal(params Params) bool {
	if p.VotePeriodBlocks != params.VotePeriodBlocks {
		return false
	}
	if len(p.Pairs) != len(params.Pairs) {
		return false
	}
	for i, pair := range p.Pairs {
		if pair != params.Pairs[i] {
			return false
		}
	}
	return true
}
