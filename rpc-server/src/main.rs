use wasmvm_rpc_server::run_server;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let addr_str = std::env::args()
        .nth(1)
        .or_else(|| std::env::var("WASMVM_GRPC_ADDR").ok())
        .unwrap_or_else(|| "0.0.0.0:50051".to_string());
    let addr = addr_str.parse()?;

    run_server(addr).await
}
