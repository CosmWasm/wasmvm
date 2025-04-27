#[cfg(test)]
mod tests {
    use super::*;
    use bech32::{encode, ToBase32, Variant};
    use cosmwasm_vm::testing::mock_backend::{MockApi, MockApiBackend};
    use sha3::{Digest, Keccak256};

    fn setup_api() -> MockApiBackend {
        MockApiBackend::default()
    }

    #[test]
    fn test_graduated_gas_costs() {
        // For our test, we need a real GoApi instance with access to the compute_validation_gas_cost method
        // We can use the approach below to test that our graduated gas costs follow the expected pattern

        // These should follow the pattern established in the compute_validation_gas_cost method:
        // Simple address (alphanumeric) < Legacy address (with hyphens) < Ethereum < Solana < Bech32
        // And longer addresses should cost more than shorter ones of the same type

        // Test addresses of increasing complexity
        let addresses = [
            "simple",                                        // Simple alphanumeric (cheapest)
            "test-address",                                  // Legacy address with hyphen
            "0x1234567890123456789012345678901234567890",    // Ethereum address (medium cost)
            "7v91N7iZ9mNicL8WfG6cgSCKyRXydQjLh6UYBWwm6y1Q",  // Solana-like address
            "cosmos1q9f0qwgmwvyg0pyp38g4lw2cznugwz8pc9qd3l", // Bech32 address (expensive)
            "cosmos1qypqxpq9qcrsszg2pvxq6rs0zqg3yyc5vkvm5zyqwsx442enk2ymqahsdf9", // Long Bech32 (most expensive)
        ];

        // Create a GoApi instance directly to test the gas computation
        let api = super::GoApi {
            state: std::ptr::null(),
            vtable: super::GoApiVtable::default(),
        };

        // Calculate gas costs for each address type
        let gas_costs: Vec<u64> = addresses
            .iter()
            .map(|addr| api.compute_validation_gas_cost(addr))
            .collect();

        // Print gas costs for debugging
        for (i, &addr) in addresses.iter().enumerate() {
            println!("{}: {} gas", addr, gas_costs[i]);
        }

        // Verify that costs increase with complexity
        for i in 0..gas_costs.len() - 1 {
            assert!(
                gas_costs[i] < gas_costs[i + 1],
                "Gas cost should increase with address complexity: {} < {}",
                addresses[i],
                addresses[i + 1]
            );
        }
    }

    #[test]
    fn test_validate_bech32_addresses() {
        let api = setup_api();

        // Valid Bech32 addresses should pass
        let valid_bech32 = vec![
            "cosmos1q9f0qwgmwvyg0pyp38g4lw2cznugwz8pc9qd3l",
            "osmo1m8pqpkly9nz3r30f0wp09h57mqkhsr9373pj9m",
            "juno1pxc48gd3cydz847wgvt0p23zlc5wf88pjdptnt",
            "akash1p7egumv92ymaut4v6egg0hrlncr20mzxyrrt3h",
            "kava1aa7vpfq09x3mqsxwx8f7vz6c3tsrw2ua57204h",
            "secret1rgm2m5t530tdzyd99775n6vzumxa5luxcllml4",
            "regen1ez43v7jhwl5kgcsljf5592h98zgl6hvlr8vesz",
            "band1yvmwt4jwhz9xshx5cvm4qh6zxqycgcvpqgvs50",
            "certik1gm04fm4ssz330hx4vmwv8ngh6zjhsca8kmj2c",
            "bitsong1jtyjtj23argjv9kf38pt6ys2vnyygrzmht74cx",
            "meter1xlcvutz2kxmtqhpvt6v58969k39fv60dfp90sa",
            "sentinel1rz4lxzddmfavqh5xnyxlvqwrg4r73hpyw48ve",
            "irishub1yar4p5jqftwqnmrgsqdp9v4z5n6c8z0qhf82g",
            "terra1su47ftahkw2dj9caekqpdv9c66dpjhhtuuhzxz",
            "chihuahua1p79gm3hf0vy79qte9lsxwtqmfv9slv3keqd86z",
            "comdex1z9a3z2vp6xk5zcm92k8netj8c5vp2wusuqtpm3",
            "like1v99c0zdermvfm5ph59kpz8dxelfwkhun6zrfhx",
            "nft1e4tnehs8enjuam5ekqkq0ncmfxc0t6a7tm58ch",
            "axelar1px6v3xqftf0sfrzz458jn7waxdv6r52efl8yk2",
            "persistence19j47q6n2jz3jmgrm9uv48n7y26ynpzkwt5ftm",
        ];

        for addr in valid_bech32 {
            let result = api.addr_validate(addr).0;
            assert!(result.is_ok(), "Valid Bech32 address should pass: {}", addr);
        }

        // Invalid Bech32 addresses should fail
        let invalid_bech32 = vec![
            "cosmos",                         // Missing separator and data
            "cosmo1xyz",                      // Invalid HRP (too short)
            "cosmos@1xyz",                    // Invalid character in HRP
            "cosmos1XYZ",                     // Invalid characters in data (uppercase)
            "cosmos1validhrpbut@invaliddata", // Invalid characters in data
            // Checksum errors
            "cosmos1q9f0qwgmwvyg0pyp38g4lw2cznugwz8pc9qd3m", // Changed last character
            "bitsong1jtyjtj23argjv9kf38pt6ys2vnyygrzmht74cz", // Changed last character
            // Implausible addresses
            "cosmos1tooshort",            // Too short data part
            "cosmos1" + &"q".repeat(200), // Too long data part
            // Inconsistent case - trying to pass Bech32 checks but with uppercase prefix
            "COSMOS1q9f0qwgmwvyg0pyp38g4lw2cznugwz8pc9qd3l",
        ];

        for addr in invalid_bech32 {
            let result = api.addr_validate(addr).0;
            assert!(
                result.is_err(),
                "Invalid Bech32 address should fail: {}",
                addr
            );
        }
    }

    #[test]
    fn test_encode_and_validate_bech32() {
        let api = setup_api();

        // List of prefixes to test
        let prefixes = vec![
            "cosmos",
            "osmo",
            "juno",
            "akash",
            "kava",
            "secret",
            "regen",
            "band",
            "certik",
            "bitsong",
            "meter",
            "sentinel",
            "irishub",
            "terra",
            "chihuahua",
            "comdex",
            "like",
            "nft",
            "axelar",
            "persistence",
            "omniflix",
            "desmos",
            "bitcanna",
            "evmos",
            "gravity",
            "injective",
            "ixo",
            "sifchain",
            "starname",
            // Add custom prefixes
            "test",
            "demo",
            "custom",
            "mychain",
            "xyz",
            "chain",
            // Shorter prefixes for testing
            "a",
            "b",
            "c",
            "x",
            "y",
            "z",
        ];

        // Sample data to encode (simulating an address)
        let data = vec![
            0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99, 0xAA, 0xBB, 0xCC, 0xDD,
            0xEE, 0xFF, 0x00, 0x11, 0x22, 0x33,
        ];

        for prefix in prefixes {
            // Encode using standard Bech32
            let addr_standard = encode(prefix, data.to_base32(), Variant::Bech32)
                .expect(&format!("Failed to encode Bech32 for prefix {}", prefix));

            // Test validation of the correctly encoded address
            let result_standard = api.addr_validate(&addr_standard).0;
            assert!(
                result_standard.is_ok(),
                "Valid Bech32 address should pass: {}",
                addr_standard
            );

            // Encode using Bech32m (newer variant, which should also be accepted)
            let addr_m = encode(prefix, data.to_base32(), Variant::Bech32m)
                .expect(&format!("Failed to encode Bech32m for prefix {}", prefix));

            // Test validation of the correctly encoded address
            let result_m = api.addr_validate(&addr_m).0;
            assert!(
                result_m.is_ok(),
                "Valid Bech32m address should pass: {}",
                addr_m
            );

            // Create an invalid address by corrupting the last character
            let mut invalid_addr = addr_standard.clone();
            let last_char = invalid_addr.pop().unwrap();
            // Change last char to something else in the charset
            let new_last_char = if last_char == 'q' { 'p' } else { 'q' };
            invalid_addr.push(new_last_char);

            // This should fail validation due to checksum error
            let result_invalid = api.addr_validate(&invalid_addr).0;
            assert!(
                result_invalid.is_err(),
                "Invalid Bech32 address (checksum error) should fail: {}",
                invalid_addr
            );
        }
    }

    #[test]
    fn test_validate_ethereum_addresses() {
        let api = setup_api();

        // Valid Ethereum addresses should pass
        let valid_eth = vec![
            "0x1234567890123456789012345678901234567890",
            "0xabcdef1234567890abcdef1234567890abcdef12",
            "0xABCDEF1234567890ABCDEF1234567890ABCDEF12",
        ];

        for addr in valid_eth {
            let result = api.addr_validate(addr).0;
            assert!(
                result.is_ok(),
                "Valid Ethereum address should pass: {}",
                addr
            );
        }

        // Invalid Ethereum addresses should fail
        let invalid_eth = vec![
            "0x",                                             // Too short
            "0x1234",                                         // Too short
            "0xXYZinvalidhex1234567890123456789012345678",    // Invalid hex
            "0x12345678901234567890123456789012345678901234", // Too long
        ];

        for addr in invalid_eth {
            let result = api.addr_validate(addr).0;
            assert!(
                result.is_err(),
                "Invalid Ethereum address should fail: {}",
                addr
            );
        }
    }

    #[test]
    fn test_validate_ethereum_eip55_checksum() {
        let api = setup_api();

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

    #[test]
    fn test_validate_solana_addresses() {
        let api = setup_api();

        // Valid Solana addresses (these are examples, replace with actual valid Solana addresses if needed)
        let valid_solana = vec![
            "8ZNnujnWZQbcwqiCZUVJ8YDtKsfWxWjLQMVANDEM8A3E",
            "4nvZMGmKLHNWgmL2Jddp7jrPuQrjKUeMD7ixkeaLfZ2i",
            "GrDMoeqMLFjeXQ24H56S1RLgT4R76jsuWCd6SvXyGPQ5",
        ];

        for addr in valid_solana {
            let result = api.addr_validate(addr).0;
            assert!(result.is_ok(), "Valid Solana address should pass: {}", addr);
        }

        // Invalid Solana addresses
        let invalid_solana = vec![
            "InvalidBase58CharsOI0", // Contains invalid Base58 chars (O and 0)
            "TooShort",              // Too short
            "ThisIsTooLongToBeAValidSolanaAddressAndShouldBeRejectedByTheValidator", // Too long
        ];

        for addr in invalid_solana {
            let result = api.addr_validate(addr).0;
            assert!(
                result.is_err(),
                "Invalid Solana address should fail: {}",
                addr
            );
        }
    }

    #[test]
    fn test_legacy_test_addresses() {
        let api = setup_api();

        // Legacy addresses with hyphens or underscores should pass for compatibility
        let legacy_addrs = vec![
            "contract-address",
            "reflect_acct_1",
            "legacy-address-with-hyphens",
            "legacy_address_with_underscores",
        ];

        for addr in legacy_addrs {
            let result = api.addr_validate(addr).0;
            assert!(result.is_ok(), "Legacy test address should pass: {}", addr);
        }
    }

    #[test]
    fn test_empty_and_oversized_addresses() {
        let api = setup_api();

        // Empty address should fail
        let result = api.addr_validate("").0;
        assert!(result.is_err(), "Empty address should fail");

        // Oversized address should fail
        let very_long_addr = "a".repeat(MAX_ADDRESS_LENGTH + 1);
        let result = api.addr_validate(&very_long_addr).0;
        assert!(result.is_err(), "Oversized address should fail");
    }
}
