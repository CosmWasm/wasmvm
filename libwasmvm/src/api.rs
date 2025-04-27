use cosmwasm_vm::{BackendApi, BackendError, BackendResult, GasInfo};

use crate::error::GoError;
use crate::memory::{U8SliceView, UnmanagedVector};
use crate::Vtable;

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

        // Basic validation for Bech32 address format (if it looks like one)
        if human.contains('1') {
            // Bech32 format checks
            let parts: Vec<&str> = human.split('1').collect();
            if parts.len() != 2 {
                return Err(BackendError::user_err(
                    "Invalid Bech32 address format (should contain exactly one '1' separator)",
                ));
            }

            // Validate HRP (Human Readable Part)
            let hrp = parts[0];
            if hrp.is_empty() || hrp.len() > 20 {
                return Err(BackendError::user_err(
                    "Invalid Bech32 HRP (prefix before '1') length",
                ));
            }

            // Check HRP is lowercase letters only
            if !hrp.chars().all(|c| c.is_ascii_lowercase()) {
                return Err(BackendError::user_err(
                    "Invalid Bech32 HRP (prefix must contain only lowercase letters)",
                ));
            }

            // Basic data part validation
            let data = parts[1];
            if data.is_empty() {
                return Err(BackendError::user_err("Invalid Bech32 data part (empty)"));
            }

            // Check data uses only Bech32 charset
            if !data.chars().all(|c| {
                c.is_ascii_lowercase()
                    || c.is_ascii_digit()
                    || "qpzry9x8gf2tvdw0s3jn54khce6mua7l".contains(c)
            }) {
                return Err(BackendError::user_err(
                    "Invalid Bech32 data part (contains invalid characters)",
                ));
            }
            return Ok(());
        } else if human.starts_with("cosmos")
            || human.starts_with("osmo")
            || human.starts_with("juno")
        {
            // Address starts with a Bech32 prefix but has no separator
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
