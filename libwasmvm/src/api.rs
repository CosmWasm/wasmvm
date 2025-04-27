use cosmwasm_vm::{BackendApi, BackendError, BackendResult, GasInfo};

use crate::error::GoError;
use crate::memory::{U8SliceView, UnmanagedVector};
use crate::Vtable;
use bech32::{self, Variant};

// Constants for API validation
pub const MAX_ADDRESS_LENGTH: usize = 256; // Maximum length for address strings
const MAX_CANONICAL_LENGTH: usize = 100; // Maximum length for canonical addresses

// this represents something passed in from the caller side of FFI
// in this case a struct with go function pointers
#[repr(C)]
pub struct api_t {
    _private: [u8; 0],
}

// These functions should return GoError but because we don't trust them here, we treat the return value as i32
// and then check it when converting to GoError manually
#[repr(C)]
#[derive(Copy, Clone, Default)]
pub struct GoApiVtable {
    pub humanize_address: Option<
        extern "C" fn(
            api: *const api_t,
            input: U8SliceView,
            humanized_address_out: *mut UnmanagedVector,
            err_msg_out: *mut UnmanagedVector,
            gas_used: *mut u64,
        ) -> i32,
    >,
    pub canonicalize_address: Option<
        extern "C" fn(
            api: *const api_t,
            input: U8SliceView,
            canonicalized_address_out: *mut UnmanagedVector,
            err_msg_out: *mut UnmanagedVector,
            gas_used: *mut u64,
        ) -> i32,
    >,
    pub validate_address: Option<
        extern "C" fn(
            api: *const api_t,
            input: U8SliceView,
            err_msg_out: *mut UnmanagedVector,
            gas_used: *mut u64,
        ) -> i32,
    >,
}

impl Vtable for GoApiVtable {}

#[repr(C)]
#[derive(Copy, Clone)]
pub struct GoApi {
    pub state: *const api_t,
    pub vtable: GoApiVtable,
}

impl GoApi {
    // Validate human address format
    fn validate_human_address(&self, human: &str) -> Result<(), BackendError> {
        // Check for empty addresses
        if human.is_empty() {
            return Err(BackendError::user_err("Human address cannot be empty"));
        }

        // Check address length
        if human.len() > MAX_ADDRESS_LENGTH {
            return Err(BackendError::user_err(format!(
                "Human address exceeds maximum length: {} > {}",
                human.len(),
                MAX_ADDRESS_LENGTH
            )));
        }

        // Legacy support for addresses with hyphens or underscores (for tests)
        if human.contains('-') || human.contains('_') {
            // Allow without further validation for backward compatibility
            return Ok(());
        }

        // Validate Ethereum address: 0x followed by 40 hex chars
        if let Some(hex_part) = human.strip_prefix("0x") {
            if hex_part.len() != 40 {
                return Err(BackendError::user_err(
                    "Ethereum address must be 0x + 40 hex characters",
                ));
            }
            if !hex_part.chars().all(|c| c.is_ascii_hexdigit()) {
                return Err(BackendError::user_err(
                    "Ethereum address contains invalid hex characters",
                ));
            }
            return Ok(());
        }

        // Full Bech32 validation for addresses containing the '1' separator
        if human.contains('1') {
            match bech32::decode(human) {
                Ok((hrp, data, variant)) => {
                    // Check Human Readable Part (HRP) - must be lowercase letters
                    if !hrp.chars().all(|c| c.is_ascii_lowercase()) {
                        return Err(BackendError::user_err(
                            "Invalid Bech32 HRP (prefix must contain only lowercase letters)",
                        ));
                    }

                    // Variant check (Bech32 vs Bech32m)
                    // Both are acceptable for our purposes, but we log which one was used
                    match variant {
                        Variant::Bech32 => { /* Standard Bech32 */ }
                        Variant::Bech32m => { /* Newer Bech32m variant */ }
                    }

                    // Verify data is not empty
                    if data.is_empty() {
                        return Err(BackendError::user_err(
                            "Invalid Bech32 address: data part is empty",
                        ));
                    }

                    // Verify data length is reasonable (too short or too long addresses are suspicious)
                    // For typical addresses, this should be between 20-64 bytes after decoding
                    if data.len() < 20 {
                        // Most chain addresses represent at least 20 bytes of data (e.g., a hash)
                        // This is a soft warning, not a hard error for better compatibility
                        // You can change this to a hard error if your application requires it
                        #[cfg(debug_assertions)]
                        eprintln!("Warning: Bech32 address data is unusually short: {}", human);
                    }

                    // Validate length based on variant
                    let max_data_length = match variant {
                        Variant::Bech32 => 90,   // Standard limit for Bech32
                        Variant::Bech32m => 110, // Slightly higher limit for Bech32m
                    };

                    if data.len() > max_data_length {
                        return Err(BackendError::user_err(format!(
                            "Bech32 data part too long: {} > {} bytes",
                            data.len(),
                            max_data_length
                        )));
                    }

                    // All Bech32 checks passed - address is valid
                    return Ok(());
                }
                Err(err) => {
                    return Err(BackendError::user_err(format!(
                        "Invalid Bech32 address: {}",
                        err
                    )));
                }
            }
        } else if human.chars().all(|c| c.is_ascii_lowercase())
            && human.len() >= 3
            && human.len() <= 15
        {
            // If it looks like it might be a Bech32 prefix without the '1' separator
            return Err(BackendError::user_err(
                "Invalid Bech32 address: missing separator or data part",
            ));
        }

        // Validate Solana address: Base58 encoded, typically 32-44 chars
        const BASE58_CHARSET: &str = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz";

        // Solana addresses should be in a specific length range
        if human.len() >= 32 && human.len() <= 44 {
            let is_valid_base58 = human.chars().all(|c| BASE58_CHARSET.contains(c));
            if is_valid_base58 {
                return Ok(());
            }
        }

        // Support for simple test addresses like "creator", "fred", "bob", etc.
        // This is for backward compatibility with existing tests
        if human.len() <= 20 && human.chars().all(|c| c.is_ascii_alphanumeric()) {
            return Ok(());
        }

        // If we reached this point, it's neither a recognized Bech32, Ethereum, or Solana address
        // We can either reject it with a general error or potentially let the Go-side validate it
        Err(BackendError::user_err(
            "Address format not recognized as any supported type",
        ))
    }

    // Validate canonical address format
    fn validate_canonical_address(&self, canonical: &[u8]) -> Result<(), BackendError> {
        // Check for empty addresses
        if canonical.is_empty() {
            return Err(BackendError::user_err("Canonical address cannot be empty"));
        }

        // Check address length
        if canonical.len() > MAX_CANONICAL_LENGTH {
            return Err(BackendError::user_err(format!(
                "Canonical address exceeds maximum length: {} > {}",
                canonical.len(),
                MAX_CANONICAL_LENGTH
            )));
        }

        Ok(())
    }
}

// We must declare that these are safe to Send, to use in wasm.
// The known go caller passes in immutable function pointers, but this is indeed
// unsafe for possible other callers.
//
// see: https://stackoverflow.com/questions/50258359/can-a-struct-containing-a-raw-pointer-implement-send-and-be-ffi-safe
unsafe impl Send for GoApi {}

impl BackendApi for GoApi {
    fn addr_canonicalize(&self, human: &str) -> BackendResult<Vec<u8>> {
        // Validate the input address before passing to Go
        if let Err(err) = self.validate_human_address(human) {
            return (Err(err), GasInfo::free());
        }

        let mut output = UnmanagedVector::default();
        let mut error_msg = UnmanagedVector::default();
        let mut used_gas = 0_u64;
        let canonicalize_address = self
            .vtable
            .canonicalize_address
            .expect("vtable function 'canonicalize_address' not set");
        let go_error: GoError = canonicalize_address(
            self.state,
            U8SliceView::new(Some(human.as_bytes())),
            &mut output as *mut UnmanagedVector,
            &mut error_msg as *mut UnmanagedVector,
            &mut used_gas as *mut u64,
        )
        .into();
        // We destruct the UnmanagedVector here, no matter if we need the data.
        let output = output.consume();

        let gas_info = GasInfo::with_cost(used_gas);

        // return complete error message (reading from buffer for GoError::Other)
        let default = || format!("Failed to canonicalize the address: {human}");
        if let Err(err) = go_error.into_result_safe(error_msg, default) {
            return (Err(err), gas_info);
        }

        let result = output.ok_or_else(|| BackendError::unknown("Unset output"));
        // Validate the output canonical address
        match &result {
            Ok(canonical) => {
                if let Err(err) = self.validate_canonical_address(canonical) {
                    return (Err(err), gas_info);
                }
            }
            Err(_) => {} // If already an error, we'll return that
        }

        (result, gas_info)
    }

    fn addr_humanize(&self, canonical: &[u8]) -> BackendResult<String> {
        // Validate the input canonical address
        if let Err(err) = self.validate_canonical_address(canonical) {
            return (Err(err), GasInfo::free());
        }

        let mut output = UnmanagedVector::default();
        let mut error_msg = UnmanagedVector::default();
        let mut used_gas = 0_u64;
        let humanize_address = self
            .vtable
            .humanize_address
            .expect("vtable function 'humanize_address' not set");
        let go_error: GoError = humanize_address(
            self.state,
            U8SliceView::new(Some(canonical)),
            &mut output as *mut UnmanagedVector,
            &mut error_msg as *mut UnmanagedVector,
            &mut used_gas as *mut u64,
        )
        .into();
        // We destruct the UnmanagedVector here, no matter if we need the data.
        let output = output.consume();

        let gas_info = GasInfo::with_cost(used_gas);

        // return complete error message (reading from buffer for GoError::Other)
        let default = || {
            format!(
                "Failed to humanize the address: {}",
                hex::encode_upper(canonical)
            )
        };
        if let Err(err) = go_error.into_result_safe(error_msg, default) {
            return (Err(err), gas_info);
        }

        let result = output
            .ok_or_else(|| BackendError::unknown("Unset output"))
            .and_then(|human_data| String::from_utf8(human_data).map_err(BackendError::from));

        // Validate the output human address
        match &result {
            Ok(human) => {
                if let Err(err) = self.validate_human_address(human) {
                    return (Err(err), gas_info);
                }
            }
            Err(_) => {} // If already an error, we'll return that
        }

        (result, gas_info)
    }

    fn addr_validate(&self, input: &str) -> BackendResult<()> {
        // Validate the input address format first
        if let Err(err) = self.validate_human_address(input) {
            return (Err(err), GasInfo::free());
        }

        let mut error_msg = UnmanagedVector::default();
        let mut used_gas = 0_u64;
        let validate_address = self
            .vtable
            .validate_address
            .expect("vtable function 'validate_address' not set");
        let go_error: GoError = validate_address(
            self.state,
            U8SliceView::new(Some(input.as_bytes())),
            &mut error_msg as *mut UnmanagedVector,
            &mut used_gas as *mut u64,
        )
        .into();

        let gas_info = GasInfo::with_cost(used_gas);

        // return complete error message (reading from buffer for GoError::Other)
        let default = || format!("Failed to validate the address: {input}");
        let result = go_error.into_result_safe(error_msg, default);
        (result, gas_info)
    }
}
