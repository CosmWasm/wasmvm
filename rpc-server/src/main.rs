use tonic::{transport::Server, Request, Response, Status};

mod cosmwasm {
    tonic::include_proto!("cosmwasm");
}

use cosmwasm::host_service_server::{HostService, HostServiceServer};
use cosmwasm::wasm_vm_service_server::{WasmVmService, WasmVmServiceServer};
use cosmwasm::{CallHostFunctionRequest, CallHostFunctionResponse};
use cosmwasm::{
    ExecuteRequest, ExecuteResponse, InstantiateRequest, InstantiateResponse, LoadModuleRequest,
    LoadModuleResponse, QueryRequest, QueryResponse,
};

#[derive(Debug, Default)]
pub struct WasmVmServiceImpl;

#[tonic::async_trait]
impl WasmVmService for WasmVmServiceImpl {
    async fn load_module(
        &self,
        _request: Request<LoadModuleRequest>,
    ) -> Result<Response<LoadModuleResponse>, Status> {
        Err(Status::unimplemented("load_module not implemented"))
    }

    async fn instantiate(
        &self,
        _request: Request<InstantiateRequest>,
    ) -> Result<Response<InstantiateResponse>, Status> {
        Err(Status::unimplemented("instantiate not implemented"))
    }

    async fn execute(
        &self,
        _request: Request<ExecuteRequest>,
    ) -> Result<Response<ExecuteResponse>, Status> {
        Err(Status::unimplemented("execute not implemented"))
    }

    async fn query(
        &self,
        _request: Request<QueryRequest>,
    ) -> Result<Response<QueryResponse>, Status> {
        Err(Status::unimplemented("query not implemented"))
    }
}

#[derive(Debug, Default)]
pub struct HostServiceImpl;

#[tonic::async_trait]
impl HostService for HostServiceImpl {
    async fn call_host_function(
        &self,
        _request: Request<CallHostFunctionRequest>,
    ) -> Result<Response<CallHostFunctionResponse>, Status> {
        Err(Status::unimplemented("call_host_function not implemented"))
    }
}

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let addr_str = std::env::args()
        .nth(1)
        .or_else(|| std::env::var("WASMVM_GRPC_ADDR").ok())
        .unwrap_or_else(|| "0.0.0.0:50051".to_string());
    let addr = addr_str.parse()?;
    let wasm_service = WasmVmServiceImpl::default();
    let host_service = HostServiceImpl::default();

    println!("WasmVM gRPC server listening on {}", addr);

    Server::builder()
        .add_service(WasmVmServiceServer::new(wasm_service))
        .add_service(HostServiceServer::new(host_service))
        .serve(addr)
        .await?;

    Ok(())
}
