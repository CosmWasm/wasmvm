use cosmwasm_vm::{testing::mock_backend::MockApiBackend, BackendResult};
use sha3::{Digest, Keccak256};

use super::*;

#[test]
fn test_validate_ethereum_eip55_checksum() {
    let api = MockApiBackend::default();

    // Helper function to generate EIP-55 checksum address
    fn to_eip55_checksum(address: &str) -> String {
        // Remove 0x prefix if present
        let hex_part = address.strip_prefix("0x").unwrap_or(address);

        // Ensure address is normalized to lowercase
        let hex_lower = hex_part.to_lowercase();

        // Hash the lowercase hex address
        let mut hasher = Keccak256::new();
        hasher.update(hex_lower.as_bytes());
        let hash = hasher.finalize();

        // Create checksummed address
        let mut result = String::with_capacity(42); // 0x + 40 chars
        result.push_str("0x");

        for (i, ch) in hex_lower.chars().enumerate() {
            // For each character, calculate the corresponding nibble from the hash
            let nibble = if i < 39 {
                let byte_pos = i / 2;
                let nibble_pos = 1 - (i % 2); // 0 or 1
                (hash[byte_pos] >> (4 * nibble_pos)) & 0xf
            } else {
                // Last character handled separately
                let byte_pos = i / 2;
                hash[byte_pos] & 0xf
            };

            // Capitalize letters based on hash value
            if ('0'..='9').contains(&ch) {
                // Numbers remain as-is
                result.push(ch);
            } else if nibble >= 8 {
                // Letters a-f become uppercase if hash nibble >= 8
                result.push(ch.to_ascii_uppercase());
            } else {
                // Letters a-f remain lowercase if hash nibble < 8
                result.push(ch);
            }
        }

        result
    }

    // Generate valid EIP-55 checksummed addresses
    let base_addresses = vec![
        "0xfb6916095ca1df60bb79ce92ce3ea74c37c5d359",
        "0x52908400098527886e0f7030069857d2e4169ee7",
        "0x8617e340b3d01fa5f11f306f4090fd50e238070d",
        "0xde709f2102306220921060314715629080e2fb77",
        "0x27b1fdb04752bbc536007a920d24acb045561c26",
        "0x5aaeb6053f3e94c9b9a09f33669435e7ef1beaed",
        "0xfb6916095ca1df60bb79ce92ce3ea74c37c5d359",
        "0xdbf03b407c01e7cd3cbea99509d93f8dddc8c6fb",
        "0xd1220a0cf47c7b9be7a2e6ba89f429762e7b9adb",
    ];

    // Convert to valid EIP-55 checksummed addresses
    let valid_eip55: Vec<String> = base_addresses
        .iter()
        .map(|addr| to_eip55_checksum(addr))
        .collect();

    // Test valid checksummed addresses
    for addr in &valid_eip55 {
        let result = api.addr_validate(addr).0;
        assert!(
            result.is_ok(),
            "Valid EIP-55 checksummed address should pass: {}",
            addr
        );
    }

    // Create invalid EIP-55 checksummed addresses by flipping a few character cases
    let invalid_eip55: Vec<String> = valid_eip55
        .iter()
        .map(|addr| {
            let mut invalid = addr.clone();
            let bytes = unsafe { invalid.as_bytes_mut() };

            // Find a character position between index 2 (after 0x) and end that contains a-f
            for i in 2..bytes.len() {
                if (b'a'..=b'f').contains(&bytes[i]) {
                    // Flip to uppercase
                    bytes[i] = bytes[i] - b'a' + b'A';
                    break;
                } else if (b'A'..=b'F').contains(&bytes[i]) {
                    // Flip to lowercase
                    bytes[i] = bytes[i] - b'A' + b'a';
                    break;
                }
            }

            invalid
        })
        .collect();

    // Test invalid checksummed addresses
    for addr in &invalid_eip55 {
        let result = api.addr_validate(addr).0;
        assert!(
            result.is_err(),
            "Invalid EIP-55 checksummed address should fail: {}",
            addr
        );
    }

    // Test all-lowercase (no checksum) addresses - should still pass
    for addr in base_addresses {
        let result = api.addr_validate(addr).0;
        assert!(
            result.is_ok(),
            "Lowercase Ethereum address should pass: {}",
            addr
        );
    }

    // Test all-uppercase addresses - should fail as they're not valid EIP-55
    let all_uppercase: Vec<String> = base_addresses
        .iter()
        .map(|addr| {
            let mut parts = addr.split("0x");
            let prefix = parts.next().unwrap_or("");
            let hex_part = parts.next().unwrap_or("");
            format!("{}0x{}", prefix, hex_part.to_uppercase())
        })
        .collect();

    for addr in &all_uppercase {
        let result = api.addr_validate(addr).0;
        assert!(
            result.is_err(),
            "All-uppercase Ethereum address should fail EIP-55 validation: {}",
            addr
        );
    }
}
