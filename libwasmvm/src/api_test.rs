#[cfg(test)]
mod tests {
    use super::*;
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
