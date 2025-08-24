// Security validation module

// Security validation constants
const MAX_MESSAGE_SIZE: usize = 1024 * 1024; // 1MB
const MAX_REQUEST_ID_LENGTH: usize = 256;
const MAX_CHAIN_ID_LENGTH: usize = 64;
const MAX_SENDER_ADDRESS_LENGTH: usize = 128;
const MAX_CONTRACT_ID_LENGTH: usize = 64;
const MIN_GAS_LIMIT: u64 = 1_000;
const MAX_GAS_LIMIT: u64 = 1_000_000_000_000; // 1 trillion
const MAX_JSON_NESTING_DEPTH: usize = 20;
const MAX_JSON_OBJECT_KEYS: usize = 500;

#[derive(Debug, Clone)]
pub enum ValidationError {
    EmptyChecksum,
    InvalidChecksumLength,
    InvalidChecksumHex,
    MessageTooLarge,
    RequestIdTooLong,
    ChainIdTooLong,
    SenderAddressTooLong,
    ContractIdTooLong,
    InvalidGasLimit,
    InvalidBlockHeight,
    EmptySender,
    EmptyChainId,
    InvalidJson,
    JsonTooComplex,
    InvalidUtf8,
    ContainsDangerousPatterns,
    InvalidFieldLength,
}

impl std::fmt::Display for ValidationError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            ValidationError::EmptyChecksum => write!(f, "Checksum cannot be empty"),
            ValidationError::InvalidChecksumLength => {
                write!(f, "Checksum must be 64 hex characters (32 bytes)")
            }
            ValidationError::InvalidChecksumHex => write!(f, "Checksum must be valid hex"),
            ValidationError::MessageTooLarge => write!(
                f,
                "Message size exceeds maximum allowed ({})",
                MAX_MESSAGE_SIZE
            ),
            ValidationError::RequestIdTooLong => write!(
                f,
                "Request ID exceeds maximum length ({})",
                MAX_REQUEST_ID_LENGTH
            ),
            ValidationError::ChainIdTooLong => write!(
                f,
                "Chain ID exceeds maximum length ({})",
                MAX_CHAIN_ID_LENGTH
            ),
            ValidationError::SenderAddressTooLong => write!(
                f,
                "Sender address exceeds maximum length ({})",
                MAX_SENDER_ADDRESS_LENGTH
            ),
            ValidationError::ContractIdTooLong => write!(
                f,
                "Contract ID exceeds maximum length ({})",
                MAX_CONTRACT_ID_LENGTH
            ),
            ValidationError::InvalidGasLimit => write!(
                f,
                "Gas limit must be between {} and {}",
                MIN_GAS_LIMIT, MAX_GAS_LIMIT
            ),
            ValidationError::InvalidBlockHeight => write!(f, "Block height cannot be zero"),
            ValidationError::EmptySender => write!(f, "Sender address cannot be empty"),
            ValidationError::EmptyChainId => write!(f, "Chain ID cannot be empty"),
            ValidationError::InvalidJson => write!(f, "Invalid JSON format"),
            ValidationError::JsonTooComplex => write!(
                f,
                "JSON structure too complex (max depth: {}, max keys: {})",
                MAX_JSON_NESTING_DEPTH, MAX_JSON_OBJECT_KEYS
            ),
            ValidationError::InvalidUtf8 => write!(f, "Invalid UTF-8 encoding"),
            ValidationError::ContainsDangerousPatterns => {
                write!(f, "Input contains potentially dangerous patterns")
            }
            ValidationError::InvalidFieldLength => {
                write!(f, "Field length exceeds maximum allowed")
            }
        }
    }
}

impl std::error::Error for ValidationError {}

/// Validate checksum format
pub fn validate_checksum(checksum: &str) -> Result<(), ValidationError> {
    if checksum.is_empty() {
        return Err(ValidationError::EmptyChecksum);
    }

    if checksum.len() != 64 {
        return Err(ValidationError::InvalidChecksumLength);
    }

    if hex::decode(checksum).is_err() {
        return Err(ValidationError::InvalidChecksumHex);
    }

    Ok(())
}

/// Validate message payload
pub fn validate_message_payload(data: &[u8]) -> Result<(), ValidationError> {
    // Check size limits
    if data.len() > MAX_MESSAGE_SIZE {
        return Err(ValidationError::MessageTooLarge);
    }

    // Validate UTF-8 if it's supposed to be text
    if !data.is_empty() {
        // Try to parse as JSON to validate structure
        if let Ok(json_str) = std::str::from_utf8(data) {
            validate_json_structure(json_str)?;
        }
    }

    Ok(())
}

/// Validate JSON structure complexity
pub fn validate_json_structure(json_str: &str) -> Result<(), ValidationError> {
    match serde_json::from_str::<serde_json::Value>(json_str) {
        Ok(value) => {
            validate_json_depth(&value, 0)?;
            validate_json_keys(&value)?;
            Ok(())
        }
        Err(_) => Err(ValidationError::InvalidJson),
    }
}

fn validate_json_depth(value: &serde_json::Value, depth: usize) -> Result<(), ValidationError> {
    if depth > MAX_JSON_NESTING_DEPTH {
        return Err(ValidationError::JsonTooComplex);
    }

    match value {
        serde_json::Value::Object(map) => {
            for v in map.values() {
                validate_json_depth(v, depth + 1)?;
            }
        }
        serde_json::Value::Array(arr) => {
            for v in arr {
                validate_json_depth(v, depth + 1)?;
            }
        }
        _ => {}
    }

    Ok(())
}

fn validate_json_keys(value: &serde_json::Value) -> Result<(), ValidationError> {
    if let serde_json::Value::Object(map) = value {
        if map.len() > MAX_JSON_OBJECT_KEYS {
            return Err(ValidationError::JsonTooComplex);
        }

        for v in map.values() {
            validate_json_keys(v)?;
        }
    } else if let serde_json::Value::Array(arr) = value {
        for v in arr {
            validate_json_keys(v)?;
        }
    }

    Ok(())
}

/// Validate gas limits
pub fn validate_gas_limit(gas_limit: u64) -> Result<(), ValidationError> {
    if gas_limit < MIN_GAS_LIMIT || gas_limit > MAX_GAS_LIMIT {
        return Err(ValidationError::InvalidGasLimit);
    }
    Ok(())
}

/// Validate field lengths
pub fn validate_field_lengths(
    request_id: &str,
    chain_id: &str,
    sender: &str,
    contract_id: &str,
) -> Result<(), ValidationError> {
    if request_id.len() > MAX_REQUEST_ID_LENGTH {
        return Err(ValidationError::RequestIdTooLong);
    }

    if chain_id.len() > MAX_CHAIN_ID_LENGTH {
        return Err(ValidationError::ChainIdTooLong);
    }

    if sender.len() > MAX_SENDER_ADDRESS_LENGTH {
        return Err(ValidationError::SenderAddressTooLong);
    }

    if contract_id.len() > MAX_CONTRACT_ID_LENGTH {
        return Err(ValidationError::ContractIdTooLong);
    }

    Ok(())
}

/// Validate context fields
pub fn validate_context_fields(
    block_height: u64,
    sender: &str,
    chain_id: &str,
) -> Result<(), ValidationError> {
    if block_height == 0 {
        return Err(ValidationError::InvalidBlockHeight);
    }

    if sender.is_empty() {
        return Err(ValidationError::EmptySender);
    }

    if chain_id.is_empty() {
        return Err(ValidationError::EmptyChainId);
    }

    Ok(())
}

/// Check for dangerous patterns in text input
pub fn validate_text_safety(text: &str) -> Result<(), ValidationError> {
    // Check for dangerous patterns
    let dangerous_patterns = [
        "'; DROP TABLE",
        "; rm -rf",
        "../../../",
        "\\x00",    // null bytes
        "\u{202E}", // RTL override
        "\u{200B}", // zero-width space
    ];

    let text_lower = text.to_lowercase();
    for pattern in &dangerous_patterns {
        if text_lower.contains(&pattern.to_lowercase()) {
            return Err(ValidationError::ContainsDangerousPatterns);
        }
    }

    // Check for invalid UTF-8 sequences
    if !text.is_utf8() {
        return Err(ValidationError::InvalidUtf8);
    }

    // Check for BOMs and other problematic sequences
    if text.starts_with('\u{FEFF}') || text.starts_with('\u{FFFE}') {
        return Err(ValidationError::ContainsDangerousPatterns);
    }

    Ok(())
}

/// Validate encoding safety
pub fn validate_encoding_safety(data: &[u8]) -> Result<(), ValidationError> {
    // Check for null bytes
    if data.contains(&0) {
        return Err(ValidationError::ContainsDangerousPatterns);
    }

    // If it's valid UTF-8, check text safety
    if let Ok(text) = std::str::from_utf8(data) {
        validate_text_safety(text)?;
    }

    Ok(())
}

trait Utf8Validator {
    fn is_utf8(&self) -> bool;
}

impl Utf8Validator for str {
    fn is_utf8(&self) -> bool {
        std::str::from_utf8(self.as_bytes()).is_ok()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_checksum_validation() {
        // Valid checksum
        let valid_checksum = "a".repeat(64);
        assert!(validate_checksum(&valid_checksum).is_ok());

        // Empty checksum
        assert!(validate_checksum("").is_err());

        // Wrong length
        assert!(validate_checksum("abc123").is_err());

        // Invalid hex
        assert!(validate_checksum(&"g".repeat(64)).is_err());
    }

    #[test]
    fn test_gas_limit_validation() {
        // Valid gas limits
        assert!(validate_gas_limit(1_000_000).is_ok());

        // Too low
        assert!(validate_gas_limit(500).is_err());

        // Too high
        assert!(validate_gas_limit(u64::MAX).is_err());
    }

    #[test]
    fn test_json_complexity() {
        // Simple JSON
        assert!(validate_json_structure(r#"{"key": "value"}"#).is_ok());

        // Too deep nesting
        let deep_json =
            (0..25).fold(String::new(), |acc, _| format!("{{\"a\":{}}}", acc)) + &"}".repeat(25);
        assert!(validate_json_structure(&deep_json).is_err());
    }

    #[test]
    fn test_dangerous_patterns() {
        // Safe text
        assert!(validate_text_safety("hello world").is_ok());

        // SQL injection
        assert!(validate_text_safety("'; DROP TABLE users; --").is_err());

        // Command injection
        assert!(validate_text_safety("; rm -rf /").is_err());

        // Path traversal
        assert!(validate_text_safety("../../../etc/passwd").is_err());
    }
}
