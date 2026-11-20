package keeper_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"cosmossdk.io/collections"

	"github.com/cosmos/cosmos-sdk/codec/address"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/gov"
	"github.com/cosmos/cosmos-sdk/x/gov/keeper"
	"github.com/cosmos/cosmos-sdk/x/gov/types"
	v1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
)

var _ types.GovHooks = &MockGovHooksReceiver{}

// GovHooks event hooks for governance proposal object (noalias)
type MockGovHooksReceiver struct {
	AfterProposalSubmissionValid        bool
	AfterProposalDepositValid           bool
	AfterProposalVoteValid              bool
	AfterProposalFailedMinDepositValid  bool
	AfterProposalVotingPeriodEndedValid bool
	AfterProposalCanceledValid          bool

	afterProposalCanceled func(context.Context, uint64) error
}

func (h *MockGovHooksReceiver) AfterProposalSubmission(ctx context.Context, proposalID uint64) error {
	h.AfterProposalSubmissionValid = true
	return nil
}

func (h *MockGovHooksReceiver) AfterProposalDeposit(ctx context.Context, proposalID uint64, depositorAddr sdk.AccAddress) error {
	h.AfterProposalDepositValid = true
	return nil
}

func (h *MockGovHooksReceiver) AfterProposalVote(ctx context.Context, proposalID uint64, voterAddr sdk.AccAddress) error {
	h.AfterProposalVoteValid = true
	return nil
}

func (h *MockGovHooksReceiver) AfterProposalFailedMinDeposit(ctx context.Context, proposalID uint64) error {
	h.AfterProposalFailedMinDepositValid = true
	return nil
}

func (h *MockGovHooksReceiver) AfterProposalVotingPeriodEnded(ctx context.Context, proposalID uint64) error {
	h.AfterProposalVotingPeriodEndedValid = true
	return nil
}

func (h *MockGovHooksReceiver) AfterProposalCanceled(ctx context.Context, proposalID uint64) error {
	h.AfterProposalCanceledValid = true
	if h.afterProposalCanceled != nil {
		return h.afterProposalCanceled(ctx, proposalID)
	}
	return nil
}

func TestHooks(t *testing.T) {
	minDeposit := v1.DefaultParams().MinDeposit
	govKeeper, authKeeper, bankKeeper, stakingKeeper, _, _, ctx := setupGovKeeper(t)
	addrs := simtestutil.AddTestAddrs(bankKeeper, stakingKeeper, ctx, 1, minDeposit[0].Amount)

	authKeeper.EXPECT().AddressCodec().Return(address.NewBech32Codec("cosmos")).AnyTimes()
	stakingKeeper.EXPECT().ValidatorAddressCodec().Return(address.NewBech32Codec("cosmosvaloper")).AnyTimes()

	govHooksReceiver := MockGovHooksReceiver{}

	keeper.UnsafeSetHooks(
		govKeeper, types.NewMultiGovHooks(&govHooksReceiver),
	)

	require.False(t, govHooksReceiver.AfterProposalSubmissionValid)
	require.False(t, govHooksReceiver.AfterProposalDepositValid)
	require.False(t, govHooksReceiver.AfterProposalVoteValid)
	require.False(t, govHooksReceiver.AfterProposalFailedMinDepositValid)
	require.False(t, govHooksReceiver.AfterProposalVotingPeriodEndedValid)
	require.False(t, govHooksReceiver.AfterProposalCanceledValid)

	proposer := sdk.AccAddress("cosmos1ghekyjucln7y67ntx7cf27m9dpuxxemn4c8g4r")
	tp := TestProposal
	_, err := govKeeper.SubmitProposal(ctx, tp, "", "test", "summary", proposer, false)
	require.NoError(t, err)
	require.True(t, govHooksReceiver.AfterProposalSubmissionValid)

	params, _ := govKeeper.Params.Get(ctx)
	newHeader := ctx.BlockHeader()
	newHeader.Time = ctx.BlockHeader().Time.Add(*params.MaxDepositPeriod).Add(time.Duration(1) * time.Second)
	ctx = ctx.WithBlockHeader(newHeader)
	require.NoError(t, gov.EndBlocker(ctx, govKeeper))

	require.True(t, govHooksReceiver.AfterProposalFailedMinDepositValid)

	p2, err := govKeeper.SubmitProposal(ctx, tp, "", "test", "summary", proposer, false)
	require.NoError(t, err)

	activated, err := govKeeper.AddDeposit(ctx, p2.Id, addrs[0], minDeposit)
	require.True(t, activated)
	require.NoError(t, err)
	require.True(t, govHooksReceiver.AfterProposalDepositValid)

	err = govKeeper.AddVote(ctx, p2.Id, addrs[0], v1.NewNonSplitVoteOption(v1.OptionYes), "")
	require.NoError(t, err)
	require.True(t, govHooksReceiver.AfterProposalVoteValid)

	newHeader = ctx.BlockHeader()
	newHeader.Time = ctx.BlockHeader().Time.Add(*params.VotingPeriod).Add(time.Duration(1) * time.Second)
	ctx = ctx.WithBlockHeader(newHeader)
	require.NoError(t, gov.EndBlocker(ctx, govKeeper))
	require.True(t, govHooksReceiver.AfterProposalVotingPeriodEndedValid)

	p3, err := govKeeper.SubmitProposal(ctx, tp, "", "test", "summary", proposer, false)
	require.NoError(t, err)
	require.False(t, govHooksReceiver.AfterProposalCanceledValid)
	require.NoError(t, govKeeper.CancelProposal(ctx, p3.Id, proposer.String()))
	require.True(t, govHooksReceiver.AfterProposalCanceledValid)
}

func TestCancelProposalHook(t *testing.T) {
	t.Run("before_deletion", func(t *testing.T) {
		for _, tc := range []struct {
			name   string
			voting bool
		}{
			{name: "deposit"},
			{name: "voting", voting: true},
		} {
			t.Run(tc.name, func(t *testing.T) {
				k, _, _, _, _, _, ctx := setupGovKeeper(t)
				proposer := sdk.AccAddress("proposer____________")
				proposal, err := k.SubmitProposal(ctx, TestProposal, "", "title", "summary", proposer, false)
				require.NoError(t, err)
				if tc.voting {
					require.NoError(t, k.ActivateVotingPeriod(ctx, proposal))
				}

				calls := 0
				k.SetHooks(&MockGovHooksReceiver{afterProposalCanceled: func(hookCtx context.Context, id uint64) error {
					calls++
					require.Equal(t, proposal.Id, id)
					_, err := k.Proposals.Get(hookCtx, id)
					require.NoError(t, err)
					return nil
				}})

				msgServer := keeper.NewMsgServerImpl(k)
				msg := v1.NewMsgCancelProposal(proposal.Id, proposer.String())
				_, err = msgServer.CancelProposal(ctx, msg)
				require.NoError(t, err)
				require.Equal(t, 1, calls)
				_, err = k.Proposals.Get(ctx, proposal.Id)
				require.ErrorIs(t, err, collections.ErrNotFound)

				_, err = msgServer.CancelProposal(ctx, msg)
				require.Error(t, err)
				require.Equal(t, 1, calls)
			})
		}
	})

	t.Run("hook_error", func(t *testing.T) {
		k, _, _, _, _, _, ctx := setupGovKeeper(t)
		proposer := sdk.AccAddress("proposer____________")
		proposal, err := k.SubmitProposal(ctx, TestProposal, "", "title", "summary", proposer, false)
		require.NoError(t, err)

		hookErr := errors.New("refund failed")
		k.SetHooks(&MockGovHooksReceiver{afterProposalCanceled: func(context.Context, uint64) error {
			return hookErr
		}})
		cache, _ := ctx.CacheContext()
		_, err = keeper.NewMsgServerImpl(k).CancelProposal(cache, v1.NewMsgCancelProposal(proposal.Id, proposer.String()))
		require.Same(t, hookErr, err)
		retained, err := k.Proposals.Get(cache, proposal.Id)
		require.NoError(t, err)
		require.Equal(t, proposal, retained)
	})
}

func TestMultiGovHooksDispatchesCancellation(t *testing.T) {
	firstErr, lastErr := errors.New("first refund"), errors.New("last refund")
	var ids []uint64
	first := &MockGovHooksReceiver{afterProposalCanceled: func(_ context.Context, id uint64) error {
		ids = append(ids, id)
		return firstErr
	}}
	last := &MockGovHooksReceiver{afterProposalCanceled: func(_ context.Context, id uint64) error {
		ids = append(ids, id)
		return lastErr
	}}
	receiver := &MockGovHooksReceiver{}

	err := types.NewMultiGovHooks(first, receiver, last).AfterProposalCanceled(context.Background(), 7)
	require.ErrorIs(t, err, firstErr)
	require.ErrorIs(t, err, lastErr)
	require.Equal(t, []uint64{7, 7}, ids)
	require.True(t, receiver.AfterProposalCanceledValid)
}
