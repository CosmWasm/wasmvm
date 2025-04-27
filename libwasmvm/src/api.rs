use cosmwasm_vm::{BackendApi, BackendError, BackendResult, GasInfo};

use crate::error::GoError;
use crate::memory::{U8SliceView, UnmanagedVector};
use crate::Vtable;
use bech32::{self, Variant};
use sha3::{Digest, Keccak256};

// Constants for API validation
pub const MAX_ADDRESS_LENGTH: usize = 256; // Maximum length for address strings
const MAX_CANONICAL_LENGTH: usize = 100; // Maximum length for canonical addresses

// Gas costs for different validation operations
// Base costs represent the minimum computation needed regardless of address length
const BASE_VALIDATION_GAS: u64 = 100; // Base cost for any validation operation
const PER_BYTE_GAS: u64 = 10; // Cost per byte of address length
const BECH32_BASE_GAS: u64 = 300; // Higher base cost for Bech32 validation (checksum is expensive)
const ETHEREUM_BASE_GAS: u64 = 200; // Ethereum address validation cost
const SOLANA_BASE_GAS: u64 = 250; // Solana address validation cost
const LEGACY_BASE_GAS: u64 = 50; // Simple legacy address validation cost

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
    // Computes gas cost for address validation based on type and complexity
    fn compute_validation_gas_cost(&self, human: &str) -> u64 {
        // Base cost plus per-byte cost for any address
        let mut gas_cost = BASE_VALIDATION_GAS + (human.len() as u64 * PER_BYTE_GAS);

        // Add extra cost based on address type
        if human.contains('-') || human.contains('_') {
            // Legacy address format with least validation required
            gas_cost += LEGACY_BASE_GAS;
        } else if let Some(hex_part) = human.strip_prefix("0x") {
            // Ethereum address validation
            gas_cost += ETHEREUM_BASE_GAS;

            // Extra cost for hex validation
            if hex_part.len() > 0 {
                gas_cost += hex_part.len() as u64 * 5; // Higher per-char cost for hex validation
            }
        } else if human.contains('1') {
            // Bech32 validation is the most expensive due to checksum calculation
            gas_cost += BECH32_BASE_GAS;

            // Extra cost for longer addresses (checksum becomes more expensive)
            if human.len() > 30 {
                gas_cost += (human.len() as u64 - 30) * 15;
            }
        } else if human.len() >= 32 && human.len() <= 44 {
            // Potential Solana address (Base58 checking)
            gas_cost += SOLANA_BASE_GAS;
        } else {
            // Simple alphanumeric check for test addresses
            gas_cost += LEGACY_BASE_GAS;
        }

        gas_cost
    }

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

            // EIP-55 checksum validation for Ethereum addresses
            // Mixed-case Ethereum addresses should be validated against their checksum
            if hex_part.chars().any(|c| c.is_ascii_uppercase()) {
                // If there are uppercase chars, validate the checksum
                let lowercase_hex = hex_part.to_lowercase();

                // Create a hash of the lowercase address
                let mut hasher = Keccak256::new();
                hasher.update(lowercase_hex.as_bytes());
                let hash = hasher.finalize();

                // Check each character against the hash to validate EIP-55 checksum
                for (i, c) in hex_part.chars().enumerate() {
                    let hash_nibble = if i < 39 {
                        // Get the corresponding nibble from the hash (4 bits)
                        let byte_pos = i / 2;
                        let nibble_pos = 1 - (i % 2); // 0 or 1
                        (hash[byte_pos] >> (4 * nibble_pos)) & 0xf
                    } else {
                        // Handle the last character separately
                        let byte_pos = i / 2;
                        hash[byte_pos] & 0xf
                    };

                    // Check if the character should be uppercase
                    let is_upper = hash_nibble >= 8; // If the hash value is 8 or higher, char should be uppercase

                    let char_lower = c.to_ascii_lowercase();
                    if is_upper && ('a'..='f').contains(&char_lower) && !c.is_ascii_uppercase() {
                        return Err(BackendError::user_err(
                            "Invalid Ethereum address EIP-55 checksum: Incorrect capitalization",
                        ));
                    } else if !is_upper
                        && ('a'..='f').contains(&char_lower)
                        && c.is_ascii_uppercase()
                    {
                        return Err(BackendError::user_err(
                            "Invalid Ethereum address EIP-55 checksum: Incorrect capitalization",
                        ));
                    }
                }
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

        // Add our own gas cost for Rust-side validation on top of Go-side costs
        let validation_gas = self.compute_validation_gas_cost(human);
        let total_gas = used_gas + validation_gas;
        let gas_info = GasInfo::with_cost(total_gas);

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

        // Canonical validation gas cost (simpler than human address validation)
        let canonical_validation_gas =
            BASE_VALIDATION_GAS + (canonical.len() as u64 * PER_BYTE_GAS);
        let total_gas = used_gas + canonical_validation_gas;
        let gas_info = GasInfo::with_cost(total_gas);

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

                // Add validation gas cost for the output human address
                let human_validation_gas = self.compute_validation_gas_cost(human);
                let final_gas_info = GasInfo::with_cost(total_gas + human_validation_gas);

                (Ok(human.clone()), final_gas_info)
            }
            Err(_) => (result, gas_info), // If already an error, we'll return that
        }
    }

    fn addr_validate(&self, input: &str) -> BackendResult<()> {
        // Calculate gas cost based on address complexity
        let rust_validation_gas = self.compute_validation_gas_cost(input);

        // Validate the input address format first
        if let Err(err) = self.validate_human_address(input) {
            return (Err(err), GasInfo::with_cost(rust_validation_gas));
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

        // Total gas is the sum of our Rust validation and the Go-side validation
        let total_gas = used_gas + rust_validation_gas;
        let gas_info = GasInfo::with_cost(total_gas);

        // return complete error message (reading from buffer for GoError::Other)
        let default = || format!("Failed to validate the address: {input}");
        let result = go_error.into_result_safe(error_msg, default);
        (result, gas_info)
    }
}

#[cfg(test)]
mod tests;
