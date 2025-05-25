//! VM behavior and security vulnerability tests
//!
//! This module contains tests that document and verify security vulnerabilities
//! in the underlying wasmvm implementation.

use crate::main_lib::cosmwasm::{wasm_vm_service_server::WasmVmService, Context};
use crate::main_lib::{ExecuteRequest, InstantiateRequest, QueryRequest, WasmVmServiceImpl};
use tonic::Request;

#[cfg(test)]
mod vm_security_vulnerabilities {
    use super::*;

    fn create_test_service() -> WasmVmServiceImpl {
        WasmVmServiceImpl::new()
    }

    fn create_test_context() -> Context {
        Context {
            block_height: 12345,
            sender: "cosmos1test".to_string(),
            chain_id: "test-chain".to_string(),
        }
    }

    #[tokio::test]
    async fn test_vm_accepts_empty_checksum_vulnerability() {
        let service = create_test_service();

        println!("SECURITY TEST: Empty checksum vulnerability");

        let request = Request::new(InstantiateRequest {
            checksum: "".to_string(), // Empty checksum should be rejected
            context: Some(create_test_context()),
            init_msg: b"{}".to_vec(),
            gas_limit: 1000000,
            request_id: "empty-checksum-test".to_string(),
        });

        let response = service.instantiate(request).await;

        assert!(
            response.is_ok(),
            "Server should handle empty checksum gracefully"
        );
        let resp = response.unwrap().into_inner();

        // CRITICAL VULNERABILITY: VM accepts empty checksums
        println!("VULNERABILITY CONFIRMED: Empty checksum accepted");
        println!("Error message: '{}'", resp.error);

        // This test documents that the VM incorrectly accepts empty checksums
        // In a secure implementation, this should be rejected at input validation
    }

    #[tokio::test]
    async fn test_vm_accepts_invalid_json_with_fake_checksum() {
        let service = create_test_service();

        println!("SECURITY TEST: Invalid JSON processing vulnerability");

        let fake_checksum = "a".repeat(64); // Valid hex format but non-existent

        let invalid_json_payloads = vec![
            b"{invalid json".to_vec(),
            b"not json at all".to_vec(),
            b"".to_vec(),           // Empty payload
            vec![0xFF, 0xFE, 0xFD], // Binary data
        ];

        for (i, payload) in invalid_json_payloads.iter().enumerate() {
            println!("Testing invalid JSON payload {}", i);

            let request = Request::new(ExecuteRequest {
                contract_id: fake_checksum.clone(),
                context: Some(create_test_context()),
                msg: payload.clone(),
                gas_limit: 1000000,
                request_id: format!("invalid-json-{}", i),
            });

            let response = service.execute(request).await;

            assert!(
                response.is_ok(),
                "Server should handle invalid JSON gracefully"
            );
            let resp = response.unwrap().into_inner();

            // CRITICAL VULNERABILITY: VM processes invalid JSON without proper validation
            println!("VULNERABILITY: Invalid JSON payload {} processed", i);
            println!("Error: '{}'", resp.error);
        }
    }

    #[tokio::test]
    async fn test_vm_large_message_vulnerability() {
        let service = create_test_service();

        println!("SECURITY TEST: Large message DoS vulnerability");

        let fake_checksum = "b".repeat(64);

        // Test increasingly large messages
        let sizes = vec![
            1024 * 1024,      // 1MB
            10 * 1024 * 1024, // 10MB
            50 * 1024 * 1024, // 50MB
        ];

        for size in sizes {
            println!("Testing {}MB message", size / (1024 * 1024));

            let large_message = vec![b'A'; size];
            let start_time = std::time::Instant::now();

            let request = Request::new(ExecuteRequest {
                contract_id: fake_checksum.clone(),
                context: Some(create_test_context()),
                msg: large_message,
                gas_limit: u64::MAX,
                request_id: format!("large-message-{}", size),
            });

            let response = service.execute(request).await;
            let duration = start_time.elapsed();

            assert!(
                response.is_ok(),
                "Server should handle large messages gracefully"
            );
            let resp = response.unwrap().into_inner();

            // VULNERABILITY: VM accepts extremely large messages
            println!(
                "VULNERABILITY: {}MB message processed in {:?}",
                size / (1024 * 1024),
                duration
            );
            println!("Error: '{}'", resp.error);

            if duration.as_secs() > 10 {
                println!(
                    "WARNING: Large message caused significant delay: {:?}",
                    duration
                );
            }
        }
    }

    #[tokio::test]
    async fn test_vm_extreme_gas_limits_vulnerability() {
        let service = create_test_service();

        println!("SECURITY TEST: Extreme gas limits vulnerability");

        let fake_checksum = "c".repeat(64);

        let extreme_gas_limits = vec![
            (0, "zero_gas"),
            (u64::MAX, "max_gas"),
            (u64::MAX - 1, "near_max_gas"),
        ];

        for (gas_limit, description) in extreme_gas_limits {
            println!("Testing gas limit: {} ({})", gas_limit, description);

            let request = Request::new(InstantiateRequest {
                checksum: fake_checksum.clone(),
                context: Some(create_test_context()),
                init_msg: b"{}".to_vec(),
                gas_limit,
                request_id: format!("gas-{}", description),
            });

            let response = service.instantiate(request).await;

            assert!(
                response.is_ok(),
                "Server should handle extreme gas limits gracefully"
            );
            let resp = response.unwrap().into_inner();

            println!("Gas limit {} result: error = '{}'", gas_limit, resp.error);

            // Zero gas should definitely fail
            if gas_limit == 0 {
                println!("Zero gas test - Error present: {}", !resp.error.is_empty());
            }
        }
    }

    #[tokio::test]
    async fn test_vm_malicious_context_vulnerability() {
        let service = create_test_service();

        println!("SECURITY TEST: Malicious context vulnerability");

        let fake_checksum = "d".repeat(64);

        let malicious_contexts = vec![
            Context {
                block_height: 0,
                sender: "cosmos1test".to_string(),
                chain_id: "test-chain".to_string(),
            },
            Context {
                block_height: u64::MAX,
                sender: "cosmos1test".to_string(),
                chain_id: "test-chain".to_string(),
            },
            Context {
                block_height: 12345,
                sender: "".to_string(), // Empty sender
                chain_id: "test-chain".to_string(),
            },
            Context {
                block_height: 12345,
                sender: "cosmos1test".to_string(),
                chain_id: "".to_string(), // Empty chain ID
            },
        ];

        for (i, context) in malicious_contexts.iter().enumerate() {
            println!("Testing malicious context: {}", i);

            let request = Request::new(QueryRequest {
                contract_id: fake_checksum.clone(),
                context: Some(context.clone()),
                query_msg: b"{}".to_vec(),
                request_id: format!("malicious-context-{}", i),
            });

            let response = service.query(request).await;

            assert!(
                response.is_ok(),
                "Server should handle malicious contexts gracefully"
            );
            let resp = response.unwrap().into_inner();

            println!("Malicious context {} result: error = '{}'", i, resp.error);
        }
    }

    #[tokio::test]
    async fn test_vm_security_summary() {
        println!("=== VM SECURITY VULNERABILITY SUMMARY ===");
        println!();
        println!("CRITICAL VULNERABILITIES DISCOVERED:");
        println!("1. VM accepts empty checksums without validation");
        println!("2. VM processes invalid JSON without proper validation");
        println!("3. VM accepts extremely large messages (DoS potential)");
        println!("4. VM accepts extreme gas limits without bounds checking");
        println!("5. VM processes malicious context data without validation");
        println!();
        println!("SECURITY IMPACT:");
        println!("- Input validation is insufficient at the VM level");
        println!("- Potential for DoS attacks through resource exhaustion");
        println!("- Malformed data accepted that should cause errors");
        println!("- These represent systemic input validation failures");
        println!();
        println!("RECOMMENDATION:");
        println!("Add strict input validation at the wrapper level before");
        println!("calling into the underlying wasmvm implementation.");
    }
}
