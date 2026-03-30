package keeper

import (
	"context"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/productscience/inference/x/inference/types"
)

const SubnetPruningThreshold = uint64(2)
const SubnetPruningMax = int64(100)

// distributeUnsettledEscrow splits the escrowed funds equally among unique validators in the group.
// Integer division remainder stays in the module account.
// Uses CacheContext for atomicity: either all payments succeed or none are committed,
// preventing double-payouts on retry.
func (k Keeper) distributeUnsettledEscrow(ctx context.Context, escrow types.SubnetEscrow) error {
	// Count unique addresses (first pass)
	seen := make(map[string]bool)
	var uniqueCount uint64
	for _, addr := range escrow.Slots {
		if !seen[addr] {
			seen[addr] = true
			uniqueCount++
		}
	}

	if uniqueCount == 0 {
		return nil
	}

	share := escrow.Amount / uniqueCount
	if share == 0 {
		return nil
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	cacheCtx, commit := sdkCtx.CacheContext()

	// Pay in slot order (deterministic iteration over escrow.Slots)
	paid := make(map[string]bool)
	for _, addr := range escrow.Slots {
		if paid[addr] {
			continue
		}
		paid[addr] = true

		recipient, err := sdk.AccAddressFromBech32(addr)
		if err != nil {
			return fmt.Errorf("invalid address %s in unsettled escrow %d: %w", addr, escrow.Id, err)
		}
		coins, err := types.GetCoins(int64(share))
		if err != nil {
			return fmt.Errorf("invalid share amount for unsettled escrow %d: %w", escrow.Id, err)
		}
		err = k.BankKeeper.SendCoinsFromModuleToAccount(cacheCtx, types.ModuleName, recipient, coins, "subnet_escrow_unsettled_distribution")
		if err != nil {
			return fmt.Errorf("failed to distribute unsettled escrow %d to %s: %w", escrow.Id, addr, err)
		}
	}

	commit()
	return nil
}
