# WasmVM Fixes Needed for wasmd Compatibility

## Critical Issues Identified from wasmd Test Failures

### 1. IBC2 Entry Point Error Handling

**Problem**: When contracts don't export IBC2 entry points, wasmvm returns errors but wasmd gets nil pointer panics.

**Root Cause**: The Go bindings/FFI layer may not be properly handling the error case for missing entry points.

**Required Fix**: Ensure that when IBC2 entry points are missing:
- wasmvm returns a proper error response structure
- The response object is never nil, even on error
- Error messages are clear and actionable

### 2. Response Structure Consistency

**Problem**: wasmd expects consistent response structures even for error cases.

**Required Fix**: 
- Always return a valid response object
- Set error field appropriately
- Ensure all response fields have sensible defaults

### 3. Entry Point Detection

**Problem**: Need better detection and reporting of missing entry points.

**Required Fix**:
- Implement `has_ibc2_entry_points` detection in analyze_code
- Return specific error codes for missing entry points
- Provide clear error messages indicating which entry point is missing

## Specific Code Changes Needed

### 1. In libwasmvm FFI Layer

```rust
// Ensure all IBC2 functions return valid response structures
pub fn ibc2_packet_send(...) -> IbcResponse {
    match call_contract_entry_point("ibc2_packet_send", ...) {
        Ok(result) => IbcResponse {
            data: result,
            error: String::new(),
            gas_used: gas_report.used,
        },
        Err(e) if e.contains("Missing export") => IbcResponse {
            data: vec![],
            error: format!("Contract does not implement IBC2: {}", e),
            gas_used: 0,
        },
        Err(e) => IbcResponse {
            data: vec![],
            error: e,
            gas_used: gas_report.used,
        }
    }
}
```

### 2. In Go Bindings

```go
// Ensure C FFI calls never return nil pointers
func CallIBC2PacketSend(...) (*IbcResponse, error) {
    result := C.ibc2_packet_send(...)
    
    // Always return a valid response object
    response := &IbcResponse{}
    
    if result.error != nil {
        response.Error = C.GoString(result.error)
    }
    
    if result.data != nil {
        response.Data = C.GoBytes(result.data, result.data_len)
    }
    
    return response, nil // Never return nil response
}
```

### 3. Enhanced Error Messages

Instead of generic "Missing export", provide specific messages:
- "Contract does not implement IBC2 packet send entry point"
- "IBC2 features require cosmwasm 2.0+ contract"
- "Use regular IBC entry points for this contract"

## Testing Requirements

1. **Unit Tests**: Verify all IBC2 methods handle missing entry points gracefully
2. **Integration Tests**: Test with real contracts that don't have IBC2 support
3. **Error Message Tests**: Verify error messages are helpful and actionable
4. **Nil Pointer Tests**: Ensure no nil pointers are ever returned to Go code

## Priority

**P0 - Critical**: These fixes are required for wasmd compatibility and prevent runtime panics.

## Impact

- Fixes wasmd test failures
- Prevents runtime panics in production
- Provides better developer experience with clear error messages
- Maintains backward compatibility with non-IBC2 contracts 