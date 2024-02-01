package types

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReplySerialization(t *testing.T) {
	reply1 := Reply{
		GasUsed: 4312324,
		ID:      75,
		Result: SubMsgResult{
			Ok: &SubMsgResponse{
				Events: []Event{
					{
						Type: "hi",
						Attributes: []EventAttribute{
							{
								Key:   "si",
								Value: "claro",
							},
						},
					},
				},
				Data: []byte{0x3f, 0x00, 0xaa, 0x5c, 0xab},
			},
		},
	}
	serialized, err := json.Marshal(&reply1)
	require.NoError(t, err)
	require.Equal(t, `{"gas_used":4312324,"id":75,"result":{"ok":{"events":[{"type":"hi","attributes":[{"key":"si","value":"claro"}]}],"data":"PwCqXKs="}}}`, string(serialized))
}
