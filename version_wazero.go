//go:build wazero

package cosmwasm

func libwasmvmVersionImpl() (string, error) {
	return "wazero", nil
}
