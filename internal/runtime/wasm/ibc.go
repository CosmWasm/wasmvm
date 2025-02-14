package wasm

import (
	"encoding/json"
	"fmt"

	"github.com/CosmWasm/wasmvm/v2/types"
)

// IBCChannelOpen calls the contract's "ibc_channel_open" entry point.
func (vm *WazeroVM) IBCChannelOpen(checksum Checksum, env types.Env, msg types.IBCChannelOpenMsg, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64) (*types.IBCChannelOpenResult, uint64, error) {
	envBz, err := json.Marshal(env)
	if err != nil {
		return nil, 0, err
	}
	msgBz, err := json.Marshal(msg)
	if err != nil {
		return nil, 0, err
	}
	resBz, gasUsed, execErr := vm.callContract(checksum, "ibc_channel_open", envBz, nil, msgBz, store, api, querier, gasMeter, gasLimit)
	if execErr != nil {
		return nil, gasUsed, execErr
	}
	var result types.IBCChannelOpenResult
	if err := json.Unmarshal(resBz, &result); err != nil {
		return nil, gasUsed, fmt.Errorf("cannot deserialize IBCChannelOpenResult: %w", err)
	}
	return &result, gasUsed, nil
}

// IBCChannelConnect calls "ibc_channel_connect" entry point.
func (vm *WazeroVM) IBCChannelConnect(checksum Checksum, env types.Env, msg types.IBCChannelConnectMsg, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64) (*types.IBCBasicResult, uint64, error) {
	envBz, err := json.Marshal(env)
	if err != nil {
		return nil, 0, err
	}
	msgBz, err := json.Marshal(msg)
	if err != nil {
		return nil, 0, err
	}
	resBz, gasUsed, execErr := vm.callContract(checksum, "ibc_channel_connect", envBz, nil, msgBz, store, api, querier, gasMeter, gasLimit)
	if execErr != nil {
		return nil, gasUsed, execErr
	}
	var result types.IBCBasicResult
	if err := json.Unmarshal(resBz, &result); err != nil {
		return nil, gasUsed, fmt.Errorf("cannot deserialize IBCChannelConnectResult: %w", err)
	}
	return &result, gasUsed, nil
}

// IBCChannelClose calls "ibc_channel_close".
func (vm *WazeroVM) IBCChannelClose(checksum Checksum, env types.Env, msg types.IBCChannelCloseMsg, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64) (*types.IBCBasicResult, uint64, error) {
	envBz, err := json.Marshal(env)
	if err != nil {
		return nil, 0, err
	}
	msgBz, err := json.Marshal(msg)
	if err != nil {
		return nil, 0, err
	}
	resBz, gasUsed, execErr := vm.callContract(checksum, "ibc_channel_close", envBz, nil, msgBz, store, api, querier, gasMeter, gasLimit)
	if execErr != nil {
		return nil, gasUsed, execErr
	}
	var result types.IBCBasicResult
	if err := json.Unmarshal(resBz, &result); err != nil {
		return nil, gasUsed, fmt.Errorf("cannot deserialize IBCChannelCloseResult: %w", err)
	}
	return &result, gasUsed, nil
}

// IBCPacketReceive calls "ibc_packet_receive".
func (vm *WazeroVM) IBCPacketReceive(checksum Checksum, env types.Env, msg types.IBCPacketReceiveMsg, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64) (*types.IBCReceiveResult, uint64, error) {
	envBz, err := json.Marshal(env)
	if err != nil {
		return nil, 0, err
	}
	msgBz, err := json.Marshal(msg)
	if err != nil {
		return nil, 0, err
	}
	resBz, gasUsed, execErr := vm.callContract(checksum, "ibc_packet_receive", envBz, nil, msgBz, store, api, querier, gasMeter, gasLimit)
	if execErr != nil {
		return nil, gasUsed, execErr
	}
	var result types.IBCReceiveResult
	if err := json.Unmarshal(resBz, &result); err != nil {
		return nil, gasUsed, fmt.Errorf("cannot deserialize IBCPacketReceiveResult: %w", err)
	}
	return &result, gasUsed, nil
}

// IBCPacketAck calls "ibc_packet_ack".
func (vm *WazeroVM) IBCPacketAck(checksum Checksum, env types.Env, msg types.IBCPacketAckMsg, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64) (*types.IBCBasicResult, uint64, error) {
	envBz, err := json.Marshal(env)
	if err != nil {
		return nil, 0, err
	}
	msgBz, err := json.Marshal(msg)
	if err != nil {
		return nil, 0, err
	}
	resBz, gasUsed, execErr := vm.callContract(checksum, "ibc_packet_ack", envBz, nil, msgBz, store, api, querier, gasMeter, gasLimit)
	if execErr != nil {
		return nil, gasUsed, execErr
	}
	var result types.IBCBasicResult
	if err := json.Unmarshal(resBz, &result); err != nil {
		return nil, gasUsed, fmt.Errorf("cannot deserialize IBCPacketAckResult: %w", err)
	}
	return &result, gasUsed, nil
}

// IBCPacketTimeout calls "ibc_packet_timeout".
func (vm *WazeroVM) IBCPacketTimeout(checksum Checksum, env types.Env, msg types.IBCPacketTimeoutMsg, store types.KVStore, api types.GoAPI, querier types.Querier, gasMeter types.GasMeter, gasLimit uint64) (*types.IBCBasicResult, uint64, error) {
	envBz, err := json.Marshal(env)
	if err != nil {
		return nil, 0, err
	}
	msgBz, err := json.Marshal(msg)
	if err != nil {
		return nil, 0, err
	}
	resBz, gasUsed, execErr := vm.callContract(checksum, "ibc_packet_timeout", envBz, nil, msgBz, store, api, querier, gasMeter, gasLimit)
	if execErr != nil {
		return nil, gasUsed, execErr
	}
	var result types.IBCBasicResult
	if err := json.Unmarshal(resBz, &result); err != nil {
		return nil, gasUsed, fmt.Errorf("cannot deserialize IBCPacketTimeoutResult: %w", err)
	}
	return &result, gasUsed, nil
}
