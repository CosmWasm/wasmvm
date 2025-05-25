use hex;
use serde_json::json;
use tonic::{transport::Server, Request, Response, Status};
use wasmvm::{analyze_code as vm_analyze_code, cache_t, init_cache, store_code};
use wasmvm::{ByteSliceView, UnmanagedVector, GoApi, GoApiVtable, Db, DbVtable, GoQuerier, QuerierVtable, GasReport};
use wasmvm::{instantiate as vm_instantiate, execute as vm_execute, query as vm_query, migrate as vm_migrate, sudo as vm_sudo, reply as vm_reply};

mod cosmwasm {
    tonic::include_proto!("cosmwasm");
}

use cosmwasm::host_service_server::{HostService, HostServiceServer};
use cosmwasm::wasm_vm_service_server::{WasmVmService, WasmVmServiceServer};
use cosmwasm::{
    AnalyzeCodeRequest, AnalyzeCodeResponse, ExecuteRequest, ExecuteResponse, InstantiateRequest,
    InstantiateResponse, LoadModuleRequest, LoadModuleResponse, MigrateRequest, MigrateResponse,
    QueryRequest, QueryResponse, ReplyRequest, ReplyResponse, SudoRequest, SudoResponse,
};
use cosmwasm::{CallHostFunctionRequest, CallHostFunctionResponse};

/// WasmVM gRPC service implementation using libwasmvm
#[derive(Clone)]
pub struct WasmVmServiceImpl {
    cache: *mut cache_t,
}
// SAFETY: cache pointer is thread-safe usage of FFI cache
unsafe impl Send for WasmVmServiceImpl {}
unsafe impl Sync for WasmVmServiceImpl {}
impl WasmVmServiceImpl {
    /// Initialize the Wasm module cache with default options
    pub fn new() -> Self {
        // Configure cache: directory, capabilities, sizes
        let config = json!({
            "cache_dir": "./wasm_cache",
            "supported_capabilities": [],
            "max_wasm_size": 104857600u64,
            "max_cache_size": 536870912u64
        });
        let config_bytes = serde_json::to_vec(&config).unwrap();
        let mut err = UnmanagedVector::default();
        let cache = unsafe { init_cache(ByteSliceView::new(Some(&config_bytes)), Some(&mut err)) };
        if cache.is_null() {
            let msg = String::from_utf8(err.consume().unwrap()).unwrap();
            panic!("init_cache failed: {}", msg);
        }
        WasmVmServiceImpl { cache }
    }
}
impl Default for WasmVmServiceImpl {
    fn default() -> Self {
        Self::new()
    }
}

#[tonic::async_trait]
impl WasmVmService for WasmVmServiceImpl {
    async fn load_module(
        &self,
        request: Request<LoadModuleRequest>,
    ) -> Result<Response<LoadModuleResponse>, Status> {
        let req = request.into_inner();
        let wasm_bytes = req.module_bytes;
        let mut err = UnmanagedVector::default();
        // Store and persist code in cache, with verification
        let stored = unsafe {
            store_code(
                self.cache,
                ByteSliceView::new(Some(&wasm_bytes)),
                true,
                true,
                Some(&mut err),
            )
        };
        let mut resp = LoadModuleResponse::default();
        if err.is_some() {
            let msg = String::from_utf8(err.consume().unwrap()).unwrap();
            resp.error = msg;
        } else {
            let checksum = stored.consume().unwrap();
            resp.checksum = hex::encode(&checksum);
        }
        Ok(Response::new(resp))
    }

    async fn instantiate(
        &self,
        request: Request<InstantiateRequest>,
    ) -> Result<Response<InstantiateResponse>, Status> {
        let req = request.into_inner();
        // Decode hex checksum
        let checksum = match hex::decode(&req.checksum) {
            Ok(c) => c,
            Err(e) => return Err(Status::invalid_argument(format!("invalid checksum hex: {}", e))),
        };
        // Prepare FFI views
        let checksum_view = ByteSliceView::new(Some(&checksum));
        let env_view = ByteSliceView::new(None);
        let info_view = ByteSliceView::new(None);
        let msg_view = ByteSliceView::new(Some(&req.init_msg));
        // Prepare gas report and error buffer
        let mut gas_report = GasReport { limit: req.gas_limit, remaining: 0, used_externally: 0, used_internally: 0 };
        let mut err = UnmanagedVector::default();
        // Empty DB, API, and Querier (host callbacks not implemented)
        let db = Db { gas_meter: std::ptr::null_mut(), state: std::ptr::null_mut(), vtable: DbVtable::default() };
        let api = GoApi { state: std::ptr::null(), vtable: GoApiVtable::default() };
        let querier = GoQuerier { state: std::ptr::null(), vtable: QuerierVtable::default() };
        // Call into WASM VM
        let result = vm_instantiate(
            self.cache,
            checksum_view,
            env_view,
            info_view,
            msg_view,
            db,
            api,
            querier,
            req.gas_limit,
            false,
            Some(&mut gas_report),
            Some(&mut err),
        );
        // Build response
        let mut resp = InstantiateResponse {
            contract_id: req.request_id.clone(),
            data: Vec::new(),
            gas_used: 0,
            error: String::new(),
        };
        if err.is_some() {
            resp.error = String::from_utf8(err.consume().unwrap_or_default()).unwrap_or_default();
        } else {
            resp.data = result.consume().unwrap_or_default();
            resp.gas_used = gas_report.limit.saturating_sub(gas_report.remaining);
        }
        Ok(Response::new(resp))
    }

    async fn execute(
        &self,
        request: Request<ExecuteRequest>,
    ) -> Result<Response<ExecuteResponse>, Status> {
        let _req = request.into_inner();
        // Decode checksum
        let checksum = match hex::decode(&req.contract_id) {
            Ok(c) => c,
            Err(e) => return Err(Status::invalid_argument(format!("invalid checksum hex: {}", e))),
        };
        let checksum_view = ByteSliceView::new(Some(&checksum));
        let env_view = ByteSliceView::new(None);
        let info_view = ByteSliceView::new(None);
        let msg_view = ByteSliceView::new(Some(&req.msg));
        let mut gas_report = GasReport { limit: req.gas_limit, remaining: 0, used_externally: 0, used_internally: 0 };
        let mut err = UnmanagedVector::default();
        let db = Db { gas_meter: std::ptr::null_mut(), state: std::ptr::null_mut(), vtable: DbVtable::default() };
        let api = GoApi { state: std::ptr::null(), vtable: GoApiVtable::default() };
        let querier = GoQuerier { state: std::ptr::null(), vtable: QuerierVtable::default() };
        let result = vm_execute(
            self.cache,
            checksum_view,
            env_view,
            info_view,
            msg_view,
            db,
            api,
            querier,
            req.gas_limit,
            false,
            Some(&mut gas_report),
            Some(&mut err),
        );
        let mut resp = ExecuteResponse { data: Vec::new(), gas_used: 0, error: String::new() };
        if err.is_some() {
            resp.error = String::from_utf8(err.consume().unwrap_or_default()).unwrap_or_default();
        } else {
            resp.data = result.consume().unwrap_or_default();
            resp.gas_used = gas_report.limit.saturating_sub(gas_report.remaining);
        }
        Ok(Response::new(resp))
    }

    async fn query(
        &self,
        request: Request<QueryRequest>,
    ) -> Result<Response<QueryResponse>, Status> {
        let _req = request.into_inner();
        // Decode checksum
        let checksum = match hex::decode(&req.contract_id) {
            Ok(c) => c,
            Err(e) => return Err(Status::invalid_argument(format!("invalid checksum hex: {}", e))),
        };
        let checksum_view = ByteSliceView::new(Some(&checksum));
        let env_view = ByteSliceView::new(None);
        let msg_view = ByteSliceView::new(Some(&req.query_msg));
        let mut err = UnmanagedVector::default();
        let db = Db { gas_meter: std::ptr::null_mut(), state: std::ptr::null_mut(), vtable: DbVtable::default() };
        let api = GoApi { state: std::ptr::null(), vtable: GoApiVtable::default() };
        let querier = GoQuerier { state: std::ptr::null(), vtable: QuerierVtable::default() };
        let result = vm_query(
            self.cache,
            checksum_view,
            env_view,
            msg_view,
            db,
            api,
            querier,
            req.request_id.clone(),
            Some(&mut err),
        );
        let mut resp = QueryResponse { result: Vec::new(), error: String::new() };
        if err.is_some() {
            resp.error = String::from_utf8(err.consume().unwrap_or_default()).unwrap_or_default();
        } else {
            resp.result = result.consume().unwrap_or_default();
        }
        Ok(Response::new(resp))
    }

    async fn migrate(
        &self,
        request: Request<MigrateRequest>,
    ) -> Result<Response<MigrateResponse>, Status> {
        let _req = request.into_inner();
        Ok(Response::new(MigrateResponse {
            data: Vec::new(),
            gas_used: 0,
            error: String::new(),
        }))
    }

    async fn sudo(&self, request: Request<SudoRequest>) -> Result<Response<SudoResponse>, Status> {
        let _req = request.into_inner();
        Ok(Response::new(SudoResponse {
            data: Vec::new(),
            gas_used: 0,
            error: String::new(),
        }))
    }

    async fn reply(
        &self,
        request: Request<ReplyRequest>,
    ) -> Result<Response<ReplyResponse>, Status> {
        let _req = request.into_inner();
        Ok(Response::new(ReplyResponse {
            data: Vec::new(),
            gas_used: 0,
            error: String::new(),
        }))
    }

    async fn analyze_code(
        &self,
        request: Request<AnalyzeCodeRequest>,
    ) -> Result<Response<AnalyzeCodeResponse>, Status> {
        let req = request.into_inner();
        // decode checksum
        let checksum = match hex::decode(&req.checksum) {
            Ok(c) => c,
            Err(e) => return Err(Status::invalid_argument(format!("invalid checksum: {}", e))),
        };
        let mut err = UnmanagedVector::default();
        // call libwasmvm analyze_code FFI
        let report = unsafe {
            vm_analyze_code(
                self.cache,
                ByteSliceView::new(Some(&checksum)),
                Some(&mut err),
            )
        };
        let mut resp = AnalyzeCodeResponse::default();
        if err.is_some() {
            let msg = String::from_utf8(err.consume().unwrap()).unwrap();
            resp.error = msg;
            return Ok(Response::new(resp));
        }
        // parse required_capabilities CSV
        let caps_bytes = report.required_capabilities.consume().unwrap_or_default();
        let caps_csv = String::from_utf8(caps_bytes).unwrap_or_default();
        resp.required_capabilities = if caps_csv.is_empty() {
            vec![]
        } else {
            caps_csv.split(',').map(|s| s.to_string()).collect()
        };
        resp.has_ibc_entry_points = report.has_ibc_entry_points;
        Ok(Response::new(resp))
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
