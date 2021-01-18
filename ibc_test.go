package cosmwasm

import (
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"testing"
)

const IBC_TEST_CONTRACT = "./api/testdata/ibc_reflect.wasm"

func TestIBC(t *testing.T) {
	vm := withVM(t)

	wasm, err := ioutil.ReadFile(IBC_TEST_CONTRACT)
	require.NoError(t, err)

	id, err := vm.Create(wasm)
	require.NoError(t, err)

	code, err := vm.GetCode(id)
	require.NoError(t, err)
	require.Equal(t, WasmCode(wasm), code)
}
