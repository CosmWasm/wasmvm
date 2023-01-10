package types

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGovMsgVoteSerialization(t *testing.T) {
	document := []byte(`{"vote":{"proposal_id":4,"vote":"no_with_veto"}}`)

	var msg GovMsg
	err := json.Unmarshal(document, &msg)
	require.NoError(t, err)

	require.Nil(t, msg.VoteWeighted)
	require.NotNil(t, msg.Vote)

	require.Equal(t, uint64(4), msg.Vote.ProposalId)
	require.Equal(t, NoWithVeto, msg.Vote.Vote)
}

func TestGovMsgVoteWeightedSerialization(t *testing.T) {
	document := []byte(`{"vote_weighted":{"proposal_id":25,"options":[{"option":"yes","weight":"0.25"},{"option":"no","weight":"0.25"},{"option":"abstain","weight":"0.5"}]}}`)

	var msg GovMsg
	err := json.Unmarshal(document, &msg)
	require.NoError(t, err)

	require.Nil(t, msg.Vote)
	require.NotNil(t, msg.VoteWeighted)

	require.Equal(t, uint64(25), msg.VoteWeighted.ProposalId)
	require.Equal(t, []WeightedVoteOption{
		{Yes, "0.25"},
		{No, "0.25"},
		{Abstain, "0.5"},
	}, msg.VoteWeighted.Options)
}
