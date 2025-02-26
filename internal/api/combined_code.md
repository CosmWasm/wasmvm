# Combined Code Files

## TOC
- [`api_test.go`](#file-1)
- [`callbacks.go`](#file-2)
- [`callbacks_cgo.go`](#file-3)
- [`iterator.go`](#file-4)
- [`iterator_test.go`](#file-5)
- [`lib.go`](#file-6)
- [`lib_test.go`](#file-7)
- [`link_glibclinux_aarch64.go`](#file-8)
- [`link_glibclinux_x86_64.go`](#file-9)
- [`link_mac.go`](#file-10)
- [`link_mac_static.go`](#file-11)
- [`link_muslc_aarch64.go`](#file-12)
- [`link_muslc_x86_64.go`](#file-13)
- [`link_system.go`](#file-14)
- [`link_windows.go`](#file-15)
- [`memory.go`](#file-16)
- [`memory_test.go`](#file-17)
- [`mock_failure.go`](#file-18)
- [`mocks.go`](#file-19)
- [`testdb/README.md`](#file-20)
- [`testdb/memdb.go`](#file-21)
- [`testdb/memdb_iterator.go`](#file-22)
- [`testdb/types.go`](#file-23)
- [`version.go`](#file-24)
- [`version_test.go`](#file-25)

---

### `api_test.go`
*2025-02-15 10:29:18 | 13 KB*
```go
// api_test.go

package api

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/CosmWasm/wasmvm/v2/types"
)

// TestAddressValidationScenarios covers multiple address lengths and behaviors.
// In the original code, we only tested a single "too long" case. Here we use
// a table-driven approach to validate multiple scenarios.
//
// We also demonstrate how to provide more debugging information with t.Logf
// in the event of test failures or for general clarity.
func TestAddressValidationScenarios(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()

	// Load the contract
	wasmPath := "../../testdata/hackatom.wasm"
	wasm, err := os.ReadFile(wasmPath)
	require.NoError(t, err, "Could not read wasm file at %s", wasmPath)

	// Store the code in the cache
	checksum, err := StoreCode(cache, wasm, true)
	require.NoError(t, err, "Storing code failed for %s", wasmPath)

	// Now define multiple test scenarios
	tests := []struct {
		name          string
		address       string
		expectFailure bool
		expErrMsg     string
	}{
		{
			name:          "Short Address - 6 chars",
			address:       "bob123",
			expectFailure: false,
			expErrMsg:     "",
		},
		{
			name:          "Exactly 32 chars",
			address:       "anhd40ch4h7jdh6j3mpcs7hrrvyv83",
			expectFailure: false,
			expErrMsg:     "",
		},
		{
			name:          "Exact Copy of Valid Address",
			address:       "akash1768hvkh7anhd40ch4h7jdh6j3mpcs7hrrvyv83",
			expectFailure: false,
			expErrMsg:     "",
		},
		{
			name:          "Too Long Address (beyond 32)",
			address:       "long123456789012345678901234567890long",
			expectFailure: true,
			expErrMsg:     "Generic error: addr_validate errored: human encoding too long",
		},
		{
			name:          "Empty Address",
			address:       "",
			expectFailure: true,
			expErrMsg:     "Generic error: addr_validate errored: Input is empty",
		},
		{
			name:          "Unicode / Special Characters",
			address:       "sömëSTRängeădd®ess!",
			expectFailure: true,
			// Adjust expectation if your environment does allow unicode addresses.
			expErrMsg: "Generic error: addr_validate errored:",
		},
	}

	for _, tc := range tests {
		tc := tc // capture loop variable
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("[DEBUG] Running scenario: %s, address='%s'", tc.name, tc.address)

			// Prepare the environment for instantiation
			gasMeter := NewMockGasMeter(TESTING_GAS_LIMIT)
			store := NewLookup(gasMeter)
			api := NewMockAPI()
			querier := DefaultQuerier(MOCK_CONTRACT_ADDR, types.Array[types.Coin]{types.NewCoin(100, "ATOM")})
			env := MockEnvBin(t)
			info := MockInfoBin(t, "creator")

			// Construct the JSON message that sets "verifier" to our test address
			msgStr := fmt.Sprintf(`{"verifier": "%s", "beneficiary": "some_beneficiary"}`, tc.address)
			msg := []byte(msgStr)

			var igasMeter types.GasMeter = gasMeter
			res, cost, err := Instantiate(
				cache,
				checksum,
				env,
				info,
				msg,
				&igasMeter,
				store,
				api,
				&querier,
				TESTING_GAS_LIMIT,
				TESTING_PRINT_DEBUG,
			)

			// Log the gas cost for debugging
			t.Logf("[DEBUG] Gas Used: %d, Gas Remaining: %d", cost.UsedInternally, cost.Remaining)

			// We expect no low-level (Go) error even if the contract validation fails
			require.NoError(t, err,
				"[GO-level error] Instantiation must not return a fatal error for scenario: %s", tc.name)

			// Now decode the contract's result
			var contractResult types.ContractResult
			err = json.Unmarshal(res, &contractResult)
			require.NoError(t, err,
				"JSON unmarshal failed on contract result for scenario: %s\nRaw contract response: %s",
				tc.name, string(res),
			)

			// If we expect a failure, check that contractResult.Err is set
			if tc.expectFailure {
				require.Nil(t, contractResult.Ok,
					"Expected no Ok response, but got: %+v for scenario: %s", contractResult.Ok, tc.name)
				require.Contains(t, contractResult.Err, tc.expErrMsg,
					"Expected error message containing '%s', but got '%s' for scenario: %s",
					tc.expErrMsg, contractResult.Err, tc.name)
				t.Logf("[OK] We got the expected error. Full error: %s", contractResult.Err)
			} else {
				// We do not expect a failure
				require.Equal(t, "", contractResult.Err,
					"Expected no error for scenario: %s, but got: %s", tc.name, contractResult.Err)
				require.NotNil(t, contractResult.Ok,
					"Expected a valid Ok response for scenario: %s, got nil", tc.name)
				t.Logf("[OK] Instantiation succeeded, contract returned: %+v", contractResult.Ok)
			}
		})
	}
}

// TestInstantiateWithVariousMsgFormats tries different JSON payloads, both valid and invalid.
// This shows how to handle scenarios where the contract message might be malformed or incorrectly typed.
func TestInstantiateWithVariousMsgFormats(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()

	// Load the contract
	wasmPath := "../../testdata/hackatom.wasm"
	wasm, err := os.ReadFile(wasmPath)
	require.NoError(t, err, "Could not read wasm file at %s", wasmPath)

	// Store the code in the cache
	checksum, err := StoreCode(cache, wasm, true)
	require.NoError(t, err, "Storing code failed for %s", wasmPath)

	tests := []struct {
		name          string
		jsonMsg       string
		expectFailure bool
		expErrMsg     string
	}{
		{
			name:          "Valid JSON - simple",
			jsonMsg:       `{"verifier":"myverifier","beneficiary":"bob"}`,
			expectFailure: false,
			expErrMsg:     "",
		},
		{
			name:          "Invalid JSON - missing closing brace",
			jsonMsg:       `{"verifier":"bob"`,
			expectFailure: true,
			expErrMsg:     "Error parsing into type hackatom::msg::InstantiateMsg",
		},
		{
			name:          "big extra field",
			jsonMsg:       buildTestJSON(30, 5), // adjust repeats as needed
			expectFailure: true,
			expErrMsg:     "Error parsing into type hackatom::msg::InstantiateMsg: missing field `beneficiary`",
		},
		{
			name:          "giant extra field",
			jsonMsg:       buildTestJSON(300, 50), // even bigger
			expectFailure: true,
			expErrMsg:     "Error parsing into type hackatom::msg::InstantiateMsg: missing field `beneficiary`",
		},
		{
			name:          "Empty JSON message",
			jsonMsg:       `{}`,
			expectFailure: true,
			expErrMsg:     "Error parsing into type hackatom::msg::InstantiateMsg: missing field `verifier`",
		},
		{
			name: "Weird fields",
			jsonMsg: `{
				"verifier": "someone",
				"beneficiary": "bob",
				"thisFieldDoesNotExistInSchema": 1234
			}`,
			expectFailure: true,
			expErrMsg:     "Error parsing into type hackatom::msg::InstantiateMsg: missing field `beneficiary`",
		},
		{
			name:          "Random text not valid JSON",
			jsonMsg:       `Garbage data here`,
			expectFailure: true,
			expErrMsg:     "Invalid type",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("[DEBUG] Checking message scenario: %s, JSON: %s", tc.name, tc.jsonMsg)

			gasMeter := NewMockGasMeter(TESTING_GAS_LIMIT)
			store := NewLookup(gasMeter)
			api := NewMockAPI()
			querier := DefaultQuerier(MOCK_CONTRACT_ADDR, nil)
			env := MockEnvBin(t)
			info := MockInfoBin(t, "creator")

			msg := []byte(tc.jsonMsg)

			var igasMeter types.GasMeter = gasMeter
			res, cost, err := Instantiate(
				cache,
				checksum,
				env,
				info,
				msg,
				&igasMeter,
				store,
				api,
				&querier,
				TESTING_GAS_LIMIT,
				TESTING_PRINT_DEBUG,
			)

			t.Logf("[DEBUG] Gas Used: %d, Gas Remaining: %d", cost.UsedInternally, cost.Remaining)

			// The contract might error at the CosmWasm level. Usually that won't produce a Go-level error,
			// unless the JSON was so malformed that we can't even pass it in to the contract. So we only
			// require that it didn't produce a fatal error. We'll check contract error vs. Ok below.
			require.NoError(t, err,
				"[GO-level error] Instantiation must not return a fatal error for scenario: %s", tc.name)

			// Now decode the contract's result
			var contractResult types.ContractResult
			err = json.Unmarshal(res, &contractResult)
			require.NoError(t, err,
				"JSON unmarshal of contract result must succeed (scenario: %s)\nRaw contract response: %s",
				tc.name, string(res),
			)

			if tc.expectFailure {
				require.Nil(t, contractResult.Ok,
					"Expected no Ok response, but got: %+v for scenario: %s", contractResult.Ok, tc.name)
				// The exact error message from the contract can vary, but we try to match a known phrase
				// from expErrMsg. Adjust or refine as your environment differs.
				require.Contains(t, contractResult.Err, tc.expErrMsg,
					"Expected error containing '%s', but got '%s' for scenario: %s",
					tc.expErrMsg, contractResult.Err, tc.name)
				t.Logf("[OK] We got the expected contract-level error. Full error: %s", contractResult.Err)
			} else {
				require.Equal(t, "", contractResult.Err,
					"Expected no error for scenario: %s, but got: %s", tc.name, contractResult.Err)
				require.NotNil(t, contractResult.Ok,
					"Expected a valid Ok response for scenario: %s, got nil", tc.name)
				t.Logf("[OK] Instantiation succeeded. Ok: %+v", contractResult.Ok)
			}
		})
	}
}

func buildTestJSON(fieldRepeat, valueRepeat int) string {
	// We'll build up the field name by repeating "thisFieldDoesNotExistInSchema" a bunch of times.
	fieldName := "thisFieldDoesNotExistInSchema" + strings.Repeat("thisFieldDoesNotExistInSchema", fieldRepeat)

	// We'll build up the value by repeating the "THIS IS ENORMOUS..." string a bunch of times.
	fieldValue := "THIS IS ENORMOUS ADDITIONAL CONTENT WE ARE PUTTING INTO THE VM LIKE WHOA"
	fieldValue = fieldValue + strings.Repeat("THIS IS ENORMOUS ADDITIONAL CONTENT WE ARE PUTTING INTO THE VM LIKE WHOA", valueRepeat)

	return fmt.Sprintf(`{
		"verifier": "someone",
		"beneficiary": "bob",
		"%s": "%s"
	}`, fieldName, fieldValue)
}

func TestExtraFieldParsing(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()

	// Load the contract
	wasmPath := "../../testdata/hackatom.wasm"
	wasm, err := os.ReadFile(wasmPath)
	require.NoError(t, err, "Could not read wasm file at %s", wasmPath)

	// Store the code in the cache
	checksum, err := StoreCode(cache, wasm, true)
	require.NoError(t, err, "Storing code failed for %s", wasmPath)

	// We'll create a few test scenarios that each produce extra-large JSON messages
	// so we're sending multiple megabytes. We'll log how many MB are being sent.
	tests := []struct {
		name        string
		fieldRepeat int
		valueRepeat int
		expErrMsg   string
	}{
		{
			name:        "0.01 MB of extra field data",
			fieldRepeat: 150, // Tweak until you reach ~1MB total payload
			valueRepeat: 25,
			expErrMsg:   "Error parsing into type hackatom::msg::InstantiateMsg",
		},
		{
			name:        "0.1 MB of extra field data",
			fieldRepeat: 15000, // Tweak until you reach ~1MB total payload
			valueRepeat: 7000,
			expErrMsg:   "Error parsing into type hackatom::msg::InstantiateMsg",
		},
		{
			name:        "~2MB of extra field data",
			fieldRepeat: 1500,
			valueRepeat: 250,
			expErrMsg:   "Error parsing into type hackatom::msg::InstantiateMsg",
		},
		{
			name:        ">10MB  of extra field data",
			fieldRepeat: 100000,
			valueRepeat: 100000,
			expErrMsg:   "Error parsing into type hackatom::msg::InstantiateMsg",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// Build JSON with a huge extra field
			jsonMsg := buildTestJSON(tc.fieldRepeat, tc.valueRepeat)

			// Log how large the JSON message is (in MB)
			sizeMB := float64(len(jsonMsg)) / (1024.0 * 1024.0)
			t.Logf("[DEBUG] Using JSON of size: %.2f MB", sizeMB)

			gasMeter := NewMockGasMeter(TESTING_GAS_LIMIT)
			store := NewLookup(gasMeter)
			api := NewMockAPI()
			querier := DefaultQuerier(MOCK_CONTRACT_ADDR, nil)
			env := MockEnvBin(t)
			info := MockInfoBin(t, "creator")

			msg := []byte(jsonMsg)

			var igasMeter types.GasMeter = gasMeter
			res, cost, err := Instantiate(
				cache,
				checksum,
				env,
				info,
				msg,
				&igasMeter,
				store,
				api,
				&querier,
				TESTING_GAS_LIMIT,
				TESTING_PRINT_DEBUG,
			)

			t.Logf("[DEBUG] Gas Used: %d, Gas Remaining: %d", cost.UsedInternally, cost.Remaining)

			// Ensure there's no Go-level fatal error
			require.NoError(t, err,
				"[GO-level error] Instantiation must not return a fatal error for scenario: %s", tc.name)

			// Decode the contract result (CosmWasm-level error will appear in contractResult.Err if any)
			var contractResult types.ContractResult
			err = json.Unmarshal(res, &contractResult)
			require.NoError(t, err,
				"JSON unmarshal of contract result must succeed (scenario: %s)\nRaw contract response: %s",
				tc.name, string(res),
			)

			// We expect the contract to reject such large messages. Adjust if your contract differs.
			require.Nil(t, contractResult.Ok,
				"Expected no Ok response for scenario: %s, but got: %+v", tc.name, contractResult.Ok)
			require.Contains(t, contractResult.Err, tc.expErrMsg,
				"Expected error containing '%s', but got '%s' for scenario: %s",
				tc.expErrMsg, contractResult.Err, tc.name)

			t.Logf("[OK] We got the expected contract-level error. Full error: %s", contractResult.Err)
		})
	}
}

```
---
### `callbacks.go`
*2025-02-15 10:17:28 | 18 KB*
```go
package api

// Check https://akrennmair.github.io/golang-cgo-slides/ to learn
// how this embedded C code works.

/*
#include "bindings.h"

// All C function types in struct fields will be represented as a *[0]byte in Go and
// we don't get any type safety on the signature. To express this fact in type conversions,
// we create a single function pointer type here.
// The only thing this is used for is casting between unsafe.Pointer and *[0]byte in Go.
// See also https://github.com/golang/go/issues/19835
typedef void (*any_function_t)();

// forward declarations (db)
GoError cGet_cgo(db_t *ptr, gas_meter_t *gas_meter, uint64_t *used_gas, U8SliceView key, UnmanagedVector *val, UnmanagedVector *errOut);
GoError cSet_cgo(db_t *ptr, gas_meter_t *gas_meter, uint64_t *used_gas, U8SliceView key, U8SliceView val, UnmanagedVector *errOut);
GoError cDelete_cgo(db_t *ptr, gas_meter_t *gas_meter, uint64_t *used_gas, U8SliceView key, UnmanagedVector *errOut);
GoError cScan_cgo(db_t *ptr, gas_meter_t *gas_meter, uint64_t *used_gas, U8SliceView start, U8SliceView end, int32_t order, GoIter *out, UnmanagedVector *errOut);
// iterator
GoError cNext_cgo(IteratorReference *ref, gas_meter_t *gas_meter, uint64_t *used_gas, UnmanagedVector *key, UnmanagedVector *val, UnmanagedVector *errOut);
GoError cNextKey_cgo(IteratorReference *ref, gas_meter_t *gas_meter, uint64_t *used_gas, UnmanagedVector *key, UnmanagedVector *errOut);
GoError cNextValue_cgo(IteratorReference *ref, gas_meter_t *gas_meter, uint64_t *used_gas, UnmanagedVector *val, UnmanagedVector *errOut);
// api
GoError cHumanizeAddress_cgo(api_t *ptr, U8SliceView src, UnmanagedVector *dest, UnmanagedVector *errOut, uint64_t *used_gas);
GoError cCanonicalizeAddress_cgo(api_t *ptr, U8SliceView src, UnmanagedVector *dest, UnmanagedVector *errOut, uint64_t *used_gas);
GoError cValidateAddress_cgo(api_t *ptr, U8SliceView src, UnmanagedVector *errOut, uint64_t *used_gas);
// and querier
GoError cQueryExternal_cgo(querier_t *ptr, uint64_t gas_limit, uint64_t *used_gas, U8SliceView request, UnmanagedVector *result, UnmanagedVector *errOut);


*/
import "C"

import (
	"encoding/json"
	"fmt"
	"log"
	"reflect"
	"runtime/debug"
	"unsafe"

	"github.com/CosmWasm/wasmvm/v2/types"
)

// Note: we have to include all exports in the same file (at least since they both import bindings.h),
// or get odd cgo build errors about duplicate definitions

func recoverPanic(ret *C.GoError) {
	if rec := recover(); rec != nil {
		// This is used to handle ErrorOutOfGas panics.
		//
		// What we do here is something that should not be done in the first place.
		// "A panic typically means something went unexpectedly wrong. Mostly we use it to fail fast
		// on errors that shouldn’t occur during normal operation, or that we aren’t prepared to
		// handle gracefully." says https://gobyexample.com/panic.
		// And 'Ask yourself "when this happens, should the application immediately crash?" If yes,
		// use a panic; otherwise, use an error.' says this popular answer on SO: https://stackoverflow.com/a/44505268.
		// Oh, and "If you're already worrying about discriminating different kinds of panics, you've lost sight of the ball."
		// (Rob Pike) from https://eli.thegreenplace.net/2018/on-the-uses-and-misuses-of-panics-in-go/
		//
		// We don't want to import Cosmos SDK and also cannot use interfaces to detect these
		// error types (as they have no methods). So, let's just rely on the descriptive names.
		name := reflect.TypeOf(rec).Name()
		switch name {
		// These three types are "thrown" (which is not a thing in Go 🙃) in panics from the gas module
		// (https://github.com/cosmos/cosmos-sdk/blob/v0.45.4/store/types/gas.go):
		// 1. ErrorOutOfGas
		// 2. ErrorGasOverflow
		// 3. ErrorNegativeGasConsumed
		//
		// In the baseapp, ErrorOutOfGas gets special treatment:
		// - https://github.com/cosmos/cosmos-sdk/blob/v0.45.4/baseapp/baseapp.go#L607
		// - https://github.com/cosmos/cosmos-sdk/blob/v0.45.4/baseapp/recovery.go#L50-L60
		// This turns the panic into a regular error with a helpful error message.
		//
		// The other two gas related panic types indicate programming errors and are handled along
		// with all other errors in https://github.com/cosmos/cosmos-sdk/blob/v0.45.4/baseapp/recovery.go#L66-L77.
		case "ErrorOutOfGas":
			// TODO: figure out how to pass the text in its `Descriptor` field through all the FFI
			*ret = C.GoError_OutOfGas
		default:
			log.Printf("Panic in Go callback: %#v\n", rec)
			debug.PrintStack()
			*ret = C.GoError_Panic
		}
	}
}

/****** DB ********/

var db_vtable = C.DbVtable{
	read_db:   C.any_function_t(C.cGet_cgo),
	write_db:  C.any_function_t(C.cSet_cgo),
	remove_db: C.any_function_t(C.cDelete_cgo),
	scan_db:   C.any_function_t(C.cScan_cgo),
}

type DBState struct {
	Store types.KVStore
	// CallID is used to lookup the proper frame for iterators associated with this contract call (iterator.go)
	CallID uint64
}

// use this to create C.Db in two steps, so the pointer lives as long as the calling stack
//
//	state := buildDBState(kv, callID)
//	db := buildDB(&state, &gasMeter)
//	// then pass db into some FFI function
func buildDBState(kv types.KVStore, callID uint64) DBState {
	return DBState{
		Store:  kv,
		CallID: callID,
	}
}

// contract: original pointer/struct referenced must live longer than C.Db struct
// since this is only used internally, we can verify the code that this is the case
func buildDB(state *DBState, gm *types.GasMeter) C.Db {
	return C.Db{
		gas_meter: (*C.gas_meter_t)(unsafe.Pointer(gm)),
		state:     (*C.db_t)(unsafe.Pointer(state)),
		vtable:    db_vtable,
	}
}

var iterator_vtable = C.IteratorVtable{
	next:       C.any_function_t(C.cNext_cgo),
	next_key:   C.any_function_t(C.cNextKey_cgo),
	next_value: C.any_function_t(C.cNextValue_cgo),
}

// An iterator including referenced objects is 117 bytes large (calculated using https://github.com/DmitriyVTitov/size).
// We limit the number of iterators per contract call ID here in order limit memory usage to 32768*117 = ~3.8 MB as a safety measure.
// In any reasonable contract, gas limits should hit sooner than that though.
const frameLenLimit = 32768

// contract: original pointer/struct referenced must live longer than C.Db struct
// since this is only used internally, we can verify the code that this is the case
func buildIterator(callID uint64, it types.Iterator) (C.IteratorReference, error) {
	iteratorID, err := storeIterator(callID, it, frameLenLimit)
	if err != nil {
		return C.IteratorReference{}, err
	}
	return C.IteratorReference{
		call_id:     cu64(callID),
		iterator_id: cu64(iteratorID),
	}, nil
}

//export cGet
func cGet(ptr *C.db_t, gasMeter *C.gas_meter_t, usedGas *cu64, key C.U8SliceView, val *C.UnmanagedVector, errOut *C.UnmanagedVector) (ret C.GoError) {
	defer recoverPanic(&ret)

	if ptr == nil || gasMeter == nil || usedGas == nil || val == nil || errOut == nil {
		// we received an invalid pointer
		return C.GoError_BadArgument
	}
	// errOut is unused and we don't check `is_none` because of https://github.com/CosmWasm/wasmvm/issues/536
	if !(*val).is_none {
		panic("Got a non-none UnmanagedVector we're about to override. This is a bug because someone has to drop the old one.")
	}

	gm := *(*types.GasMeter)(unsafe.Pointer(gasMeter))
	kv := *(*types.KVStore)(unsafe.Pointer(ptr))
	k := copyU8Slice(key)

	gasBefore := gm.GasConsumed()
	v := kv.Get(k)
	gasAfter := gm.GasConsumed()
	*usedGas = (cu64)(gasAfter - gasBefore)

	// v will equal nil when the key is missing
	// https://github.com/cosmos/cosmos-sdk/blob/1083fa948e347135861f88e07ec76b0314296832/store/types/store.go#L174
	*val = newUnmanagedVector(v)

	return C.GoError_None
}

//export cSet
func cSet(ptr *C.db_t, gasMeter *C.gas_meter_t, usedGas *cu64, key C.U8SliceView, val C.U8SliceView, errOut *C.UnmanagedVector) (ret C.GoError) {
	defer recoverPanic(&ret)

	if ptr == nil || gasMeter == nil || usedGas == nil || errOut == nil {
		// we received an invalid pointer
		return C.GoError_BadArgument
	}
	// errOut is unused and we don't check `is_none` because of https://github.com/CosmWasm/wasmvm/issues/536

	gm := *(*types.GasMeter)(unsafe.Pointer(gasMeter))
	kv := *(*types.KVStore)(unsafe.Pointer(ptr))
	k := copyU8Slice(key)
	v := copyU8Slice(val)

	gasBefore := gm.GasConsumed()
	kv.Set(k, v)
	gasAfter := gm.GasConsumed()
	*usedGas = (cu64)(gasAfter - gasBefore)

	return C.GoError_None
}

//export cDelete
func cDelete(ptr *C.db_t, gasMeter *C.gas_meter_t, usedGas *cu64, key C.U8SliceView, errOut *C.UnmanagedVector) (ret C.GoError) {
	defer recoverPanic(&ret)

	if ptr == nil || gasMeter == nil || usedGas == nil || errOut == nil {
		// we received an invalid pointer
		return C.GoError_BadArgument
	}
	// errOut is unused and we don't check `is_none` because of https://github.com/CosmWasm/wasmvm/issues/536

	gm := *(*types.GasMeter)(unsafe.Pointer(gasMeter))
	kv := *(*types.KVStore)(unsafe.Pointer(ptr))
	k := copyU8Slice(key)

	gasBefore := gm.GasConsumed()
	kv.Delete(k)
	gasAfter := gm.GasConsumed()
	*usedGas = (cu64)(gasAfter - gasBefore)

	return C.GoError_None
}

//export cScan
func cScan(ptr *C.db_t, gasMeter *C.gas_meter_t, usedGas *cu64, start C.U8SliceView, end C.U8SliceView, order ci32, out *C.GoIter, errOut *C.UnmanagedVector) (ret C.GoError) {
	defer recoverPanic(&ret)

	if ptr == nil || gasMeter == nil || usedGas == nil || out == nil || errOut == nil {
		// we received an invalid pointer
		return C.GoError_BadArgument
	}
	if !(*errOut).is_none {
		panic("Got a non-none UnmanagedVector we're about to override. This is a bug because someone has to drop the old one.")
	}

	gm := *(*types.GasMeter)(unsafe.Pointer(gasMeter))
	state := (*DBState)(unsafe.Pointer(ptr))
	kv := state.Store
	s := copyU8Slice(start)
	e := copyU8Slice(end)

	var iter types.Iterator
	gasBefore := gm.GasConsumed()
	switch order {
	case 1: // Ascending
		iter = kv.Iterator(s, e)
	case 2: // Descending
		iter = kv.ReverseIterator(s, e)
	default:
		return C.GoError_BadArgument
	}
	gasAfter := gm.GasConsumed()
	*usedGas = (cu64)(gasAfter - gasBefore)

	iteratorRef, err := buildIterator(state.CallID, iter)
	if err != nil {
		// store the actual error message in the return buffer
		*errOut = newUnmanagedVector([]byte(err.Error()))
		return C.GoError_User
	}

	*out = C.GoIter{
		gas_meter: gasMeter,
		reference: iteratorRef,
		vtable:    iterator_vtable,
	}

	return C.GoError_None
}

//export cNext
func cNext(ref C.IteratorReference, gasMeter *C.gas_meter_t, usedGas *cu64, key *C.UnmanagedVector, val *C.UnmanagedVector, errOut *C.UnmanagedVector) (ret C.GoError) {
	// typical usage of iterator
	// 	for ; itr.Valid(); itr.Next() {
	// 		k, v := itr.Key(); itr.Value()
	// 		...
	// 	}

	defer recoverPanic(&ret)
	if ref.call_id == 0 || gasMeter == nil || usedGas == nil || key == nil || val == nil || errOut == nil {
		// we received an invalid pointer
		return C.GoError_BadArgument
	}
	// errOut is unused and we don't check `is_none` because of https://github.com/CosmWasm/wasmvm/issues/536
	if !(*key).is_none || !(*val).is_none {
		panic("Got a non-none UnmanagedVector we're about to override. This is a bug because someone has to drop the old one.")
	}

	gm := *(*types.GasMeter)(unsafe.Pointer(gasMeter))
	iter := retrieveIterator(uint64(ref.call_id), uint64(ref.iterator_id))
	if iter == nil {
		panic("Unable to retrieve iterator.")
	}
	if !iter.Valid() {
		// end of iterator, return as no-op, nil key is considered end
		return C.GoError_None
	}

	gasBefore := gm.GasConsumed()
	// call Next at the end, upon creation we have first data loaded
	k := iter.Key()
	v := iter.Value()
	// check iter.Error() ????
	iter.Next()
	gasAfter := gm.GasConsumed()
	*usedGas = (cu64)(gasAfter - gasBefore)

	*key = newUnmanagedVector(k)
	*val = newUnmanagedVector(v)
	return C.GoError_None
}

//export cNextKey
func cNextKey(ref C.IteratorReference, gasMeter *C.gas_meter_t, usedGas *cu64, key *C.UnmanagedVector, errOut *C.UnmanagedVector) (ret C.GoError) {
	return nextPart(ref, gasMeter, usedGas, key, errOut, func(iter types.Iterator) []byte { return iter.Key() })
}

//export cNextValue
func cNextValue(ref C.IteratorReference, gasMeter *C.gas_meter_t, usedGas *cu64, value *C.UnmanagedVector, errOut *C.UnmanagedVector) (ret C.GoError) {
	return nextPart(ref, gasMeter, usedGas, value, errOut, func(iter types.Iterator) []byte { return iter.Value() })
}

// nextPart is a helper function that contains the shared code for key- and value-only iteration.
func nextPart(ref C.IteratorReference, gasMeter *C.gas_meter_t, usedGas *cu64, output *C.UnmanagedVector, errOut *C.UnmanagedVector, valFn func(types.Iterator) []byte) (ret C.GoError) {
	// typical usage of iterator
	// 	for ; itr.Valid(); itr.Next() {
	// 		k, v := itr.Key(); itr.Value()
	// 		...
	// 	}

	defer recoverPanic(&ret)
	if ref.call_id == 0 || gasMeter == nil || usedGas == nil || output == nil || errOut == nil {
		// we received an invalid pointer
		return C.GoError_BadArgument
	}
	// errOut is unused and we don't check `is_none` because of https://github.com/CosmWasm/wasmvm/issues/536
	if !(*output).is_none {
		panic("Got a non-none UnmanagedVector we're about to override. This is a bug because someone has to drop the old one.")
	}

	gm := *(*types.GasMeter)(unsafe.Pointer(gasMeter))
	iter := retrieveIterator(uint64(ref.call_id), uint64(ref.iterator_id))
	if iter == nil {
		panic("Unable to retrieve iterator.")
	}
	if !iter.Valid() {
		// end of iterator, return as no-op, nil `output` is considered end
		return C.GoError_None
	}

	gasBefore := gm.GasConsumed()
	// call Next at the end, upon creation we have first data loaded
	out := valFn(iter)
	// check iter.Error() ????
	iter.Next()
	gasAfter := gm.GasConsumed()
	*usedGas = (cu64)(gasAfter - gasBefore)

	*output = newUnmanagedVector(out)
	return C.GoError_None
}

var api_vtable = C.GoApiVtable{
	humanize_address:     C.any_function_t(C.cHumanizeAddress_cgo),
	canonicalize_address: C.any_function_t(C.cCanonicalizeAddress_cgo),
	validate_address:     C.any_function_t(C.cValidateAddress_cgo),
}

// contract: original pointer/struct referenced must live longer than C.GoApi struct
// since this is only used internally, we can verify the code that this is the case
func buildAPI(api *types.GoAPI) C.GoApi {
	return C.GoApi{
		state:  (*C.api_t)(unsafe.Pointer(api)),
		vtable: api_vtable,
	}
}

//export cHumanizeAddress
func cHumanizeAddress(ptr *C.api_t, src C.U8SliceView, dest *C.UnmanagedVector, errOut *C.UnmanagedVector, used_gas *cu64) (ret C.GoError) {
	defer recoverPanic(&ret)

	if dest == nil || errOut == nil {
		return C.GoError_BadArgument
	}
	if !(*dest).is_none || !(*errOut).is_none {
		panic("Got a non-none UnmanagedVector we're about to override. This is a bug because someone has to drop the old one.")
	}

	api := (*types.GoAPI)(unsafe.Pointer(ptr))
	s := copyU8Slice(src)

	h, cost, err := api.HumanizeAddress(s)
	*used_gas = cu64(cost)
	if err != nil {
		// store the actual error message in the return buffer
		*errOut = newUnmanagedVector([]byte(err.Error()))
		return C.GoError_User
	}
	if len(h) == 0 {
		panic(fmt.Sprintf("`api.HumanizeAddress()` returned an empty string for %q", s))
	}
	*dest = newUnmanagedVector([]byte(h))
	return C.GoError_None
}

//export cCanonicalizeAddress
func cCanonicalizeAddress(ptr *C.api_t, src C.U8SliceView, dest *C.UnmanagedVector, errOut *C.UnmanagedVector, used_gas *cu64) (ret C.GoError) {
	defer recoverPanic(&ret)

	if dest == nil || errOut == nil {
		return C.GoError_BadArgument
	}
	if !(*dest).is_none || !(*errOut).is_none {
		panic("Got a non-none UnmanagedVector we're about to override. This is a bug because someone has to drop the old one.")
	}

	api := (*types.GoAPI)(unsafe.Pointer(ptr))
	s := string(copyU8Slice(src))
	c, cost, err := api.CanonicalizeAddress(s)
	*used_gas = cu64(cost)
	if err != nil {
		// store the actual error message in the return buffer
		*errOut = newUnmanagedVector([]byte(err.Error()))
		return C.GoError_User
	}
	if len(c) == 0 {
		panic(fmt.Sprintf("`api.CanonicalizeAddress()` returned an empty string for %q", s))
	}
	*dest = newUnmanagedVector(c)
	return C.GoError_None
}

//export cValidateAddress
func cValidateAddress(ptr *C.api_t, src C.U8SliceView, errOut *C.UnmanagedVector, used_gas *cu64) (ret C.GoError) {
	defer recoverPanic(&ret)

	if errOut == nil {
		return C.GoError_BadArgument
	}
	if !(*errOut).is_none {
		panic("Got a non-none UnmanagedVector we're about to override. This is a bug because someone has to drop the old one.")
	}

	api := (*types.GoAPI)(unsafe.Pointer(ptr))
	s := string(copyU8Slice(src))
	cost, err := api.ValidateAddress(s)

	*used_gas = cu64(cost)
	if err != nil {
		// store the actual error message in the return buffer
		*errOut = newUnmanagedVector([]byte(err.Error()))
		return C.GoError_User
	}
	return C.GoError_None
}

/****** Go Querier ********/

var querier_vtable = C.QuerierVtable{
	query_external: C.any_function_t(C.cQueryExternal_cgo),
}

// contract: original pointer/struct referenced must live longer than C.GoQuerier struct
// since this is only used internally, we can verify the code that this is the case
func buildQuerier(q *Querier) C.GoQuerier {
	return C.GoQuerier{
		state:  (*C.querier_t)(unsafe.Pointer(q)),
		vtable: querier_vtable,
	}
}

//export cQueryExternal
func cQueryExternal(ptr *C.querier_t, gasLimit cu64, usedGas *cu64, request C.U8SliceView, result *C.UnmanagedVector, errOut *C.UnmanagedVector) (ret C.GoError) {
	defer recoverPanic(&ret)

	if ptr == nil || usedGas == nil || result == nil || errOut == nil {
		// we received an invalid pointer
		return C.GoError_BadArgument
	}
	if !(*result).is_none || !(*errOut).is_none {
		panic("Got a non-none UnmanagedVector we're about to override. This is a bug because someone has to drop the old one.")
	}

	// query the data
	querier := *(*Querier)(unsafe.Pointer(ptr))
	req := copyU8Slice(request)

	gasBefore := querier.GasConsumed()
	res := types.RustQuery(querier, req, uint64(gasLimit))
	gasAfter := querier.GasConsumed()
	*usedGas = (cu64)(gasAfter - gasBefore)

	// serialize the response
	bz, err := json.Marshal(res)
	if err != nil {
		*errOut = newUnmanagedVector([]byte(err.Error()))
		return C.GoError_CannotSerialize
	}
	*result = newUnmanagedVector(bz)
	return C.GoError_None
}

```
---
### `callbacks_cgo.go`
*2025-02-15 10:17:28 | 5 KB*
```go
package api

/*
#include "bindings.h"
#include <stdio.h>

// imports (db)
GoError cSet(db_t *ptr, gas_meter_t *gas_meter, uint64_t *used_gas, U8SliceView key, U8SliceView val, UnmanagedVector *errOut);
GoError cGet(db_t *ptr, gas_meter_t *gas_meter, uint64_t *used_gas, U8SliceView key, UnmanagedVector *val, UnmanagedVector *errOut);
GoError cDelete(db_t *ptr, gas_meter_t *gas_meter, uint64_t *used_gas, U8SliceView key, UnmanagedVector *errOut);
GoError cScan(db_t *ptr, gas_meter_t *gas_meter, uint64_t *used_gas, U8SliceView start, U8SliceView end, int32_t order, GoIter *out, UnmanagedVector *errOut);
// imports (iterator)
GoError cNext(IteratorReference *ref, gas_meter_t *gas_meter, uint64_t *used_gas, UnmanagedVector *key, UnmanagedVector *val, UnmanagedVector *errOut);
GoError cNextKey(IteratorReference *ref, gas_meter_t *gas_meter, uint64_t *used_gas, UnmanagedVector *key, UnmanagedVector *errOut);
GoError cNextValue(IteratorReference *ref, gas_meter_t *gas_meter, uint64_t *used_gas, UnmanagedVector *value, UnmanagedVector *errOut);
// imports (api)
GoError cHumanizeAddress(api_t *ptr, U8SliceView src, UnmanagedVector *dest, UnmanagedVector *errOut, uint64_t *used_gas);
GoError cCanonicalizeAddress(api_t *ptr, U8SliceView src, UnmanagedVector *dest, UnmanagedVector *errOut, uint64_t *used_gas);
GoError cValidateAddress(api_t *ptr, U8SliceView src, UnmanagedVector *errOut, uint64_t *used_gas);
// imports (querier)
GoError cQueryExternal(querier_t *ptr, uint64_t gas_limit, uint64_t *used_gas, U8SliceView request, UnmanagedVector *result, UnmanagedVector *errOut);

// Gateway functions (db)
GoError cGet_cgo(db_t *ptr, gas_meter_t *gas_meter, uint64_t *used_gas, U8SliceView key, UnmanagedVector *val, UnmanagedVector *errOut) {
	return cGet(ptr, gas_meter, used_gas, key, val, errOut);
}
GoError cSet_cgo(db_t *ptr, gas_meter_t *gas_meter, uint64_t *used_gas, U8SliceView key, U8SliceView val, UnmanagedVector *errOut) {
	return cSet(ptr, gas_meter, used_gas, key, val, errOut);
}
GoError cDelete_cgo(db_t *ptr, gas_meter_t *gas_meter, uint64_t *used_gas, U8SliceView key, UnmanagedVector *errOut) {
	return cDelete(ptr, gas_meter, used_gas, key, errOut);
}
GoError cScan_cgo(db_t *ptr, gas_meter_t *gas_meter, uint64_t *used_gas, U8SliceView start, U8SliceView end, int32_t order, GoIter *out, UnmanagedVector *errOut) {
	return cScan(ptr, gas_meter, used_gas, start, end, order, out, errOut);
}

// Gateway functions (iterator)
GoError cNext_cgo(IteratorReference *ref, gas_meter_t *gas_meter, uint64_t *used_gas, UnmanagedVector *key, UnmanagedVector *val, UnmanagedVector *errOut) {
	return cNext(ref, gas_meter, used_gas, key, val, errOut);
}
GoError cNextKey_cgo(IteratorReference *ref, gas_meter_t *gas_meter, uint64_t *used_gas, UnmanagedVector *key, UnmanagedVector *errOut) {
	return cNextKey(ref, gas_meter, used_gas, key, errOut);
}
GoError cNextValue_cgo(IteratorReference *ref, gas_meter_t *gas_meter, uint64_t *used_gas, UnmanagedVector *val, UnmanagedVector *errOut) {
	return cNextValue(ref, gas_meter, used_gas, val, errOut);
}

// Gateway functions (api)
GoError cCanonicalizeAddress_cgo(api_t *ptr, U8SliceView src, UnmanagedVector *dest, UnmanagedVector *errOut, uint64_t *used_gas) {
    return cCanonicalizeAddress(ptr, src, dest, errOut, used_gas);
}
GoError cHumanizeAddress_cgo(api_t *ptr, U8SliceView src, UnmanagedVector *dest, UnmanagedVector *errOut, uint64_t *used_gas) {
    return cHumanizeAddress(ptr, src, dest, errOut, used_gas);
}
GoError cValidateAddress_cgo(api_t *ptr, U8SliceView src, UnmanagedVector *errOut, uint64_t *used_gas) {
    return cValidateAddress(ptr, src, errOut, used_gas);
}

// Gateway functions (querier)
GoError cQueryExternal_cgo(querier_t *ptr, uint64_t gas_limit, uint64_t *used_gas, U8SliceView request, UnmanagedVector *result, UnmanagedVector *errOut) {
    return cQueryExternal(ptr, gas_limit, used_gas, request, result, errOut);
}
*/
import "C"

// We need these gateway functions to allow calling back to a go function from the c code.
// At least I didn't discover a cleaner way.
// Also, this needs to be in a different file than `callbacks.go`, as we cannot create functions
// in the same file that has //export directives. Only import header types

```
---
### `iterator.go`
*2025-02-15 10:17:28 | 4 KB*
```go
package api

import (
	"fmt"
	"math"
	"sync"

	"github.com/CosmWasm/wasmvm/v2/types"
)

// frame stores all Iterators for one contract call
type frame []types.Iterator

// iteratorFrames contains one frame for each contract call, indexed by contract call ID.
var (
	iteratorFrames      = make(map[uint64]frame)
	iteratorFramesMutex sync.Mutex
)

// this is a global counter for creating call IDs
var (
	latestCallID      uint64
	latestCallIDMutex sync.Mutex
)

// startCall is called at the beginning of a contract call to create a new frame in iteratorFrames.
// It updates latestCallID for generating a new call ID.
func startCall() uint64 {
	latestCallIDMutex.Lock()
	defer latestCallIDMutex.Unlock()
	latestCallID += 1
	return latestCallID
}

// removeFrame removes the frame with for the given call ID.
// The result can be nil when the frame is not initialized,
// i.e. when startCall() is called but no iterator is stored.
func removeFrame(callID uint64) frame {
	iteratorFramesMutex.Lock()
	defer iteratorFramesMutex.Unlock()

	remove := iteratorFrames[callID]
	delete(iteratorFrames, callID)
	return remove
}

// endCall is called at the end of a contract call to remove one item the iteratorFrames
func endCall(callID uint64) {
	// we pull removeFrame in another function so we don't hold the mutex while cleaning up the removed frame
	remove := removeFrame(callID)
	// free all iterators in the frame when we release it
	for _, iter := range remove {
		iter.Close()
	}
}

// storeIterator will add this to the end of the frame for the given call ID and return
// an iterator ID to reference it.
//
// We assign iterator IDs starting with 1 for historic reasons. This could be changed to 0
// I guess.
func storeIterator(callID uint64, it types.Iterator, frameLenLimit int) (uint64, error) {
	iteratorFramesMutex.Lock()
	defer iteratorFramesMutex.Unlock()

	new_index := len(iteratorFrames[callID])
	if new_index >= frameLenLimit {
		return 0, fmt.Errorf("Reached iterator limit (%d)", frameLenLimit)
	}

	// store at array position `new_index`
	iteratorFrames[callID] = append(iteratorFrames[callID], it)

	iterator_id, ok := indexToIteratorID(new_index)
	if !ok {
		// This error case is not expected to happen since the above code ensures the
		// index is in the range [0, frameLenLimit-1]
		return 0, fmt.Errorf("could not convert index to iterator ID")
	}
	return iterator_id, nil
}

// retrieveIterator will recover an iterator based on its ID.
func retrieveIterator(callID uint64, iteratorID uint64) types.Iterator {
	indexInFrame, ok := iteratorIdToIndex(iteratorID)
	if !ok {
		return nil
	}

	iteratorFramesMutex.Lock()
	defer iteratorFramesMutex.Unlock()
	myFrame := iteratorFrames[callID]
	if myFrame == nil {
		return nil
	}
	if indexInFrame >= len(myFrame) {
		// index out of range
		return nil
	}
	return myFrame[indexInFrame]
}

// iteratorIdToIndex converts an iterator ID to an index in the frame.
// The second value marks if the conversion succeeded.
func iteratorIdToIndex(id uint64) (int, bool) {
	if id < 1 || id > math.MaxInt32 {
		// If success is false, the int value is undefined. We use an arbitrary constant for potential debugging purposes.
		return 777777777, false
	}

	// Int conversion safe because value is in signed 32bit integer range
	return int(id) - 1, true
}

// indexToIteratorID converts an index in the frame to an iterator ID.
// The second value marks if the conversion succeeded.
func indexToIteratorID(index int) (uint64, bool) {
	if index < 0 || index > math.MaxInt32 {
		// If success is false, the return value is undefined. We use an arbitrary constant for potential debugging purposes.
		return 888888888, false
	}

	return uint64(index) + 1, true
}

```
---
### `iterator_test.go`
*2025-02-15 10:18:34 | 14 KB*
```go
// queue_iterator_test.go

package api

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/CosmWasm/wasmvm/v2/internal/api/testdb"
	"github.com/CosmWasm/wasmvm/v2/types"
)

// queueData wraps contract info to make test usage easier
type queueData struct {
	checksum []byte
	store    *Lookup
	api      *types.GoAPI
	querier  types.Querier
}

// Store provides a KVStore with an updated gas meter
func (q queueData) Store(meter MockGasMeter) types.KVStore {
	return q.store.WithGasMeter(meter)
}

// setupQueueContractWithData uploads/instantiates a queue contract, optionally enqueuing data
func setupQueueContractWithData(t *testing.T, cache Cache, values ...int) queueData {
	checksum := createQueueContract(t, cache)

	gasMeter1 := NewMockGasMeter(TESTING_GAS_LIMIT)
	store := NewLookup(gasMeter1)
	api := NewMockAPI()
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, types.Array[types.Coin]{types.NewCoin(100, "ATOM")})

	// Initialize with empty msg (`{}`)
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")
	msg := []byte(`{}`)

	igasMeter1 := types.GasMeter(gasMeter1)
	res, _, err := Instantiate(cache, checksum, env, info, msg, &igasMeter1, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err, "Instantiation must succeed")
	requireOkResponse(t, res, 0)

	// Optionally enqueue some integer values
	for _, value := range values {
		var gasMeter2 types.GasMeter = NewMockGasMeter(TESTING_GAS_LIMIT)
		push := []byte(fmt.Sprintf(`{"enqueue":{"value":%d}}`, value))
		res, _, err = Execute(cache, checksum, env, info, push, &gasMeter2, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
		require.NoError(t, err, "Enqueue must succeed for value %d", value)
		requireOkResponse(t, res, 0)
	}

	return queueData{
		checksum: checksum,
		store:    store,
		api:      api,
		querier:  querier,
	}
}

// setupQueueContract is a convenience that uses default enqueued values
func setupQueueContract(t *testing.T, cache Cache) queueData {
	return setupQueueContractWithData(t, cache, 17, 22)
}

//---------------------
// Table-based tests
//---------------------

func TestStoreIterator_TableDriven(t *testing.T) {
	type testCase struct {
		name    string
		actions []func(t *testing.T, store types.KVStore, callID uint64, limit int) (uint64, error)
		expect  []uint64 // expected return values from storeIterator
	}

	store := testdb.NewMemDB()
	const limit = 2000

	// We’ll define 2 callIDs, each storing a few iterators
	callID1 := startCall()
	callID2 := startCall()

	// Action helper: open a new iterator, then call storeIterator
	createIter := func(t *testing.T, store types.KVStore) types.Iterator {
		iter := store.Iterator(nil, nil)
		require.NotNil(t, iter, "iter creation must not fail")
		return iter
	}

	// We define test steps where each function returns a (uint64, error).
	// Then we compare with the expected result (uint64) if error is nil.
	tests := []testCase{
		{
			name: "CallID1: two iterators in sequence",
			actions: []func(t *testing.T, store types.KVStore, callID uint64, limit int) (uint64, error){
				func(t *testing.T, store types.KVStore, callID uint64, limit int) (uint64, error) {
					iter := createIter(t, store)
					return storeIterator(callID, iter, limit)
				},
				func(t *testing.T, store types.KVStore, callID uint64, limit int) (uint64, error) {
					iter := createIter(t, store)
					return storeIterator(callID, iter, limit)
				},
			},
			expect: []uint64{1, 2}, // first call ->1, second call ->2
		},
		{
			name: "CallID2: three iterators in sequence",
			actions: []func(t *testing.T, store types.KVStore, callID uint64, limit int) (uint64, error){
				func(t *testing.T, store types.KVStore, callID uint64, limit int) (uint64, error) {
					iter := createIter(t, store)
					return storeIterator(callID, iter, limit)
				},
				func(t *testing.T, store types.KVStore, callID uint64, limit int) (uint64, error) {
					iter := createIter(t, store)
					return storeIterator(callID, iter, limit)
				},
				func(t *testing.T, store types.KVStore, callID uint64, limit int) (uint64, error) {
					iter := createIter(t, store)
					return storeIterator(callID, iter, limit)
				},
			},
			expect: []uint64{1, 2, 3},
		},
	}

	for _, tc := range tests {
		tc := tc // capture range variable
		t.Run(tc.name, func(t *testing.T) {
			var results []uint64
			// Decide which callID to use by name
			// We'll do a simple check:
			var activeCallID uint64
			if tc.name == "CallID1: two iterators in sequence" {
				activeCallID = callID1
			} else {
				activeCallID = callID2
			}

			for i, step := range tc.actions {
				got, err := step(t, store, activeCallID, limit)
				require.NoError(t, err, "storeIterator must not fail in step[%d]", i)
				results = append(results, got)
			}
			require.Equal(t, tc.expect, results, "Mismatch in expected results for test '%s'", tc.name)
		})
	}

	// Cleanup
	endCall(callID1)
	endCall(callID2)
}

func TestStoreIteratorHitsLimit_TableDriven(t *testing.T) {
	const limit = 2
	callID := startCall()
	store := testdb.NewMemDB()

	// We want to store iterators up to limit and then exceed
	tests := []struct {
		name       string
		numIters   int
		shouldFail bool
	}{
		{
			name:       "Store 1st iter (success)",
			numIters:   1,
			shouldFail: false,
		},
		{
			name:       "Store 2nd iter (success)",
			numIters:   2,
			shouldFail: false,
		},
		{
			name:       "Store 3rd iter (exceeds limit =2)",
			numIters:   3,
			shouldFail: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			iter := store.Iterator(nil, nil)
			_, err := storeIterator(callID, iter, limit)
			if tc.shouldFail {
				require.ErrorContains(t, err, "Reached iterator limit (2)")
			} else {
				require.NoError(t, err, "should not exceed limit for test '%s'", tc.name)
			}
		})
	}

	endCall(callID)
}

func TestRetrieveIterator_TableDriven(t *testing.T) {
	const limit = 2000
	callID1 := startCall()
	callID2 := startCall()

	store := testdb.NewMemDB()

	// Setup initial iterators
	iterA := store.Iterator(nil, nil)
	idA, err := storeIterator(callID1, iterA, limit)
	require.NoError(t, err)
	iterB := store.Iterator(nil, nil)
	_, err = storeIterator(callID1, iterB, limit)
	require.NoError(t, err)

	iterC := store.Iterator(nil, nil)
	_, err = storeIterator(callID2, iterC, limit)
	require.NoError(t, err)
	iterD := store.Iterator(nil, nil)
	idD, err := storeIterator(callID2, iterD, limit)
	require.NoError(t, err)
	iterE := store.Iterator(nil, nil)
	idE, err := storeIterator(callID2, iterE, limit)
	require.NoError(t, err)

	tests := []struct {
		name      string
		callID    uint64
		iterID    uint64
		expectNil bool
	}{
		{
			name:      "Retrieve existing iter idA on callID1",
			callID:    callID1,
			iterID:    idA,
			expectNil: false,
		},
		{
			name:      "Retrieve existing iter idD on callID2",
			callID:    callID2,
			iterID:    idD,
			expectNil: false,
		},
		{
			name:      "Retrieve ID from different callID => nil",
			callID:    callID1,
			iterID:    idE, // e belongs to callID2
			expectNil: true,
		},
		{
			name:      "Retrieve zero => nil",
			callID:    callID1,
			iterID:    0,
			expectNil: true,
		},
		{
			name:      "Retrieve large => nil",
			callID:    callID1,
			iterID:    18446744073709551615,
			expectNil: true,
		},
		{
			name:      "Non-existent callID => nil",
			callID:    callID1 + 1234567,
			iterID:    idE,
			expectNil: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			iter := retrieveIterator(tc.callID, tc.iterID)
			if tc.expectNil {
				require.Nil(t, iter, "expected nil for test: %s", tc.name)
			} else {
				require.NotNil(t, iter, "expected a valid iterator for test: %s", tc.name)
			}
		})
	}

	endCall(callID1)
	endCall(callID2)
}

func TestQueueIteratorSimple_TableDriven(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()

	setup := setupQueueContract(t, cache)
	checksum, querier, api := setup.checksum, setup.querier, setup.api

	tests := []struct {
		name    string
		query   string
		expErr  string
		expResp string
	}{
		{
			name:    "sum query => 39",
			query:   `{"sum":{}}`,
			expErr:  "",
			expResp: `{"sum":39}`,
		},
		{
			name:    "reducer query => counters",
			query:   `{"reducer":{}}`,
			expErr:  "",
			expResp: `{"counters":[[17,22],[22,0]]}`,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			gasMeter := NewMockGasMeter(TESTING_GAS_LIMIT)
			igasMeter := types.GasMeter(gasMeter)
			store := setup.Store(gasMeter)
			env := MockEnvBin(t)

			data, _, err := Query(
				cache,
				checksum,
				env,
				[]byte(tc.query),
				&igasMeter,
				store,
				api,
				&querier,
				TESTING_GAS_LIMIT,
				TESTING_PRINT_DEBUG,
			)
			require.NoError(t, err, "Query must not fail in scenario: %s", tc.name)

			var result types.QueryResult
			err = json.Unmarshal(data, &result)
			require.NoError(t, err,
				"JSON decode of QueryResult must succeed in scenario: %s", tc.name)
			require.Equal(t, tc.expErr, result.Err,
				"Mismatch in 'Err' for scenario %s", tc.name)
			require.Equal(t, tc.expResp, string(result.Ok),
				"Mismatch in 'Ok' response for scenario %s", tc.name)
		})
	}
}

func TestQueueIteratorRaces_TableDriven(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()

	require.Empty(t, iteratorFrames)

	contract1 := setupQueueContractWithData(t, cache, 17, 22)
	contract2 := setupQueueContractWithData(t, cache, 1, 19, 6, 35, 8)
	contract3 := setupQueueContractWithData(t, cache, 11, 6, 2)
	env := MockEnvBin(t)

	reduceQuery := func(t *testing.T, c queueData, expected string) {
		checksum, querier, api := c.checksum, c.querier, c.api
		gasMeter := NewMockGasMeter(TESTING_GAS_LIMIT)
		igasMeter := types.GasMeter(gasMeter)
		store := c.Store(gasMeter)

		query := []byte(`{"reducer":{}}`)
		data, _, err := Query(cache, checksum, env, query, &igasMeter, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
		require.NoError(t, err)
		var r types.QueryResult
		err = json.Unmarshal(data, &r)
		require.NoError(t, err)
		require.Equal(t, "", r.Err)
		require.Equal(t, fmt.Sprintf(`{"counters":%s}`, expected), string(r.Ok))
	}

	// We define a table for the concurrent contract calls
	tests := []struct {
		name           string
		contract       queueData
		expectedResult string
	}{
		{"contract1", contract1, "[[17,22],[22,0]]"},
		{"contract2", contract2, "[[1,68],[19,35],[6,62],[35,0],[8,54]]"},
		{"contract3", contract3, "[[11,0],[6,11],[2,17]]"},
	}

	const numBatches = 30
	var wg sync.WaitGroup
	wg.Add(numBatches * len(tests))

	// The same concurrency approach, but now in a loop
	for i := 0; i < numBatches; i++ {
		for _, tc := range tests {
			tc := tc
			go func() {
				reduceQuery(t, tc.contract, tc.expectedResult)
				wg.Done()
			}()
		}
	}
	wg.Wait()

	// when they finish, we should have removed all frames
	require.Empty(t, iteratorFrames)
}

func TestQueueIteratorLimit_TableDriven(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()

	setup := setupQueueContract(t, cache)
	checksum, querier, api := setup.checksum, setup.querier, setup.api

	tests := []struct {
		name        string
		count       int
		multiplier  int
		expectError bool
		errContains string
	}{
		{
			name:        "Open 5000 iterators, no error",
			count:       5000,
			multiplier:  1,
			expectError: false,
		},
		{
			name:        "Open 35000 iterators => exceed limit(32768)",
			count:       35000,
			multiplier:  4,
			expectError: true,
			errContains: "Reached iterator limit (32768)",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			gasLimit := TESTING_GAS_LIMIT * uint64(tc.multiplier)
			gasMeter := NewMockGasMeter(gasLimit)
			igasMeter := types.GasMeter(gasMeter)
			store := setup.Store(gasMeter)
			env := MockEnvBin(t)

			msg := fmt.Sprintf(`{"open_iterators":{"count":%d}}`, tc.count)
			data, _, err := Query(cache, checksum, env, []byte(msg), &igasMeter, store, api, &querier, gasLimit, TESTING_PRINT_DEBUG)
			if tc.expectError {
				require.Error(t, err, "Expected an error in test '%s'", tc.name)
				require.Contains(t, err.Error(), tc.errContains, "Error mismatch in test '%s'", tc.name)
				return
			}
			require.NoError(t, err, "No error expected in test '%s'", tc.name)

			// decode the success
			var qResult types.QueryResult
			err = json.Unmarshal(data, &qResult)
			require.NoError(t, err, "JSON decode must succeed in test '%s'", tc.name)
			require.Equal(t, "", qResult.Err, "Expected no error in QueryResult for test '%s'", tc.name)
			require.Equal(t, `{}`, string(qResult.Ok),
				"Expected an empty obj response for test '%s'", tc.name)
		})
	}
}

//--------------------
// Suggestions
//--------------------
//
// 1. We added more debug logs (e.g., inline string formatting, ensuring we mention scenario names).
// 2. For concurrency tests (like "races"), we used table-driven expansions for concurrency loops.
// 3. We introduced partial success/failure checks for error messages using `require.Contains` or `require.Equal`.
// 4. You can expand your negative test cases to verify what happens if the KVStore fails or the env is invalid.
// 5. For even more thorough coverage, you might add invalid parameters or zero-limit scenarios to the tables.

```
---
### `lib.go`
*2025-02-15 10:18:01 | 27 KB*
```go
package api

// #include <stdlib.h>
// #include "bindings.h"
import "C"

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"

	"github.com/CosmWasm/wasmvm/v2/types"
)

// Value types
type (
	cint   = C.int
	cbool  = C.bool
	cusize = C.size_t
	cu8    = C.uint8_t
	cu32   = C.uint32_t
	cu64   = C.uint64_t
	ci8    = C.int8_t
	ci32   = C.int32_t
	ci64   = C.int64_t
)

// Pointers
type (
	cu8_ptr = *C.uint8_t
)

type Cache struct {
	ptr      *C.cache_t
	lockfile os.File
}

type Querier = types.Querier

func InitCache(config types.VMConfig) (Cache, error) {
	// libwasmvm would create this directory too but we need it earlier for the lockfile
	err := os.MkdirAll(config.Cache.BaseDir, 0o755)
	if err != nil {
		return Cache{}, fmt.Errorf("Could not create base directory")
	}

	lockfile, err := os.OpenFile(filepath.Join(config.Cache.BaseDir, "exclusive.lock"), os.O_WRONLY|os.O_CREATE, 0o666)
	if err != nil {
		return Cache{}, fmt.Errorf("Could not open exclusive.lock")
	}
	_, err = lockfile.WriteString("This is a lockfile that prevent two VM instances to operate on the same directory in parallel.\nSee codebase at github.com/CosmWasm/wasmvm for more information.\nSafety first – brought to you by Confio ❤️\n")
	if err != nil {
		return Cache{}, fmt.Errorf("Error writing to exclusive.lock")
	}

	err = unix.Flock(int(lockfile.Fd()), unix.LOCK_EX|unix.LOCK_NB)
	if err != nil {
		return Cache{}, fmt.Errorf("Could not lock exclusive.lock. Is a different VM running in the same directory already?")
	}

	configBytes, err := json.Marshal(config)
	if err != nil {
		return Cache{}, fmt.Errorf("Could not serialize config")
	}
	configView := makeView(configBytes)
	defer runtime.KeepAlive(configBytes)

	errmsg := uninitializedUnmanagedVector()

	ptr, err := C.init_cache(configView, &errmsg)
	if err != nil {
		return Cache{}, errorWithMessage(err, errmsg)
	}
	return Cache{ptr: ptr, lockfile: *lockfile}, nil
}

func ReleaseCache(cache Cache) {
	C.release_cache(cache.ptr)

	cache.lockfile.Close() // Also releases the file lock
}

func StoreCode(cache Cache, wasm []byte, persist bool) ([]byte, error) {
	w := makeView(wasm)
	defer runtime.KeepAlive(wasm)
	errmsg := uninitializedUnmanagedVector()
	checksum, err := C.store_code(cache.ptr, w, cbool(true), cbool(persist), &errmsg)
	if err != nil {
		return nil, errorWithMessage(err, errmsg)
	}
	return copyAndDestroyUnmanagedVector(checksum), nil
}

func StoreCodeUnchecked(cache Cache, wasm []byte) ([]byte, error) {
	w := makeView(wasm)
	defer runtime.KeepAlive(wasm)
	errmsg := uninitializedUnmanagedVector()
	checksum, err := C.store_code(cache.ptr, w, cbool(false), cbool(true), &errmsg)
	if err != nil {
		return nil, errorWithMessage(err, errmsg)
	}
	return copyAndDestroyUnmanagedVector(checksum), nil
}

func RemoveCode(cache Cache, checksum []byte) error {
	cs := makeView(checksum)
	defer runtime.KeepAlive(checksum)
	errmsg := uninitializedUnmanagedVector()
	_, err := C.remove_wasm(cache.ptr, cs, &errmsg)
	if err != nil {
		return errorWithMessage(err, errmsg)
	}
	return nil
}

func GetCode(cache Cache, checksum []byte) ([]byte, error) {
	cs := makeView(checksum)
	defer runtime.KeepAlive(checksum)
	errmsg := uninitializedUnmanagedVector()
	wasm, err := C.load_wasm(cache.ptr, cs, &errmsg)
	if err != nil {
		return nil, errorWithMessage(err, errmsg)
	}
	return copyAndDestroyUnmanagedVector(wasm), nil
}

func Pin(cache Cache, checksum []byte) error {
	cs := makeView(checksum)
	defer runtime.KeepAlive(checksum)
	errmsg := uninitializedUnmanagedVector()
	_, err := C.pin(cache.ptr, cs, &errmsg)
	if err != nil {
		return errorWithMessage(err, errmsg)
	}
	return nil
}

func Unpin(cache Cache, checksum []byte) error {
	cs := makeView(checksum)
	defer runtime.KeepAlive(checksum)
	errmsg := uninitializedUnmanagedVector()
	_, err := C.unpin(cache.ptr, cs, &errmsg)
	if err != nil {
		return errorWithMessage(err, errmsg)
	}
	return nil
}

func AnalyzeCode(cache Cache, checksum []byte) (*types.AnalysisReport, error) {
	cs := makeView(checksum)
	defer runtime.KeepAlive(checksum)
	errmsg := uninitializedUnmanagedVector()
	report, err := C.analyze_code(cache.ptr, cs, &errmsg)
	if err != nil {
		return nil, errorWithMessage(err, errmsg)
	}
	requiredCapabilities := string(copyAndDestroyUnmanagedVector(report.required_capabilities))
	entrypoints := string(copyAndDestroyUnmanagedVector(report.entrypoints))

	res := types.AnalysisReport{
		HasIBCEntryPoints:      bool(report.has_ibc_entry_points),
		RequiredCapabilities:   requiredCapabilities,
		Entrypoints:            strings.Split(entrypoints, ","),
		ContractMigrateVersion: optionalU64ToPtr(report.contract_migrate_version),
	}
	return &res, nil
}

func GetMetrics(cache Cache) (*types.Metrics, error) {
	errmsg := uninitializedUnmanagedVector()
	metrics, err := C.get_metrics(cache.ptr, &errmsg)
	if err != nil {
		return nil, errorWithMessage(err, errmsg)
	}

	return &types.Metrics{
		HitsPinnedMemoryCache:     uint32(metrics.hits_pinned_memory_cache),
		HitsMemoryCache:           uint32(metrics.hits_memory_cache),
		HitsFsCache:               uint32(metrics.hits_fs_cache),
		Misses:                    uint32(metrics.misses),
		ElementsPinnedMemoryCache: uint64(metrics.elements_pinned_memory_cache),
		ElementsMemoryCache:       uint64(metrics.elements_memory_cache),
		SizePinnedMemoryCache:     uint64(metrics.size_pinned_memory_cache),
		SizeMemoryCache:           uint64(metrics.size_memory_cache),
	}, nil
}

func GetPinnedMetrics(cache Cache) (*types.PinnedMetrics, error) {
	errmsg := uninitializedUnmanagedVector()
	metrics, err := C.get_pinned_metrics(cache.ptr, &errmsg)
	if err != nil {
		return nil, errorWithMessage(err, errmsg)
	}

	var pinnedMetrics types.PinnedMetrics
	if err := pinnedMetrics.UnmarshalMessagePack(copyAndDestroyUnmanagedVector(metrics)); err != nil {
		return nil, err
	}

	return &pinnedMetrics, nil
}

func Instantiate(
	cache Cache,
	checksum []byte,
	env []byte,
	info []byte,
	msg []byte,
	gasMeter *types.GasMeter,
	store types.KVStore,
	api *types.GoAPI,
	querier *Querier,
	gasLimit uint64,
	printDebug bool,
) ([]byte, types.GasReport, error) {
	cs := makeView(checksum)
	defer runtime.KeepAlive(checksum)
	e := makeView(env)
	defer runtime.KeepAlive(env)
	i := makeView(info)
	defer runtime.KeepAlive(info)
	m := makeView(msg)
	defer runtime.KeepAlive(msg)
	var pinner runtime.Pinner
	pinner.Pin(gasMeter)
	checkAndPinAPI(api, pinner)
	checkAndPinQuerier(querier, pinner)
	defer pinner.Unpin()

	callID := startCall()
	defer endCall(callID)

	dbState := buildDBState(store, callID)
	db := buildDB(&dbState, gasMeter)
	a := buildAPI(api)
	q := buildQuerier(querier)
	var gasReport C.GasReport
	errmsg := uninitializedUnmanagedVector()

	res, err := C.instantiate(cache.ptr, cs, e, i, m, db, a, q, cu64(gasLimit), cbool(printDebug), &gasReport, &errmsg)
	if err != nil && err.(syscall.Errno) != C.ErrnoValue_Success {
		// Depending on the nature of the error, `gasUsed` will either have a meaningful value, or just 0.
		return nil, convertGasReport(gasReport), errorWithMessage(err, errmsg)
	}
	return copyAndDestroyUnmanagedVector(res), convertGasReport(gasReport), nil
}

func Execute(
	cache Cache,
	checksum []byte,
	env []byte,
	info []byte,
	msg []byte,
	gasMeter *types.GasMeter,
	store types.KVStore,
	api *types.GoAPI,
	querier *Querier,
	gasLimit uint64,
	printDebug bool,
) ([]byte, types.GasReport, error) {
	cs := makeView(checksum)
	defer runtime.KeepAlive(checksum)
	e := makeView(env)
	defer runtime.KeepAlive(env)
	i := makeView(info)
	defer runtime.KeepAlive(info)
	m := makeView(msg)
	defer runtime.KeepAlive(msg)
	var pinner runtime.Pinner
	pinner.Pin(gasMeter)
	checkAndPinAPI(api, pinner)
	checkAndPinQuerier(querier, pinner)
	defer pinner.Unpin()

	callID := startCall()
	defer endCall(callID)

	dbState := buildDBState(store, callID)
	db := buildDB(&dbState, gasMeter)
	a := buildAPI(api)
	q := buildQuerier(querier)
	var gasReport C.GasReport
	errmsg := uninitializedUnmanagedVector()

	res, err := C.execute(cache.ptr, cs, e, i, m, db, a, q, cu64(gasLimit), cbool(printDebug), &gasReport, &errmsg)
	if err != nil && err.(syscall.Errno) != C.ErrnoValue_Success {
		// Depending on the nature of the error, `gasUsed` will either have a meaningful value, or just 0.
		return nil, convertGasReport(gasReport), errorWithMessage(err, errmsg)
	}
	return copyAndDestroyUnmanagedVector(res), convertGasReport(gasReport), nil
}

func Migrate(
	cache Cache,
	checksum []byte,
	env []byte,
	msg []byte,
	gasMeter *types.GasMeter,
	store types.KVStore,
	api *types.GoAPI,
	querier *Querier,
	gasLimit uint64,
	printDebug bool,
) ([]byte, types.GasReport, error) {
	cs := makeView(checksum)
	defer runtime.KeepAlive(checksum)
	e := makeView(env)
	defer runtime.KeepAlive(env)
	m := makeView(msg)
	defer runtime.KeepAlive(msg)
	var pinner runtime.Pinner
	pinner.Pin(gasMeter)
	checkAndPinAPI(api, pinner)
	checkAndPinQuerier(querier, pinner)
	defer pinner.Unpin()

	callID := startCall()
	defer endCall(callID)

	dbState := buildDBState(store, callID)
	db := buildDB(&dbState, gasMeter)
	a := buildAPI(api)
	q := buildQuerier(querier)
	var gasReport C.GasReport
	errmsg := uninitializedUnmanagedVector()

	res, err := C.migrate(cache.ptr, cs, e, m, db, a, q, cu64(gasLimit), cbool(printDebug), &gasReport, &errmsg)
	if err != nil && err.(syscall.Errno) != C.ErrnoValue_Success {
		// Depending on the nature of the error, `gasUsed` will either have a meaningful value, or just 0.
		return nil, convertGasReport(gasReport), errorWithMessage(err, errmsg)
	}
	return copyAndDestroyUnmanagedVector(res), convertGasReport(gasReport), nil
}

func MigrateWithInfo(
	cache Cache,
	checksum []byte,
	env []byte,
	msg []byte,
	migrateInfo []byte,
	gasMeter *types.GasMeter,
	store types.KVStore,
	api *types.GoAPI,
	querier *Querier,
	gasLimit uint64,
	printDebug bool,
) ([]byte, types.GasReport, error) {
	cs := makeView(checksum)
	defer runtime.KeepAlive(checksum)
	e := makeView(env)
	defer runtime.KeepAlive(env)
	m := makeView(msg)
	defer runtime.KeepAlive(msg)
	i := makeView(migrateInfo)
	defer runtime.KeepAlive(i)
	var pinner runtime.Pinner
	pinner.Pin(gasMeter)
	checkAndPinAPI(api, pinner)
	checkAndPinQuerier(querier, pinner)
	defer pinner.Unpin()

	callID := startCall()
	defer endCall(callID)

	dbState := buildDBState(store, callID)
	db := buildDB(&dbState, gasMeter)
	a := buildAPI(api)
	q := buildQuerier(querier)
	var gasReport C.GasReport
	errmsg := uninitializedUnmanagedVector()

	res, err := C.migrate_with_info(cache.ptr, cs, e, m, i, db, a, q, cu64(gasLimit), cbool(printDebug), &gasReport, &errmsg)
	if err != nil && err.(syscall.Errno) != C.ErrnoValue_Success {
		// Depending on the nature of the error, `gasUsed` will either have a meaningful value, or just 0.
		return nil, convertGasReport(gasReport), errorWithMessage(err, errmsg)
	}
	return copyAndDestroyUnmanagedVector(res), convertGasReport(gasReport), nil
}

func Sudo(
	cache Cache,
	checksum []byte,
	env []byte,
	msg []byte,
	gasMeter *types.GasMeter,
	store types.KVStore,
	api *types.GoAPI,
	querier *Querier,
	gasLimit uint64,
	printDebug bool,
) ([]byte, types.GasReport, error) {
	cs := makeView(checksum)
	defer runtime.KeepAlive(checksum)
	e := makeView(env)
	defer runtime.KeepAlive(env)
	m := makeView(msg)
	defer runtime.KeepAlive(msg)
	var pinner runtime.Pinner
	pinner.Pin(gasMeter)
	checkAndPinAPI(api, pinner)
	checkAndPinQuerier(querier, pinner)
	defer pinner.Unpin()

	callID := startCall()
	defer endCall(callID)

	dbState := buildDBState(store, callID)
	db := buildDB(&dbState, gasMeter)
	a := buildAPI(api)
	q := buildQuerier(querier)
	var gasReport C.GasReport
	errmsg := uninitializedUnmanagedVector()

	res, err := C.sudo(cache.ptr, cs, e, m, db, a, q, cu64(gasLimit), cbool(printDebug), &gasReport, &errmsg)
	if err != nil && err.(syscall.Errno) != C.ErrnoValue_Success {
		// Depending on the nature of the error, `gasUsed` will either have a meaningful value, or just 0.
		return nil, convertGasReport(gasReport), errorWithMessage(err, errmsg)
	}
	return copyAndDestroyUnmanagedVector(res), convertGasReport(gasReport), nil
}

func Reply(
	cache Cache,
	checksum []byte,
	env []byte,
	reply []byte,
	gasMeter *types.GasMeter,
	store types.KVStore,
	api *types.GoAPI,
	querier *Querier,
	gasLimit uint64,
	printDebug bool,
) ([]byte, types.GasReport, error) {
	cs := makeView(checksum)
	defer runtime.KeepAlive(checksum)
	e := makeView(env)
	defer runtime.KeepAlive(env)
	r := makeView(reply)
	defer runtime.KeepAlive(reply)
	var pinner runtime.Pinner
	pinner.Pin(gasMeter)
	checkAndPinAPI(api, pinner)
	checkAndPinQuerier(querier, pinner)
	defer pinner.Unpin()

	callID := startCall()
	defer endCall(callID)

	dbState := buildDBState(store, callID)
	db := buildDB(&dbState, gasMeter)
	a := buildAPI(api)
	q := buildQuerier(querier)
	var gasReport C.GasReport
	errmsg := uninitializedUnmanagedVector()

	res, err := C.reply(cache.ptr, cs, e, r, db, a, q, cu64(gasLimit), cbool(printDebug), &gasReport, &errmsg)
	if err != nil && err.(syscall.Errno) != C.ErrnoValue_Success {
		// Depending on the nature of the error, `gasUsed` will either have a meaningful value, or just 0.
		return nil, convertGasReport(gasReport), errorWithMessage(err, errmsg)
	}
	return copyAndDestroyUnmanagedVector(res), convertGasReport(gasReport), nil
}

func Query(
	cache Cache,
	checksum []byte,
	env []byte,
	msg []byte,
	gasMeter *types.GasMeter,
	store types.KVStore,
	api *types.GoAPI,
	querier *Querier,
	gasLimit uint64,
	printDebug bool,
) ([]byte, types.GasReport, error) {
	cs := makeView(checksum)
	defer runtime.KeepAlive(checksum)
	e := makeView(env)
	defer runtime.KeepAlive(env)
	m := makeView(msg)
	defer runtime.KeepAlive(msg)
	var pinner runtime.Pinner
	pinner.Pin(gasMeter)
	checkAndPinAPI(api, pinner)
	checkAndPinQuerier(querier, pinner)
	defer pinner.Unpin()

	callID := startCall()
	defer endCall(callID)

	dbState := buildDBState(store, callID)
	db := buildDB(&dbState, gasMeter)
	a := buildAPI(api)
	q := buildQuerier(querier)
	var gasReport C.GasReport
	errmsg := uninitializedUnmanagedVector()

	res, err := C.query(cache.ptr, cs, e, m, db, a, q, cu64(gasLimit), cbool(printDebug), &gasReport, &errmsg)
	if err != nil && err.(syscall.Errno) != C.ErrnoValue_Success {
		// Depending on the nature of the error, `gasUsed` will either have a meaningful value, or just 0.
		return nil, convertGasReport(gasReport), errorWithMessage(err, errmsg)
	}
	return copyAndDestroyUnmanagedVector(res), convertGasReport(gasReport), nil
}

func IBCChannelOpen(
	cache Cache,
	checksum []byte,
	env []byte,
	msg []byte,
	gasMeter *types.GasMeter,
	store types.KVStore,
	api *types.GoAPI,
	querier *Querier,
	gasLimit uint64,
	printDebug bool,
) ([]byte, types.GasReport, error) {
	cs := makeView(checksum)
	defer runtime.KeepAlive(checksum)
	e := makeView(env)
	defer runtime.KeepAlive(env)
	m := makeView(msg)
	defer runtime.KeepAlive(msg)
	var pinner runtime.Pinner
	pinner.Pin(gasMeter)
	checkAndPinAPI(api, pinner)
	checkAndPinQuerier(querier, pinner)
	defer pinner.Unpin()

	callID := startCall()
	defer endCall(callID)

	dbState := buildDBState(store, callID)
	db := buildDB(&dbState, gasMeter)
	a := buildAPI(api)
	q := buildQuerier(querier)
	var gasReport C.GasReport
	errmsg := uninitializedUnmanagedVector()

	res, err := C.ibc_channel_open(cache.ptr, cs, e, m, db, a, q, cu64(gasLimit), cbool(printDebug), &gasReport, &errmsg)
	if err != nil && err.(syscall.Errno) != C.ErrnoValue_Success {
		// Depending on the nature of the error, `gasUsed` will either have a meaningful value, or just 0.
		return nil, convertGasReport(gasReport), errorWithMessage(err, errmsg)
	}
	return copyAndDestroyUnmanagedVector(res), convertGasReport(gasReport), nil
}

func IBCChannelConnect(
	cache Cache,
	checksum []byte,
	env []byte,
	msg []byte,
	gasMeter *types.GasMeter,
	store types.KVStore,
	api *types.GoAPI,
	querier *Querier,
	gasLimit uint64,
	printDebug bool,
) ([]byte, types.GasReport, error) {
	cs := makeView(checksum)
	defer runtime.KeepAlive(checksum)
	e := makeView(env)
	defer runtime.KeepAlive(env)
	m := makeView(msg)
	defer runtime.KeepAlive(msg)
	var pinner runtime.Pinner
	pinner.Pin(gasMeter)
	checkAndPinAPI(api, pinner)
	checkAndPinQuerier(querier, pinner)
	defer pinner.Unpin()

	callID := startCall()
	defer endCall(callID)

	dbState := buildDBState(store, callID)
	db := buildDB(&dbState, gasMeter)
	a := buildAPI(api)
	q := buildQuerier(querier)
	var gasReport C.GasReport
	errmsg := uninitializedUnmanagedVector()

	res, err := C.ibc_channel_connect(cache.ptr, cs, e, m, db, a, q, cu64(gasLimit), cbool(printDebug), &gasReport, &errmsg)
	if err != nil && err.(syscall.Errno) != C.ErrnoValue_Success {
		// Depending on the nature of the error, `gasUsed` will either have a meaningful value, or just 0.
		return nil, convertGasReport(gasReport), errorWithMessage(err, errmsg)
	}
	return copyAndDestroyUnmanagedVector(res), convertGasReport(gasReport), nil
}

func IBCChannelClose(
	cache Cache,
	checksum []byte,
	env []byte,
	msg []byte,
	gasMeter *types.GasMeter,
	store types.KVStore,
	api *types.GoAPI,
	querier *Querier,
	gasLimit uint64,
	printDebug bool,
) ([]byte, types.GasReport, error) {
	cs := makeView(checksum)
	defer runtime.KeepAlive(checksum)
	e := makeView(env)
	defer runtime.KeepAlive(env)
	m := makeView(msg)
	defer runtime.KeepAlive(msg)
	var pinner runtime.Pinner
	pinner.Pin(gasMeter)
	checkAndPinAPI(api, pinner)
	checkAndPinQuerier(querier, pinner)
	defer pinner.Unpin()

	callID := startCall()
	defer endCall(callID)

	dbState := buildDBState(store, callID)
	db := buildDB(&dbState, gasMeter)
	a := buildAPI(api)
	q := buildQuerier(querier)
	var gasReport C.GasReport
	errmsg := uninitializedUnmanagedVector()

	res, err := C.ibc_channel_close(cache.ptr, cs, e, m, db, a, q, cu64(gasLimit), cbool(printDebug), &gasReport, &errmsg)
	if err != nil && err.(syscall.Errno) != C.ErrnoValue_Success {
		// Depending on the nature of the error, `gasUsed` will either have a meaningful value, or just 0.
		return nil, convertGasReport(gasReport), errorWithMessage(err, errmsg)
	}
	return copyAndDestroyUnmanagedVector(res), convertGasReport(gasReport), nil
}

func IBCPacketReceive(
	cache Cache,
	checksum []byte,
	env []byte,
	packet []byte,
	gasMeter *types.GasMeter,
	store types.KVStore,
	api *types.GoAPI,
	querier *Querier,
	gasLimit uint64,
	printDebug bool,
) ([]byte, types.GasReport, error) {
	cs := makeView(checksum)
	defer runtime.KeepAlive(checksum)
	e := makeView(env)
	defer runtime.KeepAlive(env)
	pa := makeView(packet)
	defer runtime.KeepAlive(packet)
	var pinner runtime.Pinner
	pinner.Pin(gasMeter)
	checkAndPinAPI(api, pinner)
	checkAndPinQuerier(querier, pinner)
	defer pinner.Unpin()

	callID := startCall()
	defer endCall(callID)

	dbState := buildDBState(store, callID)
	db := buildDB(&dbState, gasMeter)
	a := buildAPI(api)
	q := buildQuerier(querier)
	var gasReport C.GasReport
	errmsg := uninitializedUnmanagedVector()

	res, err := C.ibc_packet_receive(cache.ptr, cs, e, pa, db, a, q, cu64(gasLimit), cbool(printDebug), &gasReport, &errmsg)
	if err != nil && err.(syscall.Errno) != C.ErrnoValue_Success {
		// Depending on the nature of the error, `gasUsed` will either have a meaningful value, or just 0.
		return nil, convertGasReport(gasReport), errorWithMessage(err, errmsg)
	}
	return copyAndDestroyUnmanagedVector(res), convertGasReport(gasReport), nil
}

func IBCPacketAck(
	cache Cache,
	checksum []byte,
	env []byte,
	ack []byte,
	gasMeter *types.GasMeter,
	store types.KVStore,
	api *types.GoAPI,
	querier *Querier,
	gasLimit uint64,
	printDebug bool,
) ([]byte, types.GasReport, error) {
	cs := makeView(checksum)
	defer runtime.KeepAlive(checksum)
	e := makeView(env)
	defer runtime.KeepAlive(env)
	ac := makeView(ack)
	defer runtime.KeepAlive(ack)
	var pinner runtime.Pinner
	pinner.Pin(gasMeter)
	checkAndPinAPI(api, pinner)
	checkAndPinQuerier(querier, pinner)
	defer pinner.Unpin()

	callID := startCall()
	defer endCall(callID)

	dbState := buildDBState(store, callID)
	db := buildDB(&dbState, gasMeter)
	a := buildAPI(api)
	q := buildQuerier(querier)
	var gasReport C.GasReport
	errmsg := uninitializedUnmanagedVector()

	res, err := C.ibc_packet_ack(cache.ptr, cs, e, ac, db, a, q, cu64(gasLimit), cbool(printDebug), &gasReport, &errmsg)
	if err != nil && err.(syscall.Errno) != C.ErrnoValue_Success {
		// Depending on the nature of the error, `gasUsed` will either have a meaningful value, or just 0.
		return nil, convertGasReport(gasReport), errorWithMessage(err, errmsg)
	}
	return copyAndDestroyUnmanagedVector(res), convertGasReport(gasReport), nil
}

func IBCPacketTimeout(
	cache Cache,
	checksum []byte,
	env []byte,
	packet []byte,
	gasMeter *types.GasMeter,
	store types.KVStore,
	api *types.GoAPI,
	querier *Querier,
	gasLimit uint64,
	printDebug bool,
) ([]byte, types.GasReport, error) {
	cs := makeView(checksum)
	defer runtime.KeepAlive(checksum)
	e := makeView(env)
	defer runtime.KeepAlive(env)
	pa := makeView(packet)
	defer runtime.KeepAlive(packet)
	var pinner runtime.Pinner
	pinner.Pin(gasMeter)
	checkAndPinAPI(api, pinner)
	checkAndPinQuerier(querier, pinner)
	defer pinner.Unpin()

	callID := startCall()
	defer endCall(callID)

	dbState := buildDBState(store, callID)
	db := buildDB(&dbState, gasMeter)
	a := buildAPI(api)
	q := buildQuerier(querier)
	var gasReport C.GasReport
	errmsg := uninitializedUnmanagedVector()

	res, err := C.ibc_packet_timeout(cache.ptr, cs, e, pa, db, a, q, cu64(gasLimit), cbool(printDebug), &gasReport, &errmsg)
	if err != nil && err.(syscall.Errno) != C.ErrnoValue_Success {
		// Depending on the nature of the error, `gasUsed` will either have a meaningful value, or just 0.
		return nil, convertGasReport(gasReport), errorWithMessage(err, errmsg)
	}
	return copyAndDestroyUnmanagedVector(res), convertGasReport(gasReport), nil
}

func IBCSourceCallback(
	cache Cache,
	checksum []byte,
	env []byte,
	msg []byte,
	gasMeter *types.GasMeter,
	store types.KVStore,
	api *types.GoAPI,
	querier *Querier,
	gasLimit uint64,
	printDebug bool,
) ([]byte, types.GasReport, error) {
	cs := makeView(checksum)
	defer runtime.KeepAlive(checksum)
	e := makeView(env)
	defer runtime.KeepAlive(env)
	msgBytes := makeView(msg)
	defer runtime.KeepAlive(msg)
	var pinner runtime.Pinner
	pinner.Pin(gasMeter)
	checkAndPinAPI(api, pinner)
	checkAndPinQuerier(querier, pinner)
	defer pinner.Unpin()

	callID := startCall()
	defer endCall(callID)

	dbState := buildDBState(store, callID)
	db := buildDB(&dbState, gasMeter)
	a := buildAPI(api)
	q := buildQuerier(querier)
	var gasReport C.GasReport
	errmsg := uninitializedUnmanagedVector()

	res, err := C.ibc_source_callback(cache.ptr, cs, e, msgBytes, db, a, q, cu64(gasLimit), cbool(printDebug), &gasReport, &errmsg)
	if err != nil && err.(syscall.Errno) != C.ErrnoValue_Success {
		// Depending on the nature of the error, `gasUsed` will either have a meaningful value, or just 0.
		return nil, convertGasReport(gasReport), errorWithMessage(err, errmsg)
	}
	return copyAndDestroyUnmanagedVector(res), convertGasReport(gasReport), nil
}

func IBCDestinationCallback(
	cache Cache,
	checksum []byte,
	env []byte,
	msg []byte,
	gasMeter *types.GasMeter,
	store types.KVStore,
	api *types.GoAPI,
	querier *Querier,
	gasLimit uint64,
	printDebug bool,
) ([]byte, types.GasReport, error) {
	cs := makeView(checksum)
	defer runtime.KeepAlive(checksum)
	e := makeView(env)
	defer runtime.KeepAlive(env)
	msgBytes := makeView(msg)
	defer runtime.KeepAlive(msg)
	var pinner runtime.Pinner
	pinner.Pin(gasMeter)
	checkAndPinAPI(api, pinner)
	checkAndPinQuerier(querier, pinner)
	defer pinner.Unpin()

	callID := startCall()
	defer endCall(callID)

	dbState := buildDBState(store, callID)
	db := buildDB(&dbState, gasMeter)
	a := buildAPI(api)
	q := buildQuerier(querier)
	var gasReport C.GasReport
	errmsg := uninitializedUnmanagedVector()

	res, err := C.ibc_destination_callback(cache.ptr, cs, e, msgBytes, db, a, q, cu64(gasLimit), cbool(printDebug), &gasReport, &errmsg)
	if err != nil && err.(syscall.Errno) != C.ErrnoValue_Success {
		// Depending on the nature of the error, `gasUsed` will either have a meaningful value, or just 0.
		return nil, convertGasReport(gasReport), errorWithMessage(err, errmsg)
	}
	return copyAndDestroyUnmanagedVector(res), convertGasReport(gasReport), nil
}

func convertGasReport(report C.GasReport) types.GasReport {
	return types.GasReport{
		Limit:          uint64(report.limit),
		Remaining:      uint64(report.remaining),
		UsedExternally: uint64(report.used_externally),
		UsedInternally: uint64(report.used_internally),
	}
}

/**** To error module ***/

func errorWithMessage(err error, b C.UnmanagedVector) error {
	// we always destroy the unmanaged vector to avoid a memory leak
	msg := copyAndDestroyUnmanagedVector(b)

	// this checks for out of gas as a special case
	if errno, ok := err.(syscall.Errno); ok && int(errno) == 2 {
		return types.OutOfGasError{}
	}
	if msg == nil {
		return err
	}
	return fmt.Errorf("%s", string(msg))
}

// checkAndPinAPI checks and pins the API and relevant pointers inside of it.
// All errors will result in panics as they indicate misuse of the wasmvm API and are not expected
// to be caused by user data.
func checkAndPinAPI(api *types.GoAPI, pinner runtime.Pinner) {
	if api == nil {
		panic("API must not be nil. If you don't want to provide API functionality, please create an instance that returns an error on every call to HumanizeAddress(), CanonicalizeAddress() and ValidateAddress().")
	}

	// func cHumanizeAddress assumes this is set
	if api.HumanizeAddress == nil {
		panic("HumanizeAddress in API must not be nil. If you don't want to provide API functionality, please create an instance that returns an error on every call to HumanizeAddress(), CanonicalizeAddress() and ValidateAddress().")
	}

	// func cCanonicalizeAddress assumes this is set
	if api.CanonicalizeAddress == nil {
		panic("CanonicalizeAddress in API must not be nil. If you don't want to provide API functionality, please create an instance that returns an error on every call to HumanizeAddress(), CanonicalizeAddress() and ValidateAddress().")
	}

	// func cValidateAddress assumes this is set
	if api.ValidateAddress == nil {
		panic("ValidateAddress in API must not be nil. If you don't want to provide API functionality, please create an instance that returns an error on every call to HumanizeAddress(), CanonicalizeAddress() and ValidateAddress().")
	}

	pinner.Pin(api) // this pointer is used in Rust (`state` in `C.GoApi`) and must not change
}

// checkAndPinQuerier checks and pins the querier.
// All errors will result in panics as they indicate misuse of the wasmvm API and are not expected
// to be caused by user data.
func checkAndPinQuerier(querier *Querier, pinner runtime.Pinner) {
	if querier == nil {
		panic("Querier must not be nil. If you don't want to provide querier functionality, please create an instance that returns an error on every call to Query().")
	}

	pinner.Pin(querier) // this pointer is used in Rust (`state` in `C.GoQuerier`) and must not change
}

```
---
### `lib_test.go`
*2025-02-15 10:18:01 | 49 KB*
```go
package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/CosmWasm/wasmvm/v2/types"
)

const (
	TESTING_PRINT_DEBUG  = false
	TESTING_GAS_LIMIT    = uint64(500_000_000_000) // ~0.5ms
	TESTING_MEMORY_LIMIT = 32                      // MiB
	TESTING_CACHE_SIZE   = 100                     // MiB
)

var TESTING_CAPABILITIES = []string{"staking", "stargate", "iterator", "cosmwasm_1_1", "cosmwasm_1_2", "cosmwasm_1_3"}

func TestInitAndReleaseCache(t *testing.T) {
	tmpdir, err := os.MkdirTemp("", "wasmvm-testing")
	require.NoError(t, err)
	defer os.RemoveAll(tmpdir)

	config := types.VMConfig{
		Cache: types.CacheOptions{
			BaseDir:                  tmpdir,
			AvailableCapabilities:    TESTING_CAPABILITIES,
			MemoryCacheSizeBytes:     types.NewSizeMebi(TESTING_CACHE_SIZE),
			InstanceMemoryLimitBytes: types.NewSizeMebi(TESTING_MEMORY_LIMIT),
		},
	}
	cache, err := InitCache(config)
	require.NoError(t, err)
	ReleaseCache(cache)
}

// wasmd expects us to create the base directory
// https://github.com/CosmWasm/wasmd/blob/v0.30.0/x/wasm/keeper/keeper.go#L128
func TestInitCacheWorksForNonExistentDir(t *testing.T) {
	tmpdir, err := os.MkdirTemp("", "wasmvm-testing")
	require.NoError(t, err)
	defer os.RemoveAll(tmpdir)

	createMe := filepath.Join(tmpdir, "does-not-yet-exist")
	config := types.VMConfig{
		Cache: types.CacheOptions{
			BaseDir:                  createMe,
			AvailableCapabilities:    TESTING_CAPABILITIES,
			MemoryCacheSizeBytes:     types.NewSizeMebi(TESTING_CACHE_SIZE),
			InstanceMemoryLimitBytes: types.NewSizeMebi(TESTING_MEMORY_LIMIT),
		},
	}
	cache, err := InitCache(config)
	require.NoError(t, err)
	ReleaseCache(cache)
}

func TestInitCacheErrorsForBrokenDir(t *testing.T) {
	// Use colon to make this fail on Windows
	// https://gist.github.com/doctaphred/d01d05291546186941e1b7ddc02034d3
	// On Unix we should not have permission to create this.
	cannotBeCreated := "/foo:bar"
	config := types.VMConfig{
		Cache: types.CacheOptions{
			BaseDir:                  cannotBeCreated,
			AvailableCapabilities:    TESTING_CAPABILITIES,
			MemoryCacheSizeBytes:     types.NewSizeMebi(TESTING_CACHE_SIZE),
			InstanceMemoryLimitBytes: types.NewSizeMebi(TESTING_MEMORY_LIMIT),
		},
	}
	_, err := InitCache(config)
	require.ErrorContains(t, err, "Could not create base directory")
}

func TestInitLockingPreventsConcurrentAccess(t *testing.T) {
	tmpdir, err := os.MkdirTemp("", "wasmvm-testing")
	require.NoError(t, err)
	defer os.RemoveAll(tmpdir)

	config1 := types.VMConfig{
		Cache: types.CacheOptions{
			BaseDir:                  tmpdir,
			AvailableCapabilities:    TESTING_CAPABILITIES,
			MemoryCacheSizeBytes:     types.NewSizeMebi(TESTING_CACHE_SIZE),
			InstanceMemoryLimitBytes: types.NewSizeMebi(TESTING_MEMORY_LIMIT),
		},
	}
	cache1, err1 := InitCache(config1)
	require.NoError(t, err1)

	config2 := types.VMConfig{
		Cache: types.CacheOptions{
			BaseDir:                  tmpdir,
			AvailableCapabilities:    TESTING_CAPABILITIES,
			MemoryCacheSizeBytes:     types.NewSizeMebi(TESTING_CACHE_SIZE),
			InstanceMemoryLimitBytes: types.NewSizeMebi(TESTING_MEMORY_LIMIT),
		},
	}
	_, err2 := InitCache(config2)
	require.ErrorContains(t, err2, "Could not lock exclusive.lock")

	ReleaseCache(cache1)

	// Now we can try again
	config3 := types.VMConfig{
		Cache: types.CacheOptions{
			BaseDir:                  tmpdir,
			AvailableCapabilities:    TESTING_CAPABILITIES,
			MemoryCacheSizeBytes:     types.NewSizeMebi(TESTING_CACHE_SIZE),
			InstanceMemoryLimitBytes: types.NewSizeMebi(TESTING_MEMORY_LIMIT),
		},
	}
	cache3, err3 := InitCache(config3)
	require.NoError(t, err3)
	ReleaseCache(cache3)
}

func TestInitLockingAllowsMultipleInstancesInDifferentDirs(t *testing.T) {
	tmpdir1, err := os.MkdirTemp("", "wasmvm-testing1")
	require.NoError(t, err)
	tmpdir2, err := os.MkdirTemp("", "wasmvm-testing2")
	require.NoError(t, err)
	tmpdir3, err := os.MkdirTemp("", "wasmvm-testing3")
	require.NoError(t, err)
	defer os.RemoveAll(tmpdir1)
	defer os.RemoveAll(tmpdir2)
	defer os.RemoveAll(tmpdir3)

	config1 := types.VMConfig{
		Cache: types.CacheOptions{
			BaseDir:                  tmpdir1,
			AvailableCapabilities:    TESTING_CAPABILITIES,
			MemoryCacheSizeBytes:     types.NewSizeMebi(TESTING_CACHE_SIZE),
			InstanceMemoryLimitBytes: types.NewSizeMebi(TESTING_MEMORY_LIMIT),
		},
	}
	cache1, err1 := InitCache(config1)
	require.NoError(t, err1)
	config2 := types.VMConfig{
		Cache: types.CacheOptions{
			BaseDir:                  tmpdir2,
			AvailableCapabilities:    TESTING_CAPABILITIES,
			MemoryCacheSizeBytes:     types.NewSizeMebi(TESTING_CACHE_SIZE),
			InstanceMemoryLimitBytes: types.NewSizeMebi(TESTING_MEMORY_LIMIT),
		},
	}
	cache2, err2 := InitCache(config2)
	require.NoError(t, err2)
	config3 := types.VMConfig{
		Cache: types.CacheOptions{
			BaseDir:                  tmpdir3,
			AvailableCapabilities:    TESTING_CAPABILITIES,
			MemoryCacheSizeBytes:     types.NewSizeMebi(TESTING_CACHE_SIZE),
			InstanceMemoryLimitBytes: types.NewSizeMebi(TESTING_MEMORY_LIMIT),
		},
	}
	cache3, err3 := InitCache(config3)
	require.NoError(t, err3)

	ReleaseCache(cache1)
	ReleaseCache(cache2)
	ReleaseCache(cache3)
}

func TestInitCacheEmptyCapabilities(t *testing.T) {
	tmpdir, err := os.MkdirTemp("", "wasmvm-testing")
	require.NoError(t, err)
	defer os.RemoveAll(tmpdir)
	config := types.VMConfig{
		Cache: types.CacheOptions{
			BaseDir:                  tmpdir,
			AvailableCapabilities:    []string{},
			MemoryCacheSizeBytes:     types.NewSizeMebi(TESTING_CACHE_SIZE),
			InstanceMemoryLimitBytes: types.NewSizeMebi(TESTING_MEMORY_LIMIT),
		},
	}
	cache, err := InitCache(config)
	require.NoError(t, err)
	ReleaseCache(cache)
}

func withCache(t testing.TB) (Cache, func()) {
	tmpdir, err := os.MkdirTemp("", "wasmvm-testing")
	require.NoError(t, err)
	config := types.VMConfig{
		Cache: types.CacheOptions{
			BaseDir:                  tmpdir,
			AvailableCapabilities:    TESTING_CAPABILITIES,
			MemoryCacheSizeBytes:     types.NewSizeMebi(TESTING_CACHE_SIZE),
			InstanceMemoryLimitBytes: types.NewSizeMebi(TESTING_MEMORY_LIMIT),
		},
	}
	cache, err := InitCache(config)
	require.NoError(t, err)

	cleanup := func() {
		os.RemoveAll(tmpdir)
		ReleaseCache(cache)
	}
	return cache, cleanup
}

func TestStoreCodeAndGetCode(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()

	wasm, err := os.ReadFile("../../testdata/hackatom.wasm")
	require.NoError(t, err)

	checksum, err := StoreCode(cache, wasm, true)
	require.NoError(t, err)
	expectedChecksum := sha256.Sum256(wasm)
	require.Equal(t, expectedChecksum[:], checksum)

	code, err := GetCode(cache, checksum)
	require.NoError(t, err)
	require.Equal(t, wasm, code)
}

func TestRemoveCode(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()

	wasm, err := os.ReadFile("../../testdata/hackatom.wasm")
	require.NoError(t, err)

	checksum, err := StoreCode(cache, wasm, true)
	require.NoError(t, err)

	// First removal works
	err = RemoveCode(cache, checksum)
	require.NoError(t, err)

	// Second removal fails
	err = RemoveCode(cache, checksum)
	require.ErrorContains(t, err, "Wasm file does not exist")
}

func TestStoreCodeFailsWithBadData(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()

	wasm := []byte("some invalid data")
	_, err := StoreCode(cache, wasm, true)
	require.Error(t, err)
}

func TestStoreCodeUnchecked(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()

	wasm, err := os.ReadFile("../../testdata/hackatom.wasm")
	require.NoError(t, err)

	checksum, err := StoreCodeUnchecked(cache, wasm)
	require.NoError(t, err)
	expectedChecksum := sha256.Sum256(wasm)
	require.Equal(t, expectedChecksum[:], checksum)

	code, err := GetCode(cache, checksum)
	require.NoError(t, err)
	require.Equal(t, wasm, code)
}

func TestStoreCodeUncheckedWorksWithInvalidWasm(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()

	wasm, err := os.ReadFile("../../testdata/hackatom.wasm")
	require.NoError(t, err)

	// Look for "interface_version_8" in the wasm file and replace it with "interface_version_9".
	// This makes the wasm file invalid.
	wasm = bytes.Replace(wasm, []byte("interface_version_8"), []byte("interface_version_9"), 1)

	// StoreCode should fail
	_, err = StoreCode(cache, wasm, true)
	require.ErrorContains(t, err, "Wasm contract has unknown interface_version_* marker export")

	// StoreCodeUnchecked should not fail
	checksum, err := StoreCodeUnchecked(cache, wasm)
	require.NoError(t, err)
	expectedChecksum := sha256.Sum256(wasm)
	assert.Equal(t, expectedChecksum[:], checksum)
}

func TestPin(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()

	wasm, err := os.ReadFile("../../testdata/hackatom.wasm")
	require.NoError(t, err)

	checksum, err := StoreCode(cache, wasm, true)
	require.NoError(t, err)

	err = Pin(cache, checksum)
	require.NoError(t, err)

	// Can be called again with no effect
	err = Pin(cache, checksum)
	require.NoError(t, err)
}

func TestPinErrors(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()
	var err error

	// Nil checksum (errors in wasmvm Rust code)
	var nilChecksum []byte
	err = Pin(cache, nilChecksum)
	require.ErrorContains(t, err, "Null/Nil argument: checksum")

	// Checksum too short (errors in wasmvm Rust code)
	brokenChecksum := []byte{0x3f, 0xd7, 0x5a, 0x76}
	err = Pin(cache, brokenChecksum)
	require.ErrorContains(t, err, "Checksum not of length 32")

	// Unknown checksum (errors in cosmwasm-vm)
	unknownChecksum := []byte{
		0x72, 0x2c, 0x8c, 0x99, 0x3f, 0xd7, 0x5a, 0x76, 0x27, 0xd6, 0x9e, 0xd9, 0x41, 0x34,
		0x4f, 0xe2, 0xa1, 0x42, 0x3a, 0x3e, 0x75, 0xef, 0xd3, 0xe6, 0x77, 0x8a, 0x14, 0x28,
		0x84, 0x22, 0x71, 0x04,
	}
	err = Pin(cache, unknownChecksum)
	require.ErrorContains(t, err, "Error opening Wasm file for reading")
}

func TestUnpin(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()

	wasm, err := os.ReadFile("../../testdata/hackatom.wasm")
	require.NoError(t, err)

	checksum, err := StoreCode(cache, wasm, true)
	require.NoError(t, err)

	err = Pin(cache, checksum)
	require.NoError(t, err)

	err = Unpin(cache, checksum)
	require.NoError(t, err)

	// Can be called again with no effect
	err = Unpin(cache, checksum)
	require.NoError(t, err)
}

func TestUnpinErrors(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()
	var err error

	// Nil checksum (errors in wasmvm Rust code)
	var nilChecksum []byte
	err = Unpin(cache, nilChecksum)
	require.ErrorContains(t, err, "Null/Nil argument: checksum")

	// Checksum too short (errors in wasmvm Rust code)
	brokenChecksum := []byte{0x3f, 0xd7, 0x5a, 0x76}
	err = Unpin(cache, brokenChecksum)
	require.ErrorContains(t, err, "Checksum not of length 32")

	// No error case triggered in cosmwasm-vm is known right now
}

func TestGetMetrics(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()

	// GetMetrics 1
	metrics, err := GetMetrics(cache)
	require.NoError(t, err)
	require.Equal(t, &types.Metrics{}, metrics)

	// Store contract
	wasm, err := os.ReadFile("../../testdata/hackatom.wasm")
	require.NoError(t, err)
	checksum, err := StoreCode(cache, wasm, true)
	require.NoError(t, err)

	// GetMetrics 2
	metrics, err = GetMetrics(cache)
	require.NoError(t, err)
	require.Equal(t, &types.Metrics{}, metrics)

	// Instantiate 1
	gasMeter := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter := types.GasMeter(gasMeter)
	store := NewLookup(gasMeter)
	api := NewMockAPI()
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, types.Array[types.Coin]{types.NewCoin(100, "ATOM")})
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")
	msg1 := []byte(`{"verifier": "fred", "beneficiary": "bob"}`)
	_, _, err = Instantiate(cache, checksum, env, info, msg1, &igasMeter, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)

	// GetMetrics 3
	metrics, err = GetMetrics(cache)
	require.NoError(t, err)
	require.Equal(t, uint32(0), metrics.HitsMemoryCache)
	require.Equal(t, uint32(1), metrics.HitsFsCache)
	require.Equal(t, uint64(1), metrics.ElementsMemoryCache)
	require.InEpsilon(t, 3700000, metrics.SizeMemoryCache, 0.25)

	// Instantiate 2
	msg2 := []byte(`{"verifier": "fred", "beneficiary": "susi"}`)
	_, _, err = Instantiate(cache, checksum, env, info, msg2, &igasMeter, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)

	// GetMetrics 4
	metrics, err = GetMetrics(cache)
	require.NoError(t, err)
	require.Equal(t, uint32(1), metrics.HitsMemoryCache)
	require.Equal(t, uint32(1), metrics.HitsFsCache)
	require.Equal(t, uint64(1), metrics.ElementsMemoryCache)
	require.InEpsilon(t, 3700000, metrics.SizeMemoryCache, 0.25)

	// Pin
	err = Pin(cache, checksum)
	require.NoError(t, err)

	// GetMetrics 5
	metrics, err = GetMetrics(cache)
	require.NoError(t, err)
	require.Equal(t, uint32(1), metrics.HitsMemoryCache)
	require.Equal(t, uint32(2), metrics.HitsFsCache)
	require.Equal(t, uint64(1), metrics.ElementsPinnedMemoryCache)
	require.Equal(t, uint64(1), metrics.ElementsMemoryCache)
	require.InEpsilon(t, 3700000, metrics.SizePinnedMemoryCache, 0.25)
	require.InEpsilon(t, 3700000, metrics.SizeMemoryCache, 0.25)

	// Instantiate 3
	msg3 := []byte(`{"verifier": "fred", "beneficiary": "bert"}`)
	_, _, err = Instantiate(cache, checksum, env, info, msg3, &igasMeter, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)

	// GetMetrics 6
	metrics, err = GetMetrics(cache)
	require.NoError(t, err)
	require.Equal(t, uint32(1), metrics.HitsPinnedMemoryCache)
	require.Equal(t, uint32(1), metrics.HitsMemoryCache)
	require.Equal(t, uint32(2), metrics.HitsFsCache)
	require.Equal(t, uint64(1), metrics.ElementsPinnedMemoryCache)
	require.Equal(t, uint64(1), metrics.ElementsMemoryCache)
	require.InEpsilon(t, 3700000, metrics.SizePinnedMemoryCache, 0.25)
	require.InEpsilon(t, 3700000, metrics.SizeMemoryCache, 0.25)

	// Unpin
	err = Unpin(cache, checksum)
	require.NoError(t, err)

	// GetMetrics 7
	metrics, err = GetMetrics(cache)
	require.NoError(t, err)
	require.Equal(t, uint32(1), metrics.HitsPinnedMemoryCache)
	require.Equal(t, uint32(1), metrics.HitsMemoryCache)
	require.Equal(t, uint32(2), metrics.HitsFsCache)
	require.Equal(t, uint64(0), metrics.ElementsPinnedMemoryCache)
	require.Equal(t, uint64(1), metrics.ElementsMemoryCache)
	require.Equal(t, uint64(0), metrics.SizePinnedMemoryCache)
	require.InEpsilon(t, 3700000, metrics.SizeMemoryCache, 0.25)

	// Instantiate 4
	msg4 := []byte(`{"verifier": "fred", "beneficiary": "jeff"}`)
	_, _, err = Instantiate(cache, checksum, env, info, msg4, &igasMeter, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)

	// GetMetrics 8
	metrics, err = GetMetrics(cache)
	require.NoError(t, err)
	require.Equal(t, uint32(1), metrics.HitsPinnedMemoryCache)
	require.Equal(t, uint32(2), metrics.HitsMemoryCache)
	require.Equal(t, uint32(2), metrics.HitsFsCache)
	require.Equal(t, uint64(0), metrics.ElementsPinnedMemoryCache)
	require.Equal(t, uint64(1), metrics.ElementsMemoryCache)
	require.Equal(t, uint64(0), metrics.SizePinnedMemoryCache)
	require.InEpsilon(t, 3700000, metrics.SizeMemoryCache, 0.25)
}

func TestGetPinnedMetrics(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()

	// GetMetrics 1
	metrics, err := GetPinnedMetrics(cache)
	require.NoError(t, err)
	require.Equal(t, &types.PinnedMetrics{PerModule: make([]types.PerModuleEntry, 0)}, metrics)

	// Store contract 1
	wasm, err := os.ReadFile("../../testdata/hackatom.wasm")
	require.NoError(t, err)
	checksum, err := StoreCode(cache, wasm, true)
	require.NoError(t, err)

	err = Pin(cache, checksum)
	require.NoError(t, err)

	// Store contract 2
	cyberpunkWasm, err := os.ReadFile("../../testdata/cyberpunk.wasm")
	require.NoError(t, err)
	cyberpunkChecksum, err := StoreCode(cache, cyberpunkWasm, true)
	require.NoError(t, err)

	err = Pin(cache, cyberpunkChecksum)
	require.NoError(t, err)

	findMetrics := func(list []types.PerModuleEntry, checksum types.Checksum) *types.PerModuleMetrics {
		found := (*types.PerModuleMetrics)(nil)

		for _, structure := range list {
			if bytes.Equal(structure.Checksum, checksum) {
				found = &structure.Metrics
				break
			}
		}

		return found
	}

	// GetMetrics 2
	metrics, err = GetPinnedMetrics(cache)
	require.NoError(t, err)
	require.Len(t, metrics.PerModule, 2)

	hackatomMetrics := findMetrics(metrics.PerModule, checksum)
	cyberpunkMetrics := findMetrics(metrics.PerModule, cyberpunkChecksum)

	require.Equal(t, uint32(0), hackatomMetrics.Hits)
	require.NotEqual(t, uint32(0), hackatomMetrics.Size)
	require.Equal(t, uint32(0), cyberpunkMetrics.Hits)
	require.NotEqual(t, uint32(0), cyberpunkMetrics.Size)

	// Instantiate 1
	gasMeter := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter := types.GasMeter(gasMeter)
	store := NewLookup(gasMeter)
	api := NewMockAPI()
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, types.Array[types.Coin]{types.NewCoin(100, "ATOM")})
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")
	msg1 := []byte(`{"verifier": "fred", "beneficiary": "bob"}`)
	_, _, err = Instantiate(cache, checksum, env, info, msg1, &igasMeter, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)

	// GetMetrics 3
	metrics, err = GetPinnedMetrics(cache)
	require.NoError(t, err)
	require.Len(t, metrics.PerModule, 2)

	hackatomMetrics = findMetrics(metrics.PerModule, checksum)
	cyberpunkMetrics = findMetrics(metrics.PerModule, cyberpunkChecksum)

	require.Equal(t, uint32(1), hackatomMetrics.Hits)
	require.NotEqual(t, uint32(0), hackatomMetrics.Size)
	require.Equal(t, uint32(0), cyberpunkMetrics.Hits)
	require.NotEqual(t, uint32(0), cyberpunkMetrics.Size)
}

func TestInstantiate(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()

	// create contract
	wasm, err := os.ReadFile("../../testdata/hackatom.wasm")
	require.NoError(t, err)
	checksum, err := StoreCode(cache, wasm, true)
	require.NoError(t, err)

	gasMeter := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter := types.GasMeter(gasMeter)
	// instantiate it with this store
	store := NewLookup(gasMeter)
	api := NewMockAPI()
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, types.Array[types.Coin]{types.NewCoin(100, "ATOM")})
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")
	msg := []byte(`{"verifier": "fred", "beneficiary": "bob"}`)

	res, cost, err := Instantiate(cache, checksum, env, info, msg, &igasMeter, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)
	require.Equal(t, uint64(0xb1fe27), cost.UsedInternally)

	var result types.ContractResult
	err = json.Unmarshal(res, &result)
	require.NoError(t, err)
	require.Equal(t, "", result.Err)
	require.Empty(t, result.Ok.Messages)
}

func TestExecute(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()
	checksum := createHackatomContract(t, cache)

	gasMeter1 := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter1 := types.GasMeter(gasMeter1)
	// instantiate it with this store
	store := NewLookup(gasMeter1)
	api := NewMockAPI()
	balance := types.Array[types.Coin]{types.NewCoin(250, "ATOM")}
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, balance)
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")

	msg := []byte(`{"verifier": "fred", "beneficiary": "bob"}`)

	start := time.Now()
	res, cost, err := Instantiate(cache, checksum, env, info, msg, &igasMeter1, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	diff := time.Since(start)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)
	require.Equal(t, uint64(0xb1fe27), cost.UsedInternally)
	t.Logf("Time (%d gas): %s\n", cost.UsedInternally, diff)

	// execute with the same store
	gasMeter2 := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter2 := types.GasMeter(gasMeter2)
	store.SetGasMeter(gasMeter2)
	env = MockEnvBin(t)
	info = MockInfoBin(t, "fred")
	start = time.Now()
	res, cost, err = Execute(cache, checksum, env, info, []byte(`{"release":{}}`), &igasMeter2, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	diff = time.Since(start)
	require.NoError(t, err)
	require.Equal(t, uint64(0x1416da5), cost.UsedInternally)
	t.Logf("Time (%d gas): %s\n", cost.UsedInternally, diff)

	// make sure it read the balance properly and we got 250 atoms
	var result types.ContractResult
	err = json.Unmarshal(res, &result)
	require.NoError(t, err)
	require.Equal(t, "", result.Err)
	require.Len(t, result.Ok.Messages, 1)
	// Ensure we got our custom event
	require.Len(t, result.Ok.Events, 1)
	ev := result.Ok.Events[0]
	require.Equal(t, "hackatom", ev.Type)
	require.Len(t, ev.Attributes, 1)
	require.Equal(t, "action", ev.Attributes[0].Key)
	require.Equal(t, "release", ev.Attributes[0].Value)

	dispatch := result.Ok.Messages[0].Msg
	require.NotNil(t, dispatch.Bank, "%#v", dispatch)
	require.NotNil(t, dispatch.Bank.Send, "%#v", dispatch)
	send := dispatch.Bank.Send
	require.Equal(t, "bob", send.ToAddress)
	require.Equal(t, balance, send.Amount)
	// check the data is properly formatted
	expectedData := []byte{0xF0, 0x0B, 0xAA}
	require.Equal(t, expectedData, result.Ok.Data)
}

func TestExecutePanic(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()
	checksum := createCyberpunkContract(t, cache)

	maxGas := TESTING_GAS_LIMIT
	gasMeter1 := NewMockGasMeter(maxGas)
	igasMeter1 := types.GasMeter(gasMeter1)
	// instantiate it with this store
	store := NewLookup(gasMeter1)
	api := NewMockAPI()
	balance := types.Array[types.Coin]{types.NewCoin(250, "ATOM")}
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, balance)
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")

	res, _, err := Instantiate(cache, checksum, env, info, []byte(`{}`), &igasMeter1, store, api, &querier, maxGas, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)

	// execute a panic
	gasMeter2 := NewMockGasMeter(maxGas)
	igasMeter2 := types.GasMeter(gasMeter2)
	store.SetGasMeter(gasMeter2)
	info = MockInfoBin(t, "fred")
	_, _, err = Execute(cache, checksum, env, info, []byte(`{"panic":{}}`), &igasMeter2, store, api, &querier, maxGas, TESTING_PRINT_DEBUG)
	require.ErrorContains(t, err, "RuntimeError: Aborted: panicked at 'This page intentionally faulted'")
}

func TestExecuteUnreachable(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()
	checksum := createCyberpunkContract(t, cache)

	maxGas := TESTING_GAS_LIMIT
	gasMeter1 := NewMockGasMeter(maxGas)
	igasMeter1 := types.GasMeter(gasMeter1)
	// instantiate it with this store
	store := NewLookup(gasMeter1)
	api := NewMockAPI()
	balance := types.Array[types.Coin]{types.NewCoin(250, "ATOM")}
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, balance)
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")

	res, _, err := Instantiate(cache, checksum, env, info, []byte(`{}`), &igasMeter1, store, api, &querier, maxGas, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)

	// execute a panic
	gasMeter2 := NewMockGasMeter(maxGas)
	igasMeter2 := types.GasMeter(gasMeter2)
	store.SetGasMeter(gasMeter2)
	info = MockInfoBin(t, "fred")
	_, _, err = Execute(cache, checksum, env, info, []byte(`{"unreachable":{}}`), &igasMeter2, store, api, &querier, maxGas, TESTING_PRINT_DEBUG)
	require.ErrorContains(t, err, "RuntimeError: unreachable")
}

func TestExecuteCpuLoop(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()
	checksum := createCyberpunkContract(t, cache)

	gasMeter1 := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter1 := types.GasMeter(gasMeter1)
	// instantiate it with this store
	store := NewLookup(gasMeter1)
	api := NewMockAPI()
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, nil)
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")

	msg := []byte(`{}`)

	start := time.Now()
	res, cost, err := Instantiate(cache, checksum, env, info, msg, &igasMeter1, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	diff := time.Since(start)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)
	require.Equal(t, uint64(0x79f527), cost.UsedInternally)
	t.Logf("Time (%d gas): %s\n", cost.UsedInternally, diff)

	// execute a cpu loop
	maxGas := uint64(40_000_000)
	gasMeter2 := NewMockGasMeter(maxGas)
	igasMeter2 := types.GasMeter(gasMeter2)
	store.SetGasMeter(gasMeter2)
	info = MockInfoBin(t, "fred")
	start = time.Now()
	_, cost, err = Execute(cache, checksum, env, info, []byte(`{"cpu_loop":{}}`), &igasMeter2, store, api, &querier, maxGas, TESTING_PRINT_DEBUG)
	diff = time.Since(start)
	require.Error(t, err)
	require.Equal(t, cost.UsedInternally, maxGas)
	t.Logf("CPULoop Time (%d gas): %s\n", cost.UsedInternally, diff)
}

func TestExecuteStorageLoop(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()
	checksum := createCyberpunkContract(t, cache)

	gasMeter1 := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter1 := types.GasMeter(gasMeter1)
	// instantiate it with this store
	store := NewLookup(gasMeter1)
	api := NewMockAPI()
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, nil)
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")

	msg := []byte(`{}`)

	res, _, err := Instantiate(cache, checksum, env, info, msg, &igasMeter1, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)

	// execute a storage loop
	maxGas := uint64(40_000_000)
	gasMeter2 := NewMockGasMeter(maxGas)
	igasMeter2 := types.GasMeter(gasMeter2)
	store.SetGasMeter(gasMeter2)
	info = MockInfoBin(t, "fred")
	start := time.Now()
	_, gasReport, err := Execute(cache, checksum, env, info, []byte(`{"storage_loop":{}}`), &igasMeter2, store, api, &querier, maxGas, TESTING_PRINT_DEBUG)
	diff := time.Since(start)
	require.Error(t, err)
	t.Logf("StorageLoop Time (%d gas): %s\n", gasReport.UsedInternally, diff)
	t.Logf("Gas used: %d\n", gasMeter2.GasConsumed())
	t.Logf("Wasm gas: %d\n", gasReport.UsedInternally)

	// the "sdk gas" * GasMultiplier + the wasm cost should equal the maxGas (or be very close)
	totalCost := gasReport.UsedInternally + gasMeter2.GasConsumed()
	require.Equal(t, int64(maxGas), int64(totalCost))
}

func BenchmarkContractCall(b *testing.B) {
	cache, cleanup := withCache(b)
	defer cleanup()

	checksum := createCyberpunkContract(b, cache)

	gasMeter1 := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter1 := types.GasMeter(gasMeter1)
	// instantiate it with this store
	store := NewLookup(gasMeter1)
	api := NewMockAPI()
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, nil)
	env := MockEnvBin(b)
	info := MockInfoBin(b, "creator")

	msg := []byte(`{}`)

	res, _, err := Instantiate(cache, checksum, env, info, msg, &igasMeter1, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(b, err)
	requireOkResponse(b, res, 0)

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		gasMeter2 := NewMockGasMeter(TESTING_GAS_LIMIT)
		igasMeter2 := types.GasMeter(gasMeter2)
		store.SetGasMeter(gasMeter2)
		info = MockInfoBin(b, "fred")
		msg := []byte(`{"allocate_large_memory":{"pages":0}}`) // replace with noop once we have it
		res, _, err = Execute(cache, checksum, env, info, msg, &igasMeter2, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
		require.NoError(b, err)
		requireOkResponse(b, res, 0)
	}
}

func Benchmark100ConcurrentContractCalls(b *testing.B) {
	cache, cleanup := withCache(b)
	defer cleanup()

	checksum := createCyberpunkContract(b, cache)

	gasMeter1 := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter1 := types.GasMeter(gasMeter1)
	// instantiate it with this store
	store := NewLookup(gasMeter1)
	api := NewMockAPI()
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, nil)
	env := MockEnvBin(b)
	info := MockInfoBin(b, "creator")

	msg := []byte(`{}`)

	res, _, err := Instantiate(cache, checksum, env, info, msg, &igasMeter1, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(b, err)
	requireOkResponse(b, res, 0)

	info = MockInfoBin(b, "fred")

	const callCount = 100 // Calls per benchmark iteration

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		var wg sync.WaitGroup
		errChan := make(chan error, callCount)
		resChan := make(chan []byte, callCount)
		wg.Add(callCount)

		for i := 0; i < callCount; i++ {
			go func() {
				defer wg.Done()
				gasMeter2 := NewMockGasMeter(TESTING_GAS_LIMIT)
				igasMeter2 := types.GasMeter(gasMeter2)
				store.SetGasMeter(gasMeter2)
				msg := []byte(`{"allocate_large_memory":{"pages":0}}`) // replace with noop once we have it
				res, _, err = Execute(cache, checksum, env, info, msg, &igasMeter2, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
				errChan <- err
				resChan <- res
			}()
		}
		wg.Wait()
		close(errChan)
		close(resChan)

		// Now check results in the main test goroutine
		for i := 0; i < callCount; i++ {
			require.NoError(b, <-errChan)
			requireOkResponse(b, <-resChan, 0)
		}
	}
}

func TestExecuteUserErrorsInApiCalls(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()
	checksum := createHackatomContract(t, cache)

	maxGas := TESTING_GAS_LIMIT
	gasMeter1 := NewMockGasMeter(maxGas)
	igasMeter1 := types.GasMeter(gasMeter1)
	// instantiate it with this store
	store := NewLookup(gasMeter1)
	balance := types.Array[types.Coin]{types.NewCoin(250, "ATOM")}
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, balance)
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")

	defaultApi := NewMockAPI()
	msg := []byte(`{"verifier": "fred", "beneficiary": "bob"}`)
	res, _, err := Instantiate(cache, checksum, env, info, msg, &igasMeter1, store, defaultApi, &querier, maxGas, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)

	gasMeter2 := NewMockGasMeter(maxGas)
	igasMeter2 := types.GasMeter(gasMeter2)
	store.SetGasMeter(gasMeter2)
	info = MockInfoBin(t, "fred")
	failingApi := NewMockFailureAPI()
	res, _, err = Execute(cache, checksum, env, info, []byte(`{"user_errors_in_api_calls":{}}`), &igasMeter2, store, failingApi, &querier, maxGas, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)
}

func TestMigrate(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()
	checksum := createHackatomContract(t, cache)

	gasMeter := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter := types.GasMeter(gasMeter)
	// instantiate it with this store
	store := NewLookup(gasMeter)
	api := NewMockAPI()
	balance := types.Array[types.Coin]{types.NewCoin(250, "ATOM")}
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, balance)
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")
	msg := []byte(`{"verifier": "fred", "beneficiary": "bob"}`)

	res, _, err := Instantiate(cache, checksum, env, info, msg, &igasMeter, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)

	// verifier is fred
	query := []byte(`{"verifier":{}}`)
	data, _, err := Query(cache, checksum, env, query, &igasMeter, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	var qResult types.QueryResult
	err = json.Unmarshal(data, &qResult)
	require.NoError(t, err)
	require.Equal(t, "", qResult.Err)
	require.JSONEq(t, `{"verifier":"fred"}`, string(qResult.Ok))

	// migrate to a new verifier - alice
	// we use the same code blob as we are testing hackatom self-migration
	_, _, err = Migrate(cache, checksum, env, []byte(`{"verifier":"alice"}`), &igasMeter, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)

	// should update verifier to alice
	data, _, err = Query(cache, checksum, env, query, &igasMeter, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	var qResult2 types.QueryResult
	err = json.Unmarshal(data, &qResult2)
	require.NoError(t, err)
	require.Equal(t, "", qResult2.Err)
	require.JSONEq(t, `{"verifier":"alice"}`, string(qResult2.Ok))
}

func TestMultipleInstances(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()
	checksum := createHackatomContract(t, cache)

	// instance1 controlled by fred
	gasMeter1 := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter1 := types.GasMeter(gasMeter1)
	store1 := NewLookup(gasMeter1)
	api := NewMockAPI()
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, types.Array[types.Coin]{types.NewCoin(100, "ATOM")})
	env := MockEnvBin(t)
	info := MockInfoBin(t, "regen")
	msg := []byte(`{"verifier": "fred", "beneficiary": "bob"}`)
	res, cost, err := Instantiate(cache, checksum, env, info, msg, &igasMeter1, store1, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)
	// we now count wasm gas charges and db writes
	assert.Equal(t, uint64(0xb0c2cd), cost.UsedInternally)

	// instance2 controlled by mary
	gasMeter2 := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter2 := types.GasMeter(gasMeter2)
	store2 := NewLookup(gasMeter2)
	info = MockInfoBin(t, "chrous")
	msg = []byte(`{"verifier": "mary", "beneficiary": "sue"}`)
	res, cost, err = Instantiate(cache, checksum, env, info, msg, &igasMeter2, store2, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)
	assert.Equal(t, uint64(0xb1760a), cost.UsedInternally)

	// fail to execute store1 with mary
	resp := exec(t, cache, checksum, "mary", store1, api, querier, 0xa7c5ce)
	require.Equal(t, "Unauthorized", resp.Err)

	// succeed to execute store1 with fred
	resp = exec(t, cache, checksum, "fred", store1, api, querier, 0x140e8ad)
	require.Equal(t, "", resp.Err)
	require.Len(t, resp.Ok.Messages, 1)
	attributes := resp.Ok.Attributes
	require.Len(t, attributes, 2)
	require.Equal(t, "destination", attributes[1].Key)
	require.Equal(t, "bob", attributes[1].Value)

	// succeed to execute store2 with mary
	resp = exec(t, cache, checksum, "mary", store2, api, querier, 0x1412b29)
	require.Equal(t, "", resp.Err)
	require.Len(t, resp.Ok.Messages, 1)
	attributes = resp.Ok.Attributes
	require.Len(t, attributes, 2)
	require.Equal(t, "destination", attributes[1].Key)
	require.Equal(t, "sue", attributes[1].Value)
}

func TestSudo(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()
	checksum := createHackatomContract(t, cache)

	gasMeter1 := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter1 := types.GasMeter(gasMeter1)
	// instantiate it with this store
	store := NewLookup(gasMeter1)
	api := NewMockAPI()
	balance := types.Array[types.Coin]{types.NewCoin(250, "ATOM")}
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, balance)
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")

	msg := []byte(`{"verifier": "fred", "beneficiary": "bob"}`)
	res, _, err := Instantiate(cache, checksum, env, info, msg, &igasMeter1, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)

	// call sudo with same store
	gasMeter2 := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter2 := types.GasMeter(gasMeter2)
	store.SetGasMeter(gasMeter2)
	env = MockEnvBin(t)
	msg = []byte(`{"steal_funds":{"recipient":"community-pool","amount":[{"amount":"700","denom":"gold"}]}}`)
	res, _, err = Sudo(cache, checksum, env, msg, &igasMeter2, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)

	// make sure it blindly followed orders
	var result types.ContractResult
	err = json.Unmarshal(res, &result)
	require.NoError(t, err)
	require.Equal(t, "", result.Err)
	require.Len(t, result.Ok.Messages, 1)
	dispatch := result.Ok.Messages[0].Msg
	require.NotNil(t, dispatch.Bank, "%#v", dispatch)
	require.NotNil(t, dispatch.Bank.Send, "%#v", dispatch)
	send := dispatch.Bank.Send
	assert.Equal(t, "community-pool", send.ToAddress)
	expectedPayout := types.Array[types.Coin]{types.NewCoin(700, "gold")}
	assert.Equal(t, expectedPayout, send.Amount)
}

func TestDispatchSubmessage(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()
	checksum := createReflectContract(t, cache)

	gasMeter1 := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter1 := types.GasMeter(gasMeter1)
	// instantiate it with this store
	store := NewLookup(gasMeter1)
	api := NewMockAPI()
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, nil)
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")

	msg := []byte(`{}`)
	res, _, err := Instantiate(cache, checksum, env, info, msg, &igasMeter1, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)

	// dispatch a submessage
	var id uint64 = 1234
	payload := types.SubMsg{
		ID: id,
		Msg: types.CosmosMsg{Bank: &types.BankMsg{Send: &types.SendMsg{
			ToAddress: "friend",
			Amount:    types.Array[types.Coin]{types.NewCoin(1, "token")},
		}}},
		ReplyOn: types.ReplyAlways,
	}
	payloadBin, err := json.Marshal(payload)
	require.NoError(t, err)
	payloadMsg := []byte(fmt.Sprintf(`{"reflect_sub_msg":{"msgs":[%s]}}`, string(payloadBin)))

	gasMeter2 := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter2 := types.GasMeter(gasMeter2)
	store.SetGasMeter(gasMeter2)
	env = MockEnvBin(t)
	res, _, err = Execute(cache, checksum, env, info, payloadMsg, &igasMeter2, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)

	// make sure it blindly followed orders
	var result types.ContractResult
	err = json.Unmarshal(res, &result)
	require.NoError(t, err)
	require.Equal(t, "", result.Err)
	require.Len(t, result.Ok.Messages, 1)
	dispatch := result.Ok.Messages[0]
	assert.Equal(t, id, dispatch.ID)
	assert.Equal(t, payload.Msg, dispatch.Msg)
	assert.Nil(t, dispatch.GasLimit)
	assert.Equal(t, payload.ReplyOn, dispatch.ReplyOn)
}

func TestReplyAndQuery(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()
	checksum := createReflectContract(t, cache)

	gasMeter1 := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter1 := types.GasMeter(gasMeter1)
	// instantiate it with this store
	store := NewLookup(gasMeter1)
	api := NewMockAPI()
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, nil)
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")

	msg := []byte(`{}`)
	res, _, err := Instantiate(cache, checksum, env, info, msg, &igasMeter1, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)

	var id uint64 = 1234
	data := []byte("foobar")
	events := types.Array[types.Event]{{
		Type: "message",
		Attributes: types.Array[types.EventAttribute]{{
			Key:   "signer",
			Value: "caller-addr",
		}},
	}}
	reply := types.Reply{
		ID: id,
		Result: types.SubMsgResult{
			Ok: &types.SubMsgResponse{
				Events: events,
				Data:   data,
			},
		},
	}
	replyBin, err := json.Marshal(reply)
	require.NoError(t, err)

	gasMeter2 := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter2 := types.GasMeter(gasMeter2)
	store.SetGasMeter(gasMeter2)
	env = MockEnvBin(t)
	res, _, err = Reply(cache, checksum, env, replyBin, &igasMeter2, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)

	// now query the state to see if it stored the data properly
	badQuery := []byte(`{"sub_msg_result":{"id":7777}}`)
	res, _, err = Query(cache, checksum, env, badQuery, &igasMeter2, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	requireQueryError(t, res)

	query := []byte(`{"sub_msg_result":{"id":1234}}`)
	res, _, err = Query(cache, checksum, env, query, &igasMeter2, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	qResult := requireQueryOk(t, res)

	var stored types.Reply
	err = json.Unmarshal(qResult, &stored)
	require.NoError(t, err)
	assert.Equal(t, id, stored.ID)
	require.NotNil(t, stored.Result.Ok)
	val := stored.Result.Ok
	require.Equal(t, data, val.Data)
	require.Equal(t, events, val.Events)
}

func requireOkResponse(tb testing.TB, res []byte, expectedMsgs int) {
	var result types.ContractResult
	err := json.Unmarshal(res, &result)
	require.NoError(tb, err)
	require.Equal(tb, "", result.Err)
	require.Len(tb, result.Ok.Messages, expectedMsgs)
}

func requireQueryError(t *testing.T, res []byte) {
	var result types.QueryResult
	err := json.Unmarshal(res, &result)
	require.NoError(t, err)
	require.Empty(t, result.Ok)
	require.NotEmpty(t, result.Err)
}

func requireQueryOk(t *testing.T, res []byte) []byte {
	var result types.QueryResult
	err := json.Unmarshal(res, &result)
	require.NoError(t, err)
	require.Empty(t, result.Err)
	require.NotEmpty(t, result.Ok)
	return result.Ok
}

func createHackatomContract(t testing.TB, cache Cache) []byte {
	return createContract(t, cache, "../../testdata/hackatom.wasm")
}

func createCyberpunkContract(t testing.TB, cache Cache) []byte {
	return createContract(t, cache, "../../testdata/cyberpunk.wasm")
}

func createQueueContract(t testing.TB, cache Cache) []byte {
	return createContract(t, cache, "../../testdata/queue.wasm")
}

func createReflectContract(t testing.TB, cache Cache) []byte {
	return createContract(t, cache, "../../testdata/reflect.wasm")
}

func createFloaty2(t testing.TB, cache Cache) []byte {
	return createContract(t, cache, "../../testdata/floaty_2.0.wasm")
}

func createContract(t testing.TB, cache Cache, wasmFile string) []byte {
	wasm, err := os.ReadFile(wasmFile)
	require.NoError(t, err)
	checksum, err := StoreCode(cache, wasm, true)
	require.NoError(t, err)
	return checksum
}

// exec runs the handle tx with the given signer
func exec(t *testing.T, cache Cache, checksum []byte, signer types.HumanAddress, store types.KVStore, api *types.GoAPI, querier Querier, gasExpected uint64) types.ContractResult {
	gasMeter := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter := types.GasMeter(gasMeter)
	env := MockEnvBin(t)
	info := MockInfoBin(t, signer)
	res, cost, err := Execute(cache, checksum, env, info, []byte(`{"release":{}}`), &igasMeter, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	assert.Equal(t, gasExpected, cost.UsedInternally)

	var result types.ContractResult
	err = json.Unmarshal(res, &result)
	require.NoError(t, err)
	return result
}

func TestQuery(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()
	checksum := createHackatomContract(t, cache)

	// set up contract
	gasMeter1 := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter1 := types.GasMeter(gasMeter1)
	store := NewLookup(gasMeter1)
	api := NewMockAPI()
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, types.Array[types.Coin]{types.NewCoin(100, "ATOM")})
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")
	msg := []byte(`{"verifier": "fred", "beneficiary": "bob"}`)
	_, _, err := Instantiate(cache, checksum, env, info, msg, &igasMeter1, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)

	// invalid query
	gasMeter2 := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter2 := types.GasMeter(gasMeter2)
	store.SetGasMeter(gasMeter2)
	query := []byte(`{"Raw":{"val":"config"}}`)
	data, _, err := Query(cache, checksum, env, query, &igasMeter2, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	var badResult types.QueryResult
	err = json.Unmarshal(data, &badResult)
	require.NoError(t, err)
	require.Contains(t, badResult.Err, "Error parsing into type hackatom::msg::QueryMsg: unknown variant `Raw`, expected one of")

	// make a valid query
	gasMeter3 := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter3 := types.GasMeter(gasMeter3)
	store.SetGasMeter(gasMeter3)
	query = []byte(`{"verifier":{}}`)
	data, _, err = Query(cache, checksum, env, query, &igasMeter3, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	var qResult types.QueryResult
	err = json.Unmarshal(data, &qResult)
	require.NoError(t, err)
	require.Equal(t, "", qResult.Err)
	require.JSONEq(t, `{"verifier":"fred"}`, string(qResult.Ok))
}

func TestHackatomQuerier(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()
	checksum := createHackatomContract(t, cache)

	// set up contract
	gasMeter := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter := types.GasMeter(gasMeter)
	store := NewLookup(gasMeter)
	api := NewMockAPI()
	initBalance := types.Array[types.Coin]{types.NewCoin(1234, "ATOM"), types.NewCoin(65432, "ETH")}
	querier := DefaultQuerier("foobar", initBalance)

	// make a valid query to the other address
	query := []byte(`{"other_balance":{"address":"foobar"}}`)
	// TODO The query happens before the contract is initialized. How is this legal?
	env := MockEnvBin(t)
	data, _, err := Query(cache, checksum, env, query, &igasMeter, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	var qResult types.QueryResult
	err = json.Unmarshal(data, &qResult)
	require.NoError(t, err)
	require.Equal(t, "", qResult.Err)
	var balances types.AllBalancesResponse
	err = json.Unmarshal(qResult.Ok, &balances)
	require.NoError(t, err)
	require.Equal(t, balances.Amount, initBalance)
}

func TestCustomReflectQuerier(t *testing.T) {
	type CapitalizedQuery struct {
		Text string `json:"text"`
	}

	type QueryMsg struct {
		Capitalized *CapitalizedQuery `json:"capitalized,omitempty"`
		// There are more queries but we don't use them yet
		// https://github.com/CosmWasm/cosmwasm/blob/v0.11.0-alpha3/contracts/reflect/src/msg.rs#L18-L28
	}

	type CapitalizedResponse struct {
		Text string `json:"text"`
	}

	cache, cleanup := withCache(t)
	defer cleanup()
	checksum := createReflectContract(t, cache)

	// set up contract
	gasMeter := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter := types.GasMeter(gasMeter)
	store := NewLookup(gasMeter)
	api := NewMockAPI()
	initBalance := types.Array[types.Coin]{types.NewCoin(1234, "ATOM")}
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, initBalance)
	// we need this to handle the custom requests from the reflect contract
	innerQuerier := querier.(*MockQuerier)
	innerQuerier.Custom = ReflectCustom{}
	querier = Querier(innerQuerier)

	// make a valid query to the other address
	queryMsg := QueryMsg{
		Capitalized: &CapitalizedQuery{
			Text: "small Frys :)",
		},
	}
	query, err := json.Marshal(queryMsg)
	require.NoError(t, err)
	env := MockEnvBin(t)
	data, _, err := Query(cache, checksum, env, query, &igasMeter, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	var qResult types.QueryResult
	err = json.Unmarshal(data, &qResult)
	require.NoError(t, err)
	require.Equal(t, "", qResult.Err)

	var response CapitalizedResponse
	err = json.Unmarshal(qResult.Ok, &response)
	require.NoError(t, err)
	require.Equal(t, "SMALL FRYS :)", response.Text)
}

// TestFloats is a port of the float_instrs_are_deterministic test in cosmwasm-vm
func TestFloats(t *testing.T) {
	type Value struct {
		U32 *uint32 `json:"u32,omitempty"`
		U64 *uint64 `json:"u64,omitempty"`
		F32 *uint32 `json:"f32,omitempty"`
		F64 *uint64 `json:"f64,omitempty"`
	}

	// helper to print the value in the same format as Rust's Debug trait
	debugStr := func(value Value) string {
		if value.U32 != nil {
			return fmt.Sprintf("U32(%d)", *value.U32)
		} else if value.U64 != nil {
			return fmt.Sprintf("U64(%d)", *value.U64)
		} else if value.F32 != nil {
			return fmt.Sprintf("F32(%d)", *value.F32)
		} else if value.F64 != nil {
			return fmt.Sprintf("F64(%d)", *value.F64)
		} else {
			t.FailNow()
			return ""
		}
	}

	cache, cleanup := withCache(t)
	defer cleanup()
	checksum := createFloaty2(t, cache)

	gasMeter := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter := types.GasMeter(gasMeter)
	// instantiate it with this store
	store := NewLookup(gasMeter)
	api := NewMockAPI()
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, nil)
	env := MockEnvBin(t)

	// query instructions
	query := []byte(`{"instructions":{}}`)
	data, _, err := Query(cache, checksum, env, query, &igasMeter, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	var qResult types.QueryResult
	err = json.Unmarshal(data, &qResult)
	require.NoError(t, err)
	require.Empty(t, qResult.Err)
	var instructions []string
	err = json.Unmarshal(qResult.Ok, &instructions)
	require.NoError(t, err)
	// little sanity check
	require.Len(t, instructions, 70)

	hasher := sha256.New()
	const RUNS_PER_INSTRUCTION = 150
	for _, instr := range instructions {
		for seed := 0; seed < RUNS_PER_INSTRUCTION; seed++ {
			// query some input values for the instruction
			msg := fmt.Sprintf(`{"random_args_for":{"instruction":"%s","seed":%d}}`, instr, seed)
			data, _, err = Query(cache, checksum, env, []byte(msg), &igasMeter, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
			require.NoError(t, err)
			err = json.Unmarshal(data, &qResult)
			require.NoError(t, err)
			require.Empty(t, qResult.Err)
			var args []Value
			err = json.Unmarshal(qResult.Ok, &args)
			require.NoError(t, err)

			// build the run message
			argStr, err := json.Marshal(args)
			require.NoError(t, err)
			msg = fmt.Sprintf(`{"run":{"instruction":"%s","args":%s}}`, instr, argStr)

			// run the instruction
			// this might throw a runtime error (e.g. if the instruction traps)
			data, _, err = Query(cache, checksum, env, []byte(msg), &igasMeter, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
			var result string
			if err != nil {
				require.Error(t, err)
				// remove the prefix to make the error message the same as in the cosmwasm-vm test
				result = strings.Replace(err.Error(), "Error calling the VM: Error executing Wasm: ", "", 1)
			} else {
				err = json.Unmarshal(data, &qResult)
				require.NoError(t, err)
				require.Empty(t, qResult.Err)
				var response Value
				err = json.Unmarshal(qResult.Ok, &response)
				require.NoError(t, err)
				result = debugStr(response)
			}
			// add the result to the hash
			hasher.Write([]byte(fmt.Sprintf("%s%d%s", instr, seed, result)))
		}
	}

	hash := hasher.Sum(nil)
	require.Equal(t, "95f70fa6451176ab04a9594417a047a1e4d8e2ff809609b8f81099496bee2393", hex.EncodeToString(hash))
}

```
---
### `link_glibclinux_aarch64.go`
*2025-02-15 10:17:28 | 1 KB*
```go
//go:build linux && !muslc && arm64 && !sys_wasmvm

package api

// #cgo LDFLAGS: -Wl,-rpath,${SRCDIR} -L${SRCDIR} -lwasmvm.aarch64
import "C"

```
---
### `link_glibclinux_x86_64.go`
*2025-02-15 10:17:28 | 1 KB*
```go
//go:build linux && !muslc && amd64 && !sys_wasmvm

package api

// #cgo LDFLAGS: -Wl,-rpath,${SRCDIR} -L${SRCDIR} -lwasmvm.x86_64
import "C"

```
---
### `link_mac.go`
*2025-02-15 10:17:28 | 1 KB*
```go
//go:build darwin && !static_wasm && !sys_wasmvm

package api

// #cgo LDFLAGS: -Wl,-rpath,${SRCDIR} -L${SRCDIR} -lwasmvm
import "C"

```
---
### `link_mac_static.go`
*2025-02-15 10:17:28 | 1 KB*
```go
//go:build darwin && static_wasm && !sys_wasmvm

package api

// #cgo LDFLAGS: -L${SRCDIR} -lwasmvmstatic_darwin
import "C"

```
---
### `link_muslc_aarch64.go`
*2025-02-15 10:17:28 | 1 KB*
```go
//go:build linux && muslc && arm64 && !sys_wasmvm

package api

// #cgo LDFLAGS: -Wl,-rpath,${SRCDIR} -L${SRCDIR} -lwasmvm_muslc.aarch64
import "C"

```
---
### `link_muslc_x86_64.go`
*2025-02-15 10:17:28 | 1 KB*
```go
//go:build linux && muslc && amd64 && !sys_wasmvm

package api

// #cgo LDFLAGS: -Wl,-rpath,${SRCDIR} -L${SRCDIR} -lwasmvm_muslc.x86_64
import "C"

```
---
### `link_system.go`
*2025-02-15 10:17:28 | 1 KB*
```go
//go:build sys_wasmvm

package api

// #cgo LDFLAGS: -lwasmvm
import "C"

```
---
### `link_windows.go`
*2025-02-15 10:17:28 | 1 KB*
```go
//go:build windows && !sys_wasmvm

package api

// #cgo LDFLAGS: -Wl,-rpath,${SRCDIR} -L${SRCDIR} -lwasmvm
import "C"

```
---
### `memory.go`
*2025-02-15 10:17:28 | 4 KB*
```go
package api

/*
#include "bindings.h"
*/
import "C"

import "unsafe"

// makeView creates a view into the given byte slice what allows Rust code to read it.
// The byte slice is managed by Go and will be garbage collected. Use runtime.KeepAlive
// to ensure the byte slice lives long enough.
func makeView(s []byte) C.ByteSliceView {
	if s == nil {
		return C.ByteSliceView{is_nil: true, ptr: cu8_ptr(nil), len: cusize(0)}
	}

	// In Go, accessing the 0-th element of an empty array triggers a panic. That is why in the case
	// of an empty `[]byte` we can't get the internal heap pointer to the underlying array as we do
	// below with `&data[0]`. https://play.golang.org/p/xvDY3g9OqUk
	if len(s) == 0 {
		return C.ByteSliceView{is_nil: false, ptr: cu8_ptr(nil), len: cusize(0)}
	}

	return C.ByteSliceView{
		is_nil: false,
		ptr:    cu8_ptr(unsafe.Pointer(&s[0])),
		len:    cusize(len(s)),
	}
}

// Creates a C.UnmanagedVector, which cannot be done in test files directly
func constructUnmanagedVector(is_none cbool, ptr cu8_ptr, len cusize, cap cusize) C.UnmanagedVector {
	return C.UnmanagedVector{
		is_none: is_none,
		ptr:     ptr,
		len:     len,
		cap:     cap,
	}
}

// uninitializedUnmanagedVector returns an invalid C.UnmanagedVector
// instance. Only use then after someone wrote an instance to it.
func uninitializedUnmanagedVector() C.UnmanagedVector {
	return C.UnmanagedVector{}
}

func newUnmanagedVector(data []byte) C.UnmanagedVector {
	if data == nil {
		return C.new_unmanaged_vector(cbool(true), cu8_ptr(nil), cusize(0))
	} else if len(data) == 0 {
		// in Go, accessing the 0-th element of an empty array triggers a panic. That is why in the case
		// of an empty `[]byte` we can't get the internal heap pointer to the underlying array as we do
		// below with `&data[0]`.
		// https://play.golang.org/p/xvDY3g9OqUk
		return C.new_unmanaged_vector(cbool(false), cu8_ptr(nil), cusize(0))
	} else {
		// This will allocate a proper vector with content and return a description of it
		return C.new_unmanaged_vector(cbool(false), cu8_ptr(unsafe.Pointer(&data[0])), cusize(len(data)))
	}
}

func copyAndDestroyUnmanagedVector(v C.UnmanagedVector) []byte {
	var out []byte
	if v.is_none {
		out = nil
	} else if v.cap == cusize(0) {
		// There is no allocation we can copy
		out = []byte{}
	} else {
		// C.GoBytes create a copy (https://stackoverflow.com/a/40950744/2013738)
		out = C.GoBytes(unsafe.Pointer(v.ptr), cint(v.len))
	}
	C.destroy_unmanaged_vector(v)
	return out
}

func optionalU64ToPtr(val C.OptionalU64) *uint64 {
	if val.is_some {
		return (*uint64)(&val.value)
	}
	return nil
}

// copyU8Slice copies the contents of an Option<&[u8]> that was allocated on the Rust side.
// Returns nil if and only if the source is None.
func copyU8Slice(view C.U8SliceView) []byte {
	if view.is_none {
		return nil
	}
	if view.len == 0 {
		// In this case, we don't want to look into the ptr
		return []byte{}
	}
	// C.GoBytes create a copy (https://stackoverflow.com/a/40950744/2013738)
	res := C.GoBytes(unsafe.Pointer(view.ptr), cint(view.len))
	return res
}

```
---
### `memory_test.go`
*2025-02-15 10:23:27 | 4 KB*
```go
package api

import (
	"sync"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//-------------------------------------
// Example tests for memory bridging
//-------------------------------------

func TestMakeView_TableDriven(t *testing.T) {
	type testCase struct {
		name     string
		input    []byte
		expIsNil bool
		expLen   cusize
	}

	tests := []testCase{
		{
			name:     "Non-empty byte slice",
			input:    []byte{0xaa, 0xbb, 0x64},
			expIsNil: false,
			expLen:   3,
		},
		{
			name:     "Empty slice",
			input:    []byte{},
			expIsNil: false,
			expLen:   0,
		},
		{
			name:     "Nil slice",
			input:    nil,
			expIsNil: true,
			expLen:   0,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			view := makeView(tc.input)
			require.Equal(t, cbool(tc.expIsNil), view.is_nil,
				"Mismatch in is_nil for test: %s", tc.name)
			require.Equal(t, tc.expLen, view.len,
				"Mismatch in len for test: %s", tc.name)
		})
	}
}

func TestCreateAndDestroyUnmanagedVector_TableDriven(t *testing.T) {
	// Helper for the round-trip test
	checkUnmanagedRoundTrip := func(t *testing.T, input []byte, expectNone bool) {
		unmanaged := newUnmanagedVector(input)
		require.Equal(t, cbool(expectNone), unmanaged.is_none,
			"Mismatch on is_none with input: %v", input)

		if !expectNone && len(input) > 0 {
			require.Equal(t, len(input), int(unmanaged.len),
				"Length mismatch for input: %v", input)
			require.GreaterOrEqual(t, int(unmanaged.cap), int(unmanaged.len),
				"Expected cap >= len for input: %v", input)
		}

		copyData := copyAndDestroyUnmanagedVector(unmanaged)
		require.Equal(t, input, copyData,
			"Round-trip mismatch for input: %v", input)
	}

	type testCase struct {
		name       string
		input      []byte
		expectNone bool
	}

	tests := []testCase{
		{
			name:       "Non-empty data",
			input:      []byte{0xaa, 0xbb, 0x64},
			expectNone: false,
		},
		{
			name:       "Empty but non-nil",
			input:      []byte{},
			expectNone: false,
		},
		{
			name:       "Nil => none",
			input:      nil,
			expectNone: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			checkUnmanagedRoundTrip(t, tc.input, tc.expectNone)
		})
	}
}

func TestCopyDestroyUnmanagedVector_SpecificEdgeCases(t *testing.T) {
	t.Run("is_none = true ignoring ptr/len/cap", func(t *testing.T) {
		invalidPtr := unsafe.Pointer(uintptr(42))
		uv := constructUnmanagedVector(cbool(true), cu8_ptr(invalidPtr), cusize(0xBB), cusize(0xAA))
		copy := copyAndDestroyUnmanagedVector(uv)
		require.Nil(t, copy, "copy should be nil if is_none=true")
	})

	t.Run("cap=0 => no allocation => empty data", func(t *testing.T) {
		invalidPtr := unsafe.Pointer(uintptr(42))
		uv := constructUnmanagedVector(cbool(false), cu8_ptr(invalidPtr), cusize(0), cusize(0))
		copy := copyAndDestroyUnmanagedVector(uv)
		require.Equal(t, []byte{}, copy,
			"expected empty result if cap=0 and is_none=false")
	})
}

func TestCopyDestroyUnmanagedVector_Concurrent(t *testing.T) {
	inputs := [][]byte{
		{1, 2, 3},
		{},
		nil,
		{0xff, 0x00, 0x12, 0xab, 0xcd, 0xef},
	}

	var wg sync.WaitGroup
	concurrency := 10

	for i := 0; i < concurrency; i++ {
		for _, data := range inputs {
			data := data
			wg.Add(1)
			go func() {
				defer wg.Done()
				uv := newUnmanagedVector(data)
				out := copyAndDestroyUnmanagedVector(uv)
				assert.Equal(t, data, out,
					"Mismatch in concurrency test for input=%v", data)
			}()
		}
	}
	wg.Wait()
}

```
---
### `mock_failure.go`
*2024-12-19 16:14:31 | 1 KB*
```go
package api

import (
	"fmt"

	"github.com/CosmWasm/wasmvm/v2/types"
)

/***** Mock types.GoAPI ****/

func MockFailureCanonicalizeAddress(human string) ([]byte, uint64, error) {
	return nil, 0, fmt.Errorf("mock failure - canonical_address")
}

func MockFailureHumanizeAddress(canon []byte) (string, uint64, error) {
	return "", 0, fmt.Errorf("mock failure - human_address")
}

func MockFailureValidateAddress(human string) (uint64, error) {
	return 0, fmt.Errorf("mock failure - validate_address")
}

func NewMockFailureAPI() *types.GoAPI {
	return &types.GoAPI{
		HumanizeAddress:     MockFailureHumanizeAddress,
		CanonicalizeAddress: MockFailureCanonicalizeAddress,
		ValidateAddress:     MockFailureValidateAddress,
	}
}

```
---
### `mocks.go`
*2025-02-15 10:19:56 | 16 KB*
```go
package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/CosmWasm/wasmvm/v2/internal/api/testdb"
	"github.com/CosmWasm/wasmvm/v2/types"
)

/** helper constructors **/

const MOCK_CONTRACT_ADDR = "contract"

func MockEnv() types.Env {
	return types.Env{
		Block: types.BlockInfo{
			Height:  123,
			Time:    1578939743_987654321,
			ChainID: "foobar",
		},
		Transaction: &types.TransactionInfo{
			Index: 4,
		},
		Contract: types.ContractInfo{
			Address: MOCK_CONTRACT_ADDR,
		},
	}
}

func MockEnvBin(t testing.TB) []byte {
	bin, err := json.Marshal(MockEnv())
	require.NoError(t, err)
	return bin
}

func MockInfo(sender types.HumanAddress, funds []types.Coin) types.MessageInfo {
	return types.MessageInfo{
		Sender: sender,
		Funds:  funds,
	}
}

func MockInfoWithFunds(sender types.HumanAddress) types.MessageInfo {
	return MockInfo(sender, []types.Coin{{
		Denom:  "ATOM",
		Amount: "100",
	}})
}

func MockInfoBin(t testing.TB, sender types.HumanAddress) []byte {
	bin, err := json.Marshal(MockInfoWithFunds(sender))
	require.NoError(t, err)
	return bin
}

func MockIBCChannel(channelID string, ordering types.IBCOrder, ibcVersion string) types.IBCChannel {
	return types.IBCChannel{
		Endpoint: types.IBCEndpoint{
			PortID:    "my_port",
			ChannelID: channelID,
		},
		CounterpartyEndpoint: types.IBCEndpoint{
			PortID:    "their_port",
			ChannelID: "channel-7",
		},
		Order:        ordering,
		Version:      ibcVersion,
		ConnectionID: "connection-3",
	}
}

func MockIBCChannelOpenInit(channelID string, ordering types.IBCOrder, ibcVersion string) types.IBCChannelOpenMsg {
	return types.IBCChannelOpenMsg{
		OpenInit: &types.IBCOpenInit{
			Channel: MockIBCChannel(channelID, ordering, ibcVersion),
		},
		OpenTry: nil,
	}
}

func MockIBCChannelOpenTry(channelID string, ordering types.IBCOrder, ibcVersion string) types.IBCChannelOpenMsg {
	return types.IBCChannelOpenMsg{
		OpenInit: nil,
		OpenTry: &types.IBCOpenTry{
			Channel:             MockIBCChannel(channelID, ordering, ibcVersion),
			CounterpartyVersion: ibcVersion,
		},
	}
}

func MockIBCChannelConnectAck(channelID string, ordering types.IBCOrder, ibcVersion string) types.IBCChannelConnectMsg {
	return types.IBCChannelConnectMsg{
		OpenAck: &types.IBCOpenAck{
			Channel:             MockIBCChannel(channelID, ordering, ibcVersion),
			CounterpartyVersion: ibcVersion,
		},
		OpenConfirm: nil,
	}
}

func MockIBCChannelConnectConfirm(channelID string, ordering types.IBCOrder, ibcVersion string) types.IBCChannelConnectMsg {
	return types.IBCChannelConnectMsg{
		OpenAck: nil,
		OpenConfirm: &types.IBCOpenConfirm{
			Channel: MockIBCChannel(channelID, ordering, ibcVersion),
		},
	}
}

func MockIBCChannelCloseInit(channelID string, ordering types.IBCOrder, ibcVersion string) types.IBCChannelCloseMsg {
	return types.IBCChannelCloseMsg{
		CloseInit: &types.IBCCloseInit{
			Channel: MockIBCChannel(channelID, ordering, ibcVersion),
		},
		CloseConfirm: nil,
	}
}

func MockIBCChannelCloseConfirm(channelID string, ordering types.IBCOrder, ibcVersion string) types.IBCChannelCloseMsg {
	return types.IBCChannelCloseMsg{
		CloseInit: nil,
		CloseConfirm: &types.IBCCloseConfirm{
			Channel: MockIBCChannel(channelID, ordering, ibcVersion),
		},
	}
}

func MockIBCPacket(myChannel string, data []byte) types.IBCPacket {
	return types.IBCPacket{
		Data: data,
		Src: types.IBCEndpoint{
			PortID:    "their_port",
			ChannelID: "channel-7",
		},
		Dest: types.IBCEndpoint{
			PortID:    "my_port",
			ChannelID: myChannel,
		},
		Sequence: 15,
		Timeout: types.IBCTimeout{
			Block: &types.IBCTimeoutBlock{
				Revision: 1,
				Height:   123456,
			},
		},
	}
}

func MockIBCPacketReceive(myChannel string, data []byte) types.IBCPacketReceiveMsg {
	return types.IBCPacketReceiveMsg{
		Packet: MockIBCPacket(myChannel, data),
	}
}

func MockIBCPacketAck(myChannel string, data []byte, ack types.IBCAcknowledgement) types.IBCPacketAckMsg {
	packet := MockIBCPacket(myChannel, data)

	return types.IBCPacketAckMsg{
		Acknowledgement: ack,
		OriginalPacket:  packet,
	}
}

func MockIBCPacketTimeout(myChannel string, data []byte) types.IBCPacketTimeoutMsg {
	packet := MockIBCPacket(myChannel, data)

	return types.IBCPacketTimeoutMsg{
		Packet: packet,
	}
}

/*** Mock GasMeter ****/
// This code is borrowed from Cosmos-SDK store/types/gas.go

// ErrorOutOfGas defines an error thrown when an action results in out of gas.
type ErrorOutOfGas struct {
	Descriptor string
}

// ErrorGasOverflow defines an error thrown when an action results gas consumption
// unsigned integer overflow.
type ErrorGasOverflow struct {
	Descriptor string
}

type MockGasMeter interface {
	types.GasMeter
	ConsumeGas(amount types.Gas, descriptor string)
}

type mockGasMeter struct {
	limit    types.Gas
	consumed types.Gas
}

// NewMockGasMeter returns a reference to a new mockGasMeter.
func NewMockGasMeter(limit types.Gas) MockGasMeter {
	return &mockGasMeter{
		limit:    limit,
		consumed: 0,
	}
}

func (g *mockGasMeter) GasConsumed() types.Gas {
	return g.consumed
}

func (g *mockGasMeter) Limit() types.Gas {
	return g.limit
}

// addUint64Overflow performs the addition operation on two uint64 integers and
// returns a boolean on whether or not the result overflows.
func addUint64Overflow(a, b uint64) (uint64, bool) {
	if math.MaxUint64-a < b {
		return 0, true
	}

	return a + b, false
}

func (g *mockGasMeter) ConsumeGas(amount types.Gas, descriptor string) {
	var overflow bool
	// TODO: Should we set the consumed field after overflow checking?
	g.consumed, overflow = addUint64Overflow(g.consumed, amount)
	if overflow {
		panic(ErrorGasOverflow{descriptor})
	}

	if g.consumed > g.limit {
		panic(ErrorOutOfGas{descriptor})
	}
}

/*** Mock types.KVStore ****/
// Much of this code is borrowed from Cosmos-SDK store/transient.go

// Note: these gas prices are all in *wasmer gas* and (sdk gas * 100)
//
// We making simple values and non-clear multiples so it is easy to see their impact in test output
// Also note we do not charge for each read on an iterator (out of simplicity and not needed for tests)
const (
	GetPrice    uint64 = 99000
	SetPrice    uint64 = 187000
	RemovePrice uint64 = 142000
	RangePrice  uint64 = 261000
)

type Lookup struct {
	db    *testdb.MemDB
	meter MockGasMeter
}

func NewLookup(meter MockGasMeter) *Lookup {
	return &Lookup{
		db:    testdb.NewMemDB(),
		meter: meter,
	}
}

func (l *Lookup) SetGasMeter(meter MockGasMeter) {
	l.meter = meter
}

func (l *Lookup) WithGasMeter(meter MockGasMeter) *Lookup {
	return &Lookup{
		db:    l.db,
		meter: meter,
	}
}

// Get wraps the underlying DB's Get method panicking on error.
func (l Lookup) Get(key []byte) []byte {
	l.meter.ConsumeGas(GetPrice, "get")
	v := l.db.Get(key)
	if v == nil {
		panic(testdb.ErrKeyEmpty)
	}

	return v
}

// Set wraps the underlying DB's Set method panicking on error.
func (l Lookup) Set(key, value []byte) {
	l.meter.ConsumeGas(SetPrice, "set")
	l.db.Set(key, value) // No `if err := ...` capture, because Set doesn't return an error
}

// Delete wraps the underlying DB's Delete method panicking on error.
// note: Delete doesn't return an error, according to the kvstore implementation in types/store.go
func (l Lookup) Delete(key []byte) {
	l.meter.ConsumeGas(RemovePrice, "remove")
	l.db.Delete(key)
}

// Iterator wraps the underlying DB's Iterator method panicking on error.
func (l Lookup) Iterator(start, end []byte) types.Iterator {
	l.meter.ConsumeGas(RangePrice, "range")
	iter := l.db.Iterator(start, end) // returns only one value
	// no err to handle
	// no need to close
	return iter
}

// ReverseIterator wraps the underlying DB's ReverseIterator method panicking on error.
func (l Lookup) ReverseIterator(start, end []byte) types.Iterator {
	l.meter.ConsumeGas(RangePrice, "range")
	iter := l.db.ReverseIterator(start, end) // returns only one value
	// no err to handle
	// no need to close
	return iter
}

var _ types.KVStore = (*Lookup)(nil)

/***** Mock types.GoAPI ****/

const CanonicalLength = 32

const (
	CostCanonical uint64 = 440
	CostHuman     uint64 = 550
)

func MockCanonicalizeAddress(human string) ([]byte, uint64, error) {
	if len(human) > CanonicalLength {
		return nil, 0, fmt.Errorf("human encoding too long")
	}
	res := make([]byte, CanonicalLength)
	copy(res, []byte(human))
	return res, CostCanonical, nil
}

func MockHumanizeAddress(canon []byte) (string, uint64, error) {
	if len(canon) != CanonicalLength {
		return "", 0, fmt.Errorf("wrong canonical length")
	}
	cut := CanonicalLength
	for i, v := range canon {
		if v == 0 {
			cut = i
			break
		}
	}
	human := string(canon[:cut])
	return human, CostHuman, nil
}

func MockValidateAddress(input string) (gasCost uint64, _ error) {
	canonicalized, gasCostCanonicalize, err := MockCanonicalizeAddress(input)
	gasCost += gasCostCanonicalize
	if err != nil {
		return gasCost, err
	}
	humanized, gasCostHumanize, err := MockHumanizeAddress(canonicalized)
	gasCost += gasCostHumanize
	if err != nil {
		return gasCost, err
	}
	if humanized != strings.ToLower(input) {
		return gasCost, fmt.Errorf("address validation failed")
	}

	return gasCost, nil
}

func NewMockAPI() *types.GoAPI {
	return &types.GoAPI{
		HumanizeAddress:     MockHumanizeAddress,
		CanonicalizeAddress: MockCanonicalizeAddress,
		ValidateAddress:     MockValidateAddress,
	}
}

func TestMockApi(t *testing.T) {
	human := "foobar"
	canon, cost, err := MockCanonicalizeAddress(human)
	require.NoError(t, err)
	require.Len(t, canon, CanonicalLength)
	require.Equal(t, CostCanonical, cost)

	recover, cost, err := MockHumanizeAddress(canon)
	require.NoError(t, err)
	require.Equal(t, recover, human)
	require.Equal(t, CostHuman, cost)
}

/**** MockQuerier ****/

const DEFAULT_QUERIER_GAS_LIMIT = 1_000_000

type MockQuerier struct {
	Bank    BankQuerier
	Custom  CustomQuerier
	usedGas uint64
}

var _ types.Querier = &MockQuerier{}

func DefaultQuerier(contractAddr string, coins types.Array[types.Coin]) types.Querier {
	balances := map[string]types.Array[types.Coin]{
		contractAddr: coins,
	}
	return &MockQuerier{
		Bank:    NewBankQuerier(balances),
		Custom:  NoCustom{},
		usedGas: 0,
	}
}

func (q *MockQuerier) Query(request types.QueryRequest, _gasLimit uint64) ([]byte, error) {
	marshaled, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	q.usedGas += uint64(len(marshaled))
	if request.Bank != nil {
		return q.Bank.Query(request.Bank)
	}
	if request.Custom != nil {
		return q.Custom.Query(request.Custom)
	}
	if request.Staking != nil {
		return nil, types.UnsupportedRequest{Kind: "staking"}
	}
	if request.Wasm != nil {
		return nil, types.UnsupportedRequest{Kind: "wasm"}
	}
	return nil, types.Unknown{}
}

func (q MockQuerier) GasConsumed() uint64 {
	return q.usedGas
}

type BankQuerier struct {
	Balances map[string]types.Array[types.Coin]
}

func NewBankQuerier(balances map[string]types.Array[types.Coin]) BankQuerier {
	bal := make(map[string]types.Array[types.Coin], len(balances))
	for k, v := range balances {
		dst := make([]types.Coin, len(v))
		copy(dst, v)
		bal[k] = dst
	}
	return BankQuerier{
		Balances: bal,
	}
}

func (q BankQuerier) Query(request *types.BankQuery) ([]byte, error) {
	if request.Balance != nil {
		denom := request.Balance.Denom
		coin := types.NewCoin(0, denom)
		for _, c := range q.Balances[request.Balance.Address] {
			if c.Denom == denom {
				coin = c
			}
		}
		resp := types.BalanceResponse{
			Amount: coin,
		}
		return json.Marshal(resp)
	}
	if request.AllBalances != nil {
		coins := q.Balances[request.AllBalances.Address]
		resp := types.AllBalancesResponse{
			Amount: coins,
		}
		return json.Marshal(resp)
	}
	return nil, types.UnsupportedRequest{Kind: "Empty BankQuery"}
}

type CustomQuerier interface {
	Query(request json.RawMessage) ([]byte, error)
}

type NoCustom struct{}

var _ CustomQuerier = NoCustom{}

func (q NoCustom) Query(request json.RawMessage) ([]byte, error) {
	return nil, types.UnsupportedRequest{Kind: "custom"}
}

// ReflectCustom fulfills the requirements for testing `reflect` contract
type ReflectCustom struct{}

var _ CustomQuerier = ReflectCustom{}

type CustomQuery struct {
	Ping        *struct{}         `json:"ping,omitempty"`
	Capitalized *CapitalizedQuery `json:"capitalized,omitempty"`
}

type CapitalizedQuery struct {
	Text string `json:"text"`
}

// CustomResponse is the response for all `CustomQuery`s
type CustomResponse struct {
	Msg string `json:"msg"`
}

func (q ReflectCustom) Query(request json.RawMessage) ([]byte, error) {
	var query CustomQuery
	err := json.Unmarshal(request, &query)
	if err != nil {
		return nil, err
	}
	var resp CustomResponse
	if query.Ping != nil {
		resp.Msg = "PONG"
	} else if query.Capitalized != nil {
		resp.Msg = strings.ToUpper(query.Capitalized.Text)
	} else {
		return nil, errors.New("Unsupported query")
	}
	return json.Marshal(resp)
}

//************ test code for mocks *************************//

func TestBankQuerierAllBalances(t *testing.T) {
	addr := "foobar"
	balance := types.Array[types.Coin]{types.NewCoin(12345678, "ATOM"), types.NewCoin(54321, "ETH")}
	q := DefaultQuerier(addr, balance)

	// query existing account
	req := types.QueryRequest{
		Bank: &types.BankQuery{
			AllBalances: &types.AllBalancesQuery{
				Address: addr,
			},
		},
	}
	res, err := q.Query(req, DEFAULT_QUERIER_GAS_LIMIT)
	require.NoError(t, err)
	var resp types.AllBalancesResponse
	err = json.Unmarshal(res, &resp)
	require.NoError(t, err)
	assert.Equal(t, resp.Amount, balance)

	// query missing account
	req2 := types.QueryRequest{
		Bank: &types.BankQuery{
			AllBalances: &types.AllBalancesQuery{
				Address: "someone-else",
			},
		},
	}
	res, err = q.Query(req2, DEFAULT_QUERIER_GAS_LIMIT)
	require.NoError(t, err)
	var resp2 types.AllBalancesResponse
	err = json.Unmarshal(res, &resp2)
	require.NoError(t, err)
	assert.Nil(t, resp2.Amount)
}

func TestBankQuerierBalance(t *testing.T) {
	addr := "foobar"
	balance := types.Array[types.Coin]{types.NewCoin(12345678, "ATOM"), types.NewCoin(54321, "ETH")}
	q := DefaultQuerier(addr, balance)

	// query existing account with matching denom
	req := types.QueryRequest{
		Bank: &types.BankQuery{
			Balance: &types.BalanceQuery{
				Address: addr,
				Denom:   "ATOM",
			},
		},
	}
	res, err := q.Query(req, DEFAULT_QUERIER_GAS_LIMIT)
	require.NoError(t, err)
	var resp types.BalanceResponse
	err = json.Unmarshal(res, &resp)
	require.NoError(t, err)
	assert.Equal(t, resp.Amount, types.NewCoin(12345678, "ATOM"))

	// query existing account with missing denom
	req2 := types.QueryRequest{
		Bank: &types.BankQuery{
			Balance: &types.BalanceQuery{
				Address: addr,
				Denom:   "BTC",
			},
		},
	}
	res, err = q.Query(req2, DEFAULT_QUERIER_GAS_LIMIT)
	require.NoError(t, err)
	var resp2 types.BalanceResponse
	err = json.Unmarshal(res, &resp2)
	require.NoError(t, err)
	assert.Equal(t, resp2.Amount, types.NewCoin(0, "BTC"))

	// query missing account
	req3 := types.QueryRequest{
		Bank: &types.BankQuery{
			Balance: &types.BalanceQuery{
				Address: "someone-else",
				Denom:   "ATOM",
			},
		},
	}
	res, err = q.Query(req3, DEFAULT_QUERIER_GAS_LIMIT)
	require.NoError(t, err)
	var resp3 types.BalanceResponse
	err = json.Unmarshal(res, &resp3)
	require.NoError(t, err)
	assert.Equal(t, resp3.Amount, types.NewCoin(0, "ATOM"))
}

func TestReflectCustomQuerier(t *testing.T) {
	q := ReflectCustom{}

	// try ping
	msg, err := json.Marshal(CustomQuery{Ping: &struct{}{}})
	require.NoError(t, err)
	bz, err := q.Query(msg)
	require.NoError(t, err)
	var resp CustomResponse
	err = json.Unmarshal(bz, &resp)
	require.NoError(t, err)
	assert.Equal(t, "PONG", resp.Msg)

	// try capital
	msg2, err := json.Marshal(CustomQuery{Capitalized: &CapitalizedQuery{Text: "small."}})
	require.NoError(t, err)
	bz, err = q.Query(msg2)
	require.NoError(t, err)
	var resp2 CustomResponse
	err = json.Unmarshal(bz, &resp2)
	require.NoError(t, err)
	assert.Equal(t, "SMALL.", resp2.Msg)
}

```
---
### `testdb/README.md`
*2024-12-19 16:14:31 | 1 KB*
```markdown
# Testdb
This package contains an in memory DB for testing purpose only. The original code was copied from
https://github.com/tendermint/tm-db/tree/v0.6.7 to decouple project dependencies.

All credits and a big thank you go to the original authors!

```
---
### `testdb/memdb.go`
*2025-02-15 10:17:28 | 5 KB*
```go
package testdb

import (
	"bytes"
	"fmt"
	"sync"

	"github.com/google/btree"
)

const (
	// The approximate number of items and children per B-tree node. Tuned with benchmarks.
	bTreeDegree = 32
)

// item is a btree.Item with byte slices as keys and values
type item struct {
	key   []byte
	value []byte
}

// Less implements btree.Item.
func (i *item) Less(other btree.Item) bool {
	// this considers nil == []byte{}, but that's ok since we handle nil endpoints
	// in iterators specially anyway
	return bytes.Compare(i.key, other.(*item).key) == -1
}

// newKey creates a new key item.
func newKey(key []byte) *item {
	return &item{key: key}
}

// newPair creates a new pair item.
func newPair(key, value []byte) *item {
	return &item{key: key, value: value}
}

// MemDB is an in-memory database backend using a B-tree for storage.
//
// For performance reasons, all given and returned keys and values are pointers to the in-memory
// database, so modifying them will cause the stored values to be modified as well. All DB methods
// already specify that keys and values should be considered read-only, but this is especially
// important with MemDB.
type MemDB struct {
	mtx   sync.RWMutex
	btree *btree.BTree
}

// NewMemDB creates a new in-memory database.
func NewMemDB() *MemDB {
	database := &MemDB{
		btree: btree.New(bTreeDegree),
	}
	return database
}

// Get implements DB.
func (db *MemDB) Get(key []byte) []byte {
	if len(key) == 0 {
		panic(ErrKeyEmpty)
	}
	db.mtx.RLock()
	defer db.mtx.RUnlock()

	i := db.btree.Get(newKey(key))
	if i != nil {
		return i.(*item).value
	}
	return nil
}

// Has implements DB.
func (db *MemDB) Has(key []byte) (bool, error) {
	if len(key) == 0 {
		return false, ErrKeyEmpty
	}
	db.mtx.RLock()
	defer db.mtx.RUnlock()

	return db.btree.Has(newKey(key)), nil
}

// Set implements DB.
func (db *MemDB) Set(key []byte, value []byte) {
	if len(key) == 0 {
		panic(ErrKeyEmpty)
	}
	if value == nil {
		panic(ErrValueNil)
	}
	db.mtx.Lock()
	defer db.mtx.Unlock()

	db.set(key, value)
}

// set sets a value without locking the mutex.
func (db *MemDB) set(key []byte, value []byte) {
	db.btree.ReplaceOrInsert(newPair(key, value))
}

// SetSync implements DB.
func (db *MemDB) SetSync(key []byte, value []byte) {
	db.Set(key, value)
}

// Delete implements DB.
func (db *MemDB) Delete(key []byte) {
	if len(key) == 0 {
		panic(ErrKeyEmpty)
	}
	db.mtx.Lock()
	defer db.mtx.Unlock()

	db.delete(key)
}

// delete deletes a key without locking the mutex.
func (db *MemDB) delete(key []byte) {
	db.btree.Delete(newKey(key))
}

// DeleteSync implements DB.
func (db *MemDB) DeleteSync(key []byte) {
	db.Delete(key)
}

// Close implements DB.
func (db *MemDB) Close() error {
	// Close is a noop since for an in-memory database, we don't have a destination to flush
	// contents to nor do we want any data loss on invoking Close().
	// See the discussion in https://github.com/tendermint/tendermint/libs/pull/56
	return nil
}

// Print implements DB.
func (db *MemDB) Print() error {
	db.mtx.RLock()
	defer db.mtx.RUnlock()

	db.btree.Ascend(func(i btree.Item) bool {
		item := i.(*item)
		fmt.Printf("[%X]:\t[%X]\n", item.key, item.value)
		return true
	})
	return nil
}

// Stats implements DB.
func (db *MemDB) Stats() map[string]string {
	db.mtx.RLock()
	defer db.mtx.RUnlock()

	stats := make(map[string]string)
	stats["database.type"] = "memDB"
	stats["database.size"] = fmt.Sprintf("%d", db.btree.Len())
	return stats
}

// Iterator implements DB.
// Takes out a read-lock on the database until the iterator is closed.
func (db *MemDB) Iterator(start, end []byte) Iterator {
	if (start != nil && len(start) == 0) || (end != nil && len(end) == 0) {
		panic(ErrKeyEmpty)
	}
	return newMemDBIterator(db, start, end, false)
}

// ReverseIterator implements DB.
// Takes out a read-lock on the database until the iterator is closed.
func (db *MemDB) ReverseIterator(start, end []byte) Iterator {
	if (start != nil && len(start) == 0) || (end != nil && len(end) == 0) {
		panic(ErrKeyEmpty)
	}
	return newMemDBIterator(db, start, end, true)
}

// IteratorNoMtx makes an iterator with no mutex.
func (db *MemDB) IteratorNoMtx(start, end []byte) Iterator {
	if (start != nil && len(start) == 0) || (end != nil && len(end) == 0) {
		panic(ErrKeyEmpty)
	}
	return newMemDBIteratorMtxChoice(db, start, end, false, false)
}

// ReverseIteratorNoMtx makes an iterator with no mutex.
func (db *MemDB) ReverseIteratorNoMtx(start, end []byte) (Iterator, error) {
	if (start != nil && len(start) == 0) || (end != nil && len(end) == 0) {
		return nil, ErrKeyEmpty
	}
	return newMemDBIteratorMtxChoice(db, start, end, true, false), nil
}

```
---
### `testdb/memdb_iterator.go`
*2025-02-15 10:17:28 | 4 KB*
```go
package testdb

import (
	"bytes"
	"context"

	"github.com/google/btree"
)

const (
	// Size of the channel buffer between traversal goroutine and iterator. Using an unbuffered
	// channel causes two context switches per item sent, while buffering allows more work per
	// context switch. Tuned with benchmarks.
	chBufferSize = 64
)

// memDBIterator is a memDB iterator.
type memDBIterator struct {
	ch     <-chan *item
	cancel context.CancelFunc
	item   *item
	start  []byte
	end    []byte
	useMtx bool
}

var _ Iterator = (*memDBIterator)(nil)

// newMemDBIterator creates a new memDBIterator.
func newMemDBIterator(db *MemDB, start []byte, end []byte, reverse bool) *memDBIterator {
	return newMemDBIteratorMtxChoice(db, start, end, reverse, true)
}

func newMemDBIteratorMtxChoice(db *MemDB, start []byte, end []byte, reverse bool, useMtx bool) *memDBIterator {
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan *item, chBufferSize)
	iter := &memDBIterator{
		ch:     ch,
		cancel: cancel,
		start:  start,
		end:    end,
		useMtx: useMtx,
	}

	if useMtx {
		db.mtx.RLock()
	}
	go func() {
		if useMtx {
			defer db.mtx.RUnlock()
		}
		// Because we use [start, end) for reverse ranges, while btree uses (start, end], we need
		// the following variables to handle some reverse iteration conditions ourselves.
		var (
			skipEqual     []byte
			abortLessThan []byte
		)
		visitor := func(i btree.Item) bool {
			item := i.(*item)
			if skipEqual != nil && bytes.Equal(item.key, skipEqual) {
				skipEqual = nil
				return true
			}
			if abortLessThan != nil && bytes.Compare(item.key, abortLessThan) == -1 {
				return false
			}
			select {
			case <-ctx.Done():
				return false
			case ch <- item:
				return true
			}
		}
		switch {
		case start == nil && end == nil && !reverse:
			db.btree.Ascend(visitor)
		case start == nil && end == nil && reverse:
			db.btree.Descend(visitor)
		case end == nil && !reverse:
			// must handle this specially, since nil is considered less than anything else
			db.btree.AscendGreaterOrEqual(newKey(start), visitor)
		case !reverse:
			db.btree.AscendRange(newKey(start), newKey(end), visitor)
		case end == nil:
			// abort after start, since we use [start, end) while btree uses (start, end]
			abortLessThan = start
			db.btree.Descend(visitor)
		default:
			// skip end and abort after start, since we use [start, end) while btree uses (start, end]
			skipEqual = end
			abortLessThan = start
			db.btree.DescendLessOrEqual(newKey(end), visitor)
		}
		close(ch)
	}()

	// prime the iterator with the first value, if any
	if item, ok := <-ch; ok {
		iter.item = item
	}

	return iter
}

// Close implements Iterator.
func (i *memDBIterator) Close() error {
	i.cancel()
	for range i.ch { // drain channel
	}
	i.item = nil
	return nil
}

// Domain implements Iterator.
func (i *memDBIterator) Domain() ([]byte, []byte) {
	return i.start, i.end
}

// Valid implements Iterator.
func (i *memDBIterator) Valid() bool {
	return i.item != nil
}

// Next implements Iterator.
func (i *memDBIterator) Next() {
	i.assertIsValid()
	item, ok := <-i.ch
	switch {
	case ok:
		i.item = item
	default:
		i.item = nil
	}
}

// Error implements Iterator.
func (i *memDBIterator) Error() error {
	return nil // famous last words
}

// Key implements Iterator.
func (i *memDBIterator) Key() []byte {
	i.assertIsValid()
	return i.item.key
}

// Value implements Iterator.
func (i *memDBIterator) Value() []byte {
	i.assertIsValid()
	return i.item.value
}

func (i *memDBIterator) assertIsValid() {
	if !i.Valid() {
		panic("iterator is invalid")
	}
}

```
---
### `testdb/types.go`
*2025-02-15 10:17:28 | 1 KB*
```go
package testdb

import (
	"errors"

	"github.com/CosmWasm/wasmvm/v2/types"
)

var (

	// ErrKeyEmpty is returned when attempting to use an empty or nil key.
	ErrKeyEmpty = errors.New("key cannot be empty")

	// ErrValueNil is returned when attempting to set a nil value.
	ErrValueNil = errors.New("value cannot be nil")
)

type Iterator = types.Iterator

```
---
### `version.go`
*2025-02-15 10:17:28 | 1 KB*
```go
package api

/*
#include "bindings.h"
*/
import "C"

func LibwasmvmVersion() (string, error) {
	version_ptr, err := C.version_str()
	if err != nil {
		return "", err
	}
	// For C.GoString documentation see https://pkg.go.dev/cmd/cgo and
	// https://gist.github.com/helinwang/2c7bd2867ea5110f70e6431a7c80cd9b
	version_copy := C.GoString(version_ptr)
	return version_copy, nil
}

```
---
### `version_test.go`
*2025-02-15 10:18:01 | 1 KB*
```go
package api

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLibwasmvmVersion(t *testing.T) {
	version, err := LibwasmvmVersion()
	require.NoError(t, err)
	require.Regexp(t, `^([0-9]+)\.([0-9]+)\.([0-9]+)(-[a-z0-9.]+)?$`, version)
}

```
---

## Summary
Files: 25, Total: 176 KB
Breakdown:
- go: 175 KB
- md: 1 KB