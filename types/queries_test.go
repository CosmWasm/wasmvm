package types

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDelegationWithEmptyArray(t *testing.T) {
	var del Delegations
	bz, err := json.Marshal(&del)
	require.NoError(t, err)
	assert.Equal(t, string(bz), `[]`)

	var redel Delegations
	err = json.Unmarshal(bz, &redel)
	require.NoError(t, err)
	assert.Nil(t, redel)
}

func TestDelegationWithData(t *testing.T) {
	del := Delegations{{
		Validator: "foo",
		Delegator: "bar",
		Amount:    NewCoin(123, "stake"),
	}}
	bz, err := json.Marshal(&del)
	require.NoError(t, err)

	var redel Delegations
	err = json.Unmarshal(bz, &redel)
	require.NoError(t, err)
	assert.Equal(t, redel, del)
}

func TestValidatorWithEmptyArray(t *testing.T) {
	var val Validators
	bz, err := json.Marshal(&val)
	require.NoError(t, err)
	assert.Equal(t, string(bz), `[]`)

	var reval Validators
	err = json.Unmarshal(bz, &reval)
	require.NoError(t, err)
	assert.Nil(t, reval)
}

func TestValidatorWithData(t *testing.T) {
	val := Validators{{
		Address:       "1234567890",
		Commission:    "0.05",
		MaxCommission: "0.1",
		MaxChangeRate: "0.02",
	}}
	bz, err := json.Marshal(&val)
	require.NoError(t, err)

	var reval Validators
	err = json.Unmarshal(bz, &reval)
	require.NoError(t, err)
	assert.Equal(t, reval, val)
}

func TestQueryResponseWithEmptyData(t *testing.T) {
	cases := map[string]struct {
		req       QueryResponse
		resp      string
		unmarshal bool
	}{
		"ok with data": {
			req: QueryResponse{Ok: []byte("foo")},
			// base64-encoded "foo"
			resp:      `{"ok":"Zm9v"}`,
			unmarshal: true,
		},
		"error": {
			req:       QueryResponse{Err: "try again later"},
			resp:      `{"error":"try again later"}`,
			unmarshal: true,
		},
		"ok with empty slice": {
			req:       QueryResponse{Ok: []byte{}},
			resp:      `{"ok":""}`,
			unmarshal: true,
		},
		"nil data": {
			req:  QueryResponse{},
			resp: `{"ok":""}`,
			// Once converted to the Rust enum `ContractResult<Binary>` or
			// its JSON serialization, we cannot differentiate between
			// nil and an empty slice anymore. As a consequence,
			// only this or the above deserialization test can be executed.
			// We prefer empty slice over nil for no reason.
			unmarshal: false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			data, err := json.Marshal(tc.req)
			require.NoError(t, err)
			require.Equal(t, tc.resp, string(data))

			// if unmarshall, make sure this comes back to the proper state
			if tc.unmarshal {
				var parsed QueryResponse
				err = json.Unmarshal(data, &parsed)
				require.NoError(t, err)
				require.Equal(t, tc.req, parsed)
			}
		})
	}
}
