#[cfg(test)]
mod tests {
    use super::*;
    use bech32::{encode, ToBase32, Variant};
    use cosmwasm_vm::testing::mock_backend::{MockApi, MockApiBackend};

    fn setup_api() -> MockApiBackend {
        MockApiBackend::default()
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
