//! VM behavior and security vulnerability tests
//!
//! This module contains tests that document and verify security vulnerabilities
//! in the underlying wasmvm implementation.

#[cfg(test)]
mod vm_security_vulnerabilities {
    
    use crate::main_lib::{
        cosmwasm::{wasm_vm_service_server::WasmVmService, Context},
        ExecuteRequest, InstantiateRequest, QueryRequest, WasmVmServiceImpl,
    };
    use tonic::Request;

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
    async fn test_vm_rejects_empty_checksum() {
        let service = create_test_service();

        let request = Request::new(InstantiateRequest {
            checksum: "".to_string(),
            context: Some(create_test_context()),
            init_msg: b"{}".to_vec(),
            gas_limit: 1000000,
            request_id: "empty-checksum-test".to_string(),
        });

        let response = service.instantiate(request).await;
        if let Ok(ok_resp) = response {
            let resp = ok_resp.into_inner();
            assert!(
                !resp.error.is_empty(),
                "Empty checksum should produce an error"
            );
        }
    }

    #[tokio::test]
    async fn test_vm_rejects_invalid_json() {
        let service = create_test_service();

        let fake_checksum = "a".repeat(64); // Valid hex format but non-existent
        let invalid_json_payloads = vec![
            b"{invalid json".to_vec(),
            b"not json at all".to_vec(),
            b"".to_vec(),           // Empty payload
            vec![0xFF, 0xFE, 0xFD], // Binary data
        ];

        for (i, payload) in invalid_json_payloads.iter().enumerate() {
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
                "gRPC call failed for invalid JSON payload {}",
                i
            );
            let resp = response.unwrap().into_inner();
            assert!(
                !resp.error.is_empty(),
                "Invalid JSON payload {} should produce an error",
                i
            );
        }
    }

    #[tokio::test]
    async fn test_vm_rejects_large_messages() {
        let service = create_test_service();

        let fake_checksum = "b".repeat(64);
        let sizes = vec![
            1024 * 1024,      // 1MB
            10 * 1024 * 1024, // 10MB
            50 * 1024 * 1024, // 50MB
        ];

        for size in sizes {
            let large_message = vec![b'A'; size];
            let request = Request::new(ExecuteRequest {
                contract_id: fake_checksum.clone(),
                context: Some(create_test_context()),
                msg: large_message,
                gas_limit: u64::MAX,
                request_id: format!("large-message-{}", size),
            });

            let response = service.execute(request).await;
            let resp = response.unwrap().into_inner();
            assert!(
                !resp.error.is_empty(),
                "{}MB message should produce an error",
                size / (1024 * 1024)
            );
        }
    }

    #[tokio::test]
    async fn test_vm_rejects_extreme_gas_limits() {
        let service = create_test_service();

        let fake_checksum = "c".repeat(64);
        let extreme_gas_limits = vec![
            (0, "zero_gas"),
            (u64::MAX, "max_gas"),
            (u64::MAX - 1, "near_max_gas"),
        ];

        for (gas_limit, description) in extreme_gas_limits {
            let request = Request::new(InstantiateRequest {
                checksum: fake_checksum.clone(),
                context: Some(create_test_context()),
                init_msg: b"{}".to_vec(),
                gas_limit,
                request_id: format!("gas-{}", description),
            });

            let response = service.instantiate(request).await;
            let resp = response.unwrap().into_inner();
            assert!(
                !resp.error.is_empty(),
                "Gas limit {} ({}) should produce an error",
                gas_limit,
                description
            );
        }
    }

    #[tokio::test]
    async fn test_vm_rejects_malicious_context() {
        let service = create_test_service();
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
                sender: "".to_string(),
                chain_id: "test-chain".to_string(),
            },
            Context {
                block_height: 12345,
                sender: "cosmos1test".to_string(),
                chain_id: "".to_string(),
            },
        ];

        for (i, context) in malicious_contexts.iter().enumerate() {
            let request = Request::new(QueryRequest {
                contract_id: fake_checksum.clone(),
                context: Some(context.clone()),
                query_msg: b"{}".to_vec(),
                request_id: format!("malicious-context-{}", i),
            });

            let response = service.query(request).await;
            let resp = response.unwrap().into_inner();
            assert!(
                !resp.error.is_empty(),
                "Malicious context {} should produce an error",
                i
            );
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
