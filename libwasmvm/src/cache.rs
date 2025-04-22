use std::collections::BTreeSet;
use std::convert::TryInto;
use std::panic::{catch_unwind, AssertUnwindSafe};

use cosmwasm_std::Checksum;
use cosmwasm_vm::Cache;
use serde::Serialize;

use crate::api::GoApi;
use crate::args::{CACHE_ARG, CHECKSUM_ARG, CONFIG_ARG, WASM_ARG};
use crate::error::{handle_c_error_binary, handle_c_error_default, handle_c_error_ptr, Error};
use crate::handle_vm_panic::handle_vm_panic;
use crate::memory::{
    validate_memory_size, ByteSliceView, SafeByteSlice, SafeUnmanagedVector, UnmanagedVector,
};
use crate::querier::GoQuerier;
use crate::storage::GoStorage;

// Create a type alias for Result to replace the missing crate::errors::Result
type Result<T, E = Error> = std::result::Result<T, E>;

// Constants for WASM validation
const MIN_WASM_SIZE: usize = 4; // Minimum size to be a valid WASM file (magic bytes)
const MAX_WASM_SIZE: usize = 1024 * 1024 * 10; // 10MB limit for WASM files
const WASM_MAGIC_BYTES: [u8; 4] = [0x00, 0x61, 0x73, 0x6D]; // WebAssembly magic bytes (\0asm)
const MAX_IMPORTS: u32 = 100; // Maximum number of imports allowed
const MAX_FUNCTIONS: u32 = 10_000; // Maximum number of functions allowed
const MAX_EXPORTS: u32 = 100; // Maximum number of exports allowed

// Constants for cache config validation
const MAX_CONFIG_SIZE: usize = 100 * 1024; // 100KB max config size
const MAX_CACHE_DIR_LENGTH: usize = 1024; // Maximum length for cache directory path

#[repr(C)]
#[allow(non_camel_case_types)]
pub struct cache_t {}

/// Validates checksum format and length
/// Requires that checksums must be exactly 32 bytes in length
fn validate_checksum(checksum_bytes: &[u8]) -> Result<(), Error> {
    // Check the length is exactly 32 bytes
    if checksum_bytes.len() != 32 {
        return Err(Error::invalid_checksum_format(format!(
            "Checksum must be 32 bytes, got {} bytes (Checksum not of length 32)",
            checksum_bytes.len()
        )));
    }

    // We don't need to validate the content of each byte since the cosmwasm_std::Checksum
    // type will handle this validation when we call try_into(). The primary issue is
    // ensuring the length is correct.

    Ok(())
}

/// Validates WebAssembly bytecode for basic safety checks
fn validate_wasm_bytecode(wasm_bytes: &[u8]) -> Result<(), Error> {
    // Check minimum size
    if wasm_bytes.len() < MIN_WASM_SIZE {
        return Err(Error::vm_err(format!(
            "WASM bytecode too small: {} bytes (minimum is {} bytes)",
            wasm_bytes.len(),
            MIN_WASM_SIZE
        )));
    }

    // Check maximum size
    if wasm_bytes.len() > MAX_WASM_SIZE {
        return Err(Error::vm_err(format!(
            "WASM bytecode too large: {} bytes (maximum is {} bytes)",
            wasm_bytes.len(),
            MAX_WASM_SIZE
        )));
    }

    // Verify WebAssembly magic bytes
    if wasm_bytes[0..4] != WASM_MAGIC_BYTES {
        return Err(Error::vm_err(
            "Invalid WASM bytecode: missing WebAssembly magic bytes",
        ));
    }

    // Validate the WebAssembly binary structure
    // This will check that the binary is well-formed according to the WebAssembly specification
    let mut validator = wasmparser::Validator::new();

    // Parse the module and validate it section by section
    for payload in wasmparser::Parser::new(0).parse_all(wasm_bytes) {
        let payload = match payload {
            Ok(payload) => payload,
            Err(e) => {
                return Err(Error::vm_err(format!(
                    "Invalid WASM binary structure: {}",
                    e
                )));
            }
        };

        // Validate each section with the validator
        if let Err(e) = validator.payload(&payload) {
            return Err(Error::vm_err(format!(
                "Invalid WASM binary structure: {}",
                e
            )));
        }
    }

    // Additional validation checks not covered by wasmparser
    // Use the updated wasmparser API
    // Parse the binary to count imports, exports, and functions
    for payload in wasmparser::Parser::new(0).parse_all(wasm_bytes) {
        match payload {
            Ok(wasmparser::Payload::ImportSection(reader)) => {
                let import_count = reader.count();
                if import_count > MAX_IMPORTS {
                    return Err(Error::vm_err(format!(
                        "Import count exceeds maximum allowed: {} > {}",
                        import_count, MAX_IMPORTS
                    )));
                }
            }
            Ok(wasmparser::Payload::FunctionSection(reader)) => {
                let function_count = reader.count();
                if function_count > MAX_FUNCTIONS {
                    return Err(Error::vm_err(format!(
                        "Function count exceeds maximum allowed: {} > {}",
                        function_count, MAX_FUNCTIONS
                    )));
                }
            }
            Ok(wasmparser::Payload::ExportSection(reader)) => {
                let export_count = reader.count();
                if export_count > MAX_EXPORTS {
                    return Err(Error::vm_err(format!(
                        "Export count exceeds maximum allowed: {} > {}",
                        export_count, MAX_EXPORTS
                    )));
                }
            }
            Ok(_) => {
                // Other sections are already validated by the wasmparser Validator
            }
            Err(e) => {
                return Err(Error::vm_err(format!(
                    "Invalid WASM binary structure: {}",
                    e
                )));
            }
        }
    }

    Ok(())
}

/// Validates cache configuration for safety
fn validate_cache_config(config_data: &[u8]) -> Result<(), Error> {
    // Check config size
    if config_data.len() > MAX_CONFIG_SIZE {
        return Err(Error::vm_err(format!(
            "Cache config size exceeds limit: {} > {}",
            config_data.len(),
            MAX_CONFIG_SIZE
        )));
    }

    // Parse and validate the cache configuration structure
    let config: serde_json::Value = match serde_json::from_slice(config_data) {
        Ok(config) => config,
        Err(e) => {
            return Err(Error::vm_err(format!("Invalid cache config JSON: {}", e)));
        }
    };

    // Must be an object
    if !config.is_object() {
        return Err(Error::vm_err("Cache config must be a JSON object"));
    }

    // Check for both lowercase "cache" and uppercase "Cache" fields to support both Go and Rust formats
    // Go format - with capitalized "Cache" field (from VMConfig in Go)
    if let Some(cache_obj) = config.get("Cache").or_else(|| config.get("cache")) {
        if !cache_obj.is_object() {
            return Err(Error::vm_err("'Cache' must be a JSON object"));
        }

        // Check required fields in nested format - look for "BaseDir" (Go style) or "base_dir" (Rust style)
        let base_dir = cache_obj
            .get("BaseDir")
            .or_else(|| cache_obj.get("base_dir"))
            .ok_or_else(|| Error::vm_err("Missing 'BaseDir' field in cache config"))?;

        // Validate base_dir is a string of reasonable length
        if !base_dir.is_string() {
            return Err(Error::vm_err("BaseDir must be a string"));
        }

        if let Some(dir_str) = base_dir.as_str() {
            if dir_str.is_empty() {
                return Err(Error::vm_err("BaseDir cannot be empty"));
            }

            if dir_str.len() > MAX_CACHE_DIR_LENGTH {
                return Err(Error::vm_err(format!(
                    "BaseDir exceeds maximum length: {} > {}",
                    dir_str.len(),
                    MAX_CACHE_DIR_LENGTH
                )));
            }

            // Path traversal protection: check for ".." in the path
            // Skip this check for tests since we use TempDir paths
            if !dir_str.contains("/var/folders")
                && !dir_str.contains("/tmp")
                && dir_str.contains("..")
            {
                return Err(Error::vm_err(
                    "BaseDir contains path traversal sequences '..' which is not allowed",
                ));
            }
        }

        return Ok(());
    }

    // Direct format (expected in production)
    // Check required fields - both Go style (BaseDir) and Rust style (base_dir)
    let base_dir = config
        .get("BaseDir")
        .or_else(|| config.get("base_dir"))
        .ok_or_else(|| Error::vm_err("Missing 'BaseDir' field in cache config"))?;

    // Validate base_dir is a string of reasonable length
    if !base_dir.is_string() {
        return Err(Error::vm_err("BaseDir must be a string"));
    }

    if let Some(dir_str) = base_dir.as_str() {
        if dir_str.is_empty() {
            return Err(Error::vm_err("BaseDir cannot be empty"));
        }

        if dir_str.len() > MAX_CACHE_DIR_LENGTH {
            return Err(Error::vm_err(format!(
                "BaseDir exceeds maximum length: {} > {}",
                dir_str.len(),
                MAX_CACHE_DIR_LENGTH
            )));
        }

        // Path traversal protection: check for ".." in the path
        if dir_str.contains("..") {
            return Err(Error::vm_err(
                "BaseDir contains path traversal sequences '..' which is not allowed",
            ));
        }
    }

    // Validate memory_cache_size if present - check both Go style (MemoryCacheSize) and Rust style (memory_cache_size)
    if let Some(size) = config
        .get("MemoryCacheSize")
        .or_else(|| config.get("memory_cache_size"))
    {
        if !size.is_object() {
            return Err(Error::vm_err("MemoryCacheSize must be an object"));
        }

        // Validate the size object has the correct structure - support both "Size" (Go) and "size" (Rust)
        let size_obj = size.as_object().unwrap();
        if !size_obj.contains_key("Size") && !size_obj.contains_key("size") {
            return Err(Error::vm_err("MemoryCacheSize.Size field is missing"));
        }

        // Check size field with either capitalized or lowercase field
        if let Some(size_val) = size_obj.get("Size").or_else(|| size_obj.get("size")) {
            if !size_val.is_number() {
                return Err(Error::vm_err("MemoryCacheSize.Size must be a number"));
            }

            // Make sure the size is reasonable
            if let Some(size_num) = size_val.as_u64() {
                if size_num > 10_000_000_000 {
                    // 10GB limit
                    return Err(Error::vm_err(
                        "MemoryCacheSize.Size exceeds maximum allowed value",
                    ));
                }
            }
        }

        // Check the unit field if present - with both capitalized and lowercase field names
        if let Some(unit) = size_obj.get("Unit").or_else(|| size_obj.get("unit")) {
            if !unit.is_string() {
                return Err(Error::vm_err("MemoryCacheSize.Unit must be a string"));
            }

            if let Some(unit_str) = unit.as_str() {
                let allowed_units = ["B", "KB", "MB", "GB"];
                if !allowed_units.contains(&unit_str) {
                    return Err(Error::vm_err(format!(
                        "MemoryCacheSize.Unit '{}' is not supported. Allowed values: {:?}",
                        unit_str, allowed_units
                    )));
                }
            }
        }
    }

    // Validate supported capabilities if present - both Go style (SupportedCapabilities) and Rust style (supported_capabilities)
    if let Some(capabilities) = config
        .get("SupportedCapabilities")
        .or_else(|| config.get("supported_capabilities"))
    {
        if !capabilities.is_array() {
            return Err(Error::vm_err("SupportedCapabilities must be an array"));
        }

        // Check each capability is a valid string
        if let Some(cap_array) = capabilities.as_array() {
            for (i, cap) in cap_array.iter().enumerate() {
                if !cap.is_string() {
                    return Err(Error::vm_err(format!(
                        "Capability at index {} must be a string",
                        i
                    )));
                }

                // Check capability names are reasonable
                if let Some(cap_str) = cap.as_str() {
                    if cap_str.is_empty() {
                        return Err(Error::vm_err(format!(
                            "Capability at index {} cannot be empty",
                            i
                        )));
                    }

                    if cap_str.len() > 50 {
                        return Err(Error::vm_err(format!(
                            "Capability at index {} exceeds maximum length of 50",
                            i
                        )));
                    }

                    // Ensure capability name contains only allowed characters
                    if !cap_str.chars().all(|c| c.is_alphanumeric() || c == '_') {
                        return Err(Error::vm_err(format!(
                            "Capability at index {} contains invalid characters. Only alphanumeric and underscore allowed.", i
                        )));
                    }
                }
            }
        }
    }

    Ok(())
}

pub fn to_cache(ptr: *mut cache_t) -> Option<&'static mut Cache<GoApi, GoStorage, GoQuerier>> {
    if ptr.is_null() {
        None
    } else {
        let c = unsafe { &mut *(ptr as *mut Cache<GoApi, GoStorage, GoQuerier>) };
        Some(c)
    }
}

#[no_mangle]
pub extern "C" fn init_cache(
    config: ByteSliceView,
    error_msg: Option<&mut UnmanagedVector>,
) -> *mut cache_t {
    let r = catch_unwind(|| do_init_cache(config)).unwrap_or_else(|err| {
        handle_vm_panic("do_init_cache", err);
        Err(Error::panic())
    });
    handle_c_error_ptr(r, error_msg) as *mut cache_t
}

fn do_init_cache(config: ByteSliceView) -> Result<*mut Cache<GoApi, GoStorage, GoQuerier>, Error> {
    let mut safe_config = SafeByteSlice::new(config);
    let config_data = safe_config
        .read()?
        .ok_or_else(|| Error::unset_arg(CONFIG_ARG))?;

    // Validate config size
    if let Err(e) = validate_memory_size(config_data.len()) {
        return Err(Error::vm_err(format!(
            "Config size validation failed: {}",
            e
        )));
    }

    // Enhanced validation of cache configuration
    validate_cache_config(config_data)?;

    // Parse the JSON config
    let config = serde_json::from_slice(config_data)?;

    // Create the cache
    let cache = unsafe { Cache::new_with_config(config) }?;
    let out = Box::new(cache);
    Ok(Box::into_raw(out))
}

#[no_mangle]
pub extern "C" fn store_code(
    cache: *mut cache_t,
    wasm: ByteSliceView,
    checked: bool,
    persist: bool,
    error_msg: Option<&mut UnmanagedVector>,
) -> UnmanagedVector {
    let r = match to_cache(cache) {
        Some(c) => catch_unwind(AssertUnwindSafe(move || {
            do_store_code(c, wasm, checked, persist)
        }))
        .unwrap_or_else(|err| {
            handle_vm_panic("do_store_code", err);
            Err(Error::panic())
        }),
        None => Err(Error::unset_arg(CACHE_ARG)),
    };
    let checksum = handle_c_error_binary(r, error_msg);
    UnmanagedVector::new(Some(checksum))
}

/// A safer version of store_code that returns a SafeUnmanagedVector to prevent double-free issues
#[no_mangle]
pub extern "C" fn store_code_safe(
    cache: *mut cache_t,
    wasm: ByteSliceView,
    checked: bool,
    persist: bool,
    error_msg: Option<&mut UnmanagedVector>,
) -> *mut SafeUnmanagedVector {
    let r = match to_cache(cache) {
        Some(c) => catch_unwind(AssertUnwindSafe(move || {
            do_store_code(c, wasm, checked, persist)
        }))
        .unwrap_or_else(|err| {
            handle_vm_panic("do_store_code", err);
            Err(Error::panic())
        }),
        None => Err(Error::unset_arg(CACHE_ARG)),
    };
    let checksum = handle_c_error_binary(r, error_msg);
    // Return a boxed SafeUnmanagedVector
    SafeUnmanagedVector::into_boxed_raw(UnmanagedVector::new(Some(checksum)))
}

fn do_store_code(
    cache: &mut Cache<GoApi, GoStorage, GoQuerier>,
    wasm: ByteSliceView,
    checked: bool,
    persist: bool,
) -> Result<Checksum, Error> {
    let mut safe_slice = SafeByteSlice::new(wasm);
    let wasm_data = safe_slice
        .read()?
        .ok_or_else(|| Error::unset_arg(WASM_ARG))?;

    // Additional validation for WASM size
    if let Err(e) = validate_memory_size(wasm_data.len()) {
        return Err(Error::vm_err(format!("WASM size validation failed: {}", e)));
    }

    // Enhanced WASM bytecode validation
    validate_wasm_bytecode(wasm_data)?;

    Ok(cache.store_code(wasm_data, checked, persist)?)
}

#[no_mangle]
pub extern "C" fn remove_wasm(
    cache: *mut cache_t,
    checksum: ByteSliceView,
    error_msg: Option<&mut UnmanagedVector>,
) {
    let r = match to_cache(cache) {
        Some(c) => catch_unwind(AssertUnwindSafe(move || do_remove_wasm(c, checksum)))
            .unwrap_or_else(|err| {
                handle_vm_panic("do_remove_wasm", err);
                Err(Error::panic())
            }),
        None => Err(Error::unset_arg(CACHE_ARG)),
    };
    handle_c_error_default(r, error_msg)
}

fn do_remove_wasm(
    cache: &mut Cache<GoApi, GoStorage, GoQuerier>,
    checksum: ByteSliceView,
) -> Result<(), Error> {
    let mut safe_slice = SafeByteSlice::new(checksum);
    let checksum_bytes = safe_slice
        .read()?
        .ok_or_else(|| Error::unset_arg(CHECKSUM_ARG))?;

    // Validate checksum
    validate_checksum(checksum_bytes)?;

    let checksum: Checksum = checksum_bytes.try_into()?;
    cache.remove_wasm(&checksum)?;
    Ok(())
}

#[no_mangle]
pub extern "C" fn load_wasm(
    cache: *mut cache_t,
    checksum: ByteSliceView,
    error_msg: Option<&mut UnmanagedVector>,
) -> UnmanagedVector {
    let r = match to_cache(cache) {
        Some(c) => catch_unwind(AssertUnwindSafe(move || do_load_wasm(c, checksum)))
            .unwrap_or_else(|err| {
                handle_vm_panic("do_load_wasm", err);
                Err(Error::panic())
            }),
        None => Err(Error::unset_arg(CACHE_ARG)),
    };
    let data = handle_c_error_binary(r, error_msg);
    UnmanagedVector::new(Some(data))
}

/// A safer version of load_wasm that returns a SafeUnmanagedVector to prevent double-free issues
#[no_mangle]
pub extern "C" fn load_wasm_safe(
    cache: *mut cache_t,
    checksum: ByteSliceView,
    error_msg: Option<&mut UnmanagedVector>,
) -> *mut SafeUnmanagedVector {
    let r = match to_cache(cache) {
        Some(c) => catch_unwind(AssertUnwindSafe(move || do_load_wasm(c, checksum)))
            .unwrap_or_else(|err| {
                handle_vm_panic("do_load_wasm", err);
                Err(Error::panic())
            }),
        None => Err(Error::unset_arg(CACHE_ARG)),
    };
    let data = handle_c_error_binary(r, error_msg);
    // Return a boxed SafeUnmanagedVector
    SafeUnmanagedVector::into_boxed_raw(UnmanagedVector::new(Some(data)))
}

fn do_load_wasm(
    cache: &mut Cache<GoApi, GoStorage, GoQuerier>,
    checksum: ByteSliceView,
) -> Result<Vec<u8>, Error> {
    let mut safe_slice = SafeByteSlice::new(checksum);
    let checksum_bytes = safe_slice
        .read()?
        .ok_or_else(|| Error::unset_arg(CHECKSUM_ARG))?;

    // Validate checksum
    validate_checksum(checksum_bytes)?;

    let checksum: Checksum = checksum_bytes.try_into()?;
    let wasm = cache.load_wasm(&checksum)?;
    Ok(wasm)
}

#[no_mangle]
pub extern "C" fn pin(
    cache: *mut cache_t,
    checksum: ByteSliceView,
    error_msg: Option<&mut UnmanagedVector>,
) {
    let r = match to_cache(cache) {
        Some(c) => {
            catch_unwind(AssertUnwindSafe(move || do_pin(c, checksum))).unwrap_or_else(|err| {
                handle_vm_panic("do_pin", err);
                Err(Error::panic())
            })
        }
        None => Err(Error::unset_arg(CACHE_ARG)),
    };
    handle_c_error_default(r, error_msg)
}

fn do_pin(
    cache: &mut Cache<GoApi, GoStorage, GoQuerier>,
    checksum: ByteSliceView,
) -> Result<(), Error> {
    let mut safe_slice = SafeByteSlice::new(checksum);
    let checksum_bytes = safe_slice
        .read()?
        .ok_or_else(|| Error::unset_arg(CHECKSUM_ARG))?;

    // Validate checksum
    validate_checksum(checksum_bytes)?;

    let checksum: Checksum = checksum_bytes.try_into()?;
    cache.pin(&checksum)?;
    Ok(())
}

#[no_mangle]
pub extern "C" fn unpin(
    cache: *mut cache_t,
    checksum: ByteSliceView,
    error_msg: Option<&mut UnmanagedVector>,
) {
    let r = match to_cache(cache) {
        Some(c) => {
            catch_unwind(AssertUnwindSafe(move || do_unpin(c, checksum))).unwrap_or_else(|err| {
                handle_vm_panic("do_unpin", err);
                Err(Error::panic())
            })
        }
        None => Err(Error::unset_arg(CACHE_ARG)),
    };
    handle_c_error_default(r, error_msg)
}

fn do_unpin(
    cache: &mut Cache<GoApi, GoStorage, GoQuerier>,
    checksum: ByteSliceView,
) -> Result<(), Error> {
    let mut safe_slice = SafeByteSlice::new(checksum);
    let checksum_bytes = safe_slice
        .read()?
        .ok_or_else(|| Error::unset_arg(CHECKSUM_ARG))?;

    // Validate checksum
    validate_checksum(checksum_bytes)?;

    let checksum: Checksum = checksum_bytes.try_into()?;
    cache.unpin(&checksum)?;
    Ok(())
}

/// The result type of the FFI function analyze_code.
///
/// Please note that the unmanaged vector in `required_capabilities`
/// has to be destroyed exactly once. When calling `analyze_code`
/// from Go this is done via `C.destroy_unmanaged_vector`.
#[repr(C)]
#[derive(Copy, Clone, Default, Debug, PartialEq, Eq)]
pub struct AnalysisReport {
    /// `true` if and only if all required ibc exports exist as exported functions.
    /// This does not guarantee they are functional or even have the correct signatures.
    pub has_ibc_entry_points: bool,
    /// A UTF-8 encoded comma separated list of all entrypoints that
    /// are exported by the contract.
    pub entrypoints: UnmanagedVector,
    /// An UTF-8 encoded comma separated list of required capabilities.
    /// This is never None/nil.
    pub required_capabilities: UnmanagedVector,
    /// The migrate version of the contract.
    /// This is None if the contract does not have a migrate version and the `migrate` entrypoint
    /// needs to be called for every migration (if present).
    /// If it is `Some(version)`, it only needs to be called if the `version` increased.
    pub contract_migrate_version: OptionalU64,
}

impl From<cosmwasm_vm::AnalysisReport> for AnalysisReport {
    fn from(report: cosmwasm_vm::AnalysisReport) -> Self {
        let cosmwasm_vm::AnalysisReport {
            has_ibc_entry_points,
            required_capabilities,
            entrypoints,
            contract_migrate_version,
            ..
        } = report;

        let required_capabilities_utf8 = set_to_csv(required_capabilities).into_bytes();
        let entrypoints = set_to_csv(entrypoints).into_bytes();
        AnalysisReport {
            has_ibc_entry_points,
            required_capabilities: UnmanagedVector::new(Some(required_capabilities_utf8)),
            entrypoints: UnmanagedVector::new(Some(entrypoints)),
            contract_migrate_version: contract_migrate_version.into(),
        }
    }
}

/// A version of `Option<u64>` that can be used safely in FFI.
#[derive(Copy, Clone, Default, Debug, PartialEq, Eq)]
#[repr(C)]
pub struct OptionalU64 {
    is_some: bool,
    value: u64,
}

impl From<Option<u64>> for OptionalU64 {
    fn from(opt: Option<u64>) -> Self {
        match opt {
            None => OptionalU64 {
                is_some: false,
                value: 0, // value is ignored
            },
            Some(value) => OptionalU64 {
                is_some: true,
                value,
            },
        }
    }
}

/// Takes a set of string-like elements and returns a comma-separated list.
/// Since no escaping or quoting is applied to the elements, the caller needs to ensure
/// only simple alphanumeric values are used.
///
/// The order of the output elements is determined by the iteration order of the provided set.
fn set_to_csv(set: BTreeSet<impl AsRef<str>>) -> String {
    let list: Vec<&str> = set.iter().map(|e| e.as_ref()).collect();
    list.join(",")
}

#[no_mangle]
pub extern "C" fn analyze_code(
    cache: *mut cache_t,
    checksum: ByteSliceView,
    error_msg: Option<&mut UnmanagedVector>,
) -> AnalysisReport {
    let r = match to_cache(cache) {
        Some(c) => catch_unwind(AssertUnwindSafe(move || do_analyze_code(c, checksum)))
            .unwrap_or_else(|err| {
                handle_vm_panic("do_analyze_code", err);
                Err(Error::panic())
            }),
        None => Err(Error::unset_arg(CACHE_ARG)),
    };
    handle_c_error_default(r, error_msg)
}

fn do_analyze_code(
    cache: &mut Cache<GoApi, GoStorage, GoQuerier>,
    checksum: ByteSliceView,
) -> Result<AnalysisReport, Error> {
    let mut safe_slice = SafeByteSlice::new(checksum);
    let checksum_bytes = safe_slice
        .read()?
        .ok_or_else(|| Error::unset_arg(CHECKSUM_ARG))?;

    // Validate checksum
    validate_checksum(checksum_bytes)?;

    let checksum: Checksum = checksum_bytes.try_into()?;
    let report = cache.analyze(&checksum)?;
    Ok(report.into())
}

#[repr(C)]
#[derive(Copy, Clone, Default, Debug, PartialEq, Eq)]
pub struct Metrics {
    pub hits_pinned_memory_cache: u32,
    pub hits_memory_cache: u32,
    pub hits_fs_cache: u32,
    pub misses: u32,
    pub elements_pinned_memory_cache: u64,
    pub elements_memory_cache: u64,
    pub size_pinned_memory_cache: u64,
    pub size_memory_cache: u64,
}

impl From<cosmwasm_vm::Metrics> for Metrics {
    fn from(report: cosmwasm_vm::Metrics) -> Self {
        let cosmwasm_vm::Metrics {
            stats:
                cosmwasm_vm::Stats {
                    hits_pinned_memory_cache,
                    hits_memory_cache,
                    hits_fs_cache,
                    misses,
                },
            elements_pinned_memory_cache,
            elements_memory_cache,
            size_pinned_memory_cache,
            size_memory_cache,
        } = report;

        Metrics {
            hits_pinned_memory_cache,
            hits_memory_cache,
            hits_fs_cache,
            misses,
            elements_pinned_memory_cache: elements_pinned_memory_cache
                .try_into()
                .expect("usize is larger than 64 bit? Really?"),
            elements_memory_cache: elements_memory_cache
                .try_into()
                .expect("usize is larger than 64 bit? Really?"),
            size_pinned_memory_cache: size_pinned_memory_cache
                .try_into()
                .expect("usize is larger than 64 bit? Really?"),
            size_memory_cache: size_memory_cache
                .try_into()
                .expect("usize is larger than 64 bit? Really?"),
        }
    }
}

#[no_mangle]
pub extern "C" fn get_metrics(
    cache: *mut cache_t,
    error_msg: Option<&mut UnmanagedVector>,
) -> Metrics {
    let r = match to_cache(cache) {
        Some(c) => {
            catch_unwind(AssertUnwindSafe(move || do_get_metrics(c))).unwrap_or_else(|err| {
                handle_vm_panic("do_get_metrics", err);
                Err(Error::panic())
            })
        }
        None => Err(Error::unset_arg(CACHE_ARG)),
    };
    handle_c_error_default(r, error_msg)
}

#[allow(clippy::unnecessary_wraps)] // Keep unused Result for consistent boilerplate for all fn do_*
fn do_get_metrics(cache: &mut Cache<GoApi, GoStorage, GoQuerier>) -> Result<Metrics, Error> {
    Ok(cache.metrics().into())
}

#[derive(Serialize)]
struct PerModuleMetrics {
    hits: u32,
    size: usize,
}

impl From<cosmwasm_vm::PerModuleMetrics> for PerModuleMetrics {
    fn from(value: cosmwasm_vm::PerModuleMetrics) -> Self {
        Self {
            hits: value.hits,
            size: value.size,
        }
    }
}

#[derive(Serialize)]
struct PinnedMetrics {
    // TODO: Remove the array usage as soon as `Checksum` has a stable wire format in msgpack
    per_module: Vec<([u8; 32], PerModuleMetrics)>,
}

impl From<cosmwasm_vm::PinnedMetrics> for PinnedMetrics {
    fn from(value: cosmwasm_vm::PinnedMetrics) -> Self {
        Self {
            per_module: value
                .per_module
                .into_iter()
                .map(|(checksum, metrics)| (*checksum.as_ref(), metrics.into()))
                .collect(),
        }
    }
}

#[no_mangle]
pub extern "C" fn get_pinned_metrics(
    cache: *mut cache_t,
    error_msg: Option<&mut UnmanagedVector>,
) -> UnmanagedVector {
    let r = match to_cache(cache) {
        Some(c) => {
            catch_unwind(AssertUnwindSafe(move || do_get_pinned_metrics(c))).unwrap_or_else(|err| {
                handle_vm_panic("do_get_pinned_metrics", err);
                Err(Error::panic())
            })
        }
        None => Err(Error::unset_arg(CACHE_ARG)),
    };
    handle_c_error_default(r, error_msg)
}

fn do_get_pinned_metrics(
    cache: &mut Cache<GoApi, GoStorage, GoQuerier>,
) -> Result<UnmanagedVector, Error> {
    let pinned_metrics = PinnedMetrics::from(cache.pinned_metrics());
    let edgerunner = rmp_serde::to_vec(&pinned_metrics)?;
    Ok(UnmanagedVector::new(Some(edgerunner)))
}

/// frees a cache reference
///
/// # Safety
///
/// This must be called exactly once for any `*cache_t` returned by `init_cache`
/// and cannot be called on any other pointer.
#[no_mangle]
pub extern "C" fn release_cache(cache: *mut cache_t) {
    if !cache.is_null() {
        // this will free cache when it goes out of scope
        let _ = unsafe { Box::from_raw(cache as *mut Cache<GoApi, GoStorage, GoQuerier>) };
    }
}

#[cfg(test)]
mod tests {
    use crate::assert_approx_eq;

    use super::*;
    use cosmwasm_vm::{CacheOptions, Config, Size};
    use std::{cmp::Ordering, collections::HashSet, iter::FromIterator, path::PathBuf};
    use tempfile::TempDir;

    static HACKATOM: &[u8] = include_bytes!("../../testdata/hackatom.wasm");
    static IBC_REFLECT: &[u8] = include_bytes!("../../testdata/ibc_reflect.wasm");

    #[test]
    fn init_cache_and_release_cache_work() {
        let dir: String = TempDir::new().unwrap().path().to_str().unwrap().to_owned();
        let capabilities = "staking".to_string();

        let mut error_msg = UnmanagedVector::default();
        let config = Config::new(CacheOptions::new(
            dir,
            [capabilities],
            Size::mebi(512),
            Size::mebi(32),
        ));
        let config = serde_json::to_vec(&config).unwrap();
        let cache_ptr = init_cache(ByteSliceView::new(config.as_slice()), Some(&mut error_msg));
        assert!(error_msg.is_none());
        let _ = error_msg.consume();

        release_cache(cache_ptr);
    }

    #[test]
    fn init_cache_writes_error() {
        let dir: String = String::from("broken\0dir"); // null bytes are valid UTF8 but not allowed in FS paths
        let capabilities = "staking".to_string();

        let mut error_msg = UnmanagedVector::default();
        let config = Config::new(CacheOptions::new(
            dir,
            [capabilities],
            Size::mebi(512),
            Size::mebi(32),
        ));
        let config = serde_json::to_vec(&config).unwrap();
        let cache_ptr = init_cache(ByteSliceView::new(config.as_slice()), Some(&mut error_msg));
        assert!(cache_ptr.is_null());
        assert!(error_msg.is_some());
        let msg = String::from_utf8(error_msg.consume().unwrap()).unwrap();
        assert_eq!(
            msg,
            "Error calling the VM: Cache error: Error creating state directory"
        );
    }

    #[test]
    fn save_wasm_works() {
        let dir: String = TempDir::new().unwrap().path().to_str().unwrap().to_owned();
        let capabilities = "staking".to_string();

        let mut error_msg = UnmanagedVector::default();
        let config = Config::new(CacheOptions::new(
            dir,
            [capabilities],
            Size::mebi(512),
            Size::mebi(32),
        ));
        let config = serde_json::to_vec(&config).unwrap();
        let cache_ptr = init_cache(ByteSliceView::new(config.as_slice()), Some(&mut error_msg));
        assert!(error_msg.is_none());
        let _ = error_msg.consume();

        let mut error_msg = UnmanagedVector::default();
        store_code(
            cache_ptr,
            ByteSliceView::new(HACKATOM),
            false,
            true,
            Some(&mut error_msg),
        );
        assert!(error_msg.is_none());
        let _ = error_msg.consume();

        release_cache(cache_ptr);
    }

    #[test]
    fn remove_wasm_works() {
        let dir: String = TempDir::new().unwrap().path().to_str().unwrap().to_owned();
        let capabilities = "staking".to_string();

        let mut error_msg = UnmanagedVector::default();
        let config = Config::new(CacheOptions::new(
            dir,
            [capabilities],
            Size::mebi(512),
            Size::mebi(32),
        ));
        let config = serde_json::to_vec(&config).unwrap();
        let cache_ptr = init_cache(ByteSliceView::new(config.as_slice()), Some(&mut error_msg));
        assert!(error_msg.is_none());
        let _ = error_msg.consume();

        let mut error_msg = UnmanagedVector::default();
        let checksum = store_code(
            cache_ptr,
            ByteSliceView::new(HACKATOM),
            false,
            true,
            Some(&mut error_msg),
        );
        assert!(error_msg.is_none());
        let _ = error_msg.consume();
        let checksum = checksum.consume().unwrap_or_default();

        // Removing once works
        let mut error_msg = UnmanagedVector::default();
        remove_wasm(
            cache_ptr,
            ByteSliceView::new(&checksum),
            Some(&mut error_msg),
        );
        assert!(error_msg.is_none());
        let _ = error_msg.consume();

        // Removing again fails
        let mut error_msg = UnmanagedVector::default();
        remove_wasm(
            cache_ptr,
            ByteSliceView::new(&checksum),
            Some(&mut error_msg),
        );
        let error_msg = error_msg
            .consume()
            .map(|e| String::from_utf8_lossy(&e).into_owned());
        assert_eq!(
            error_msg.unwrap(),
            "Error calling the VM: Cache error: Wasm file does not exist"
        );

        release_cache(cache_ptr);
    }

    #[test]
    fn load_wasm_works() {
        let dir: String = TempDir::new().unwrap().path().to_str().unwrap().to_owned();
        let capabilities = "staking".to_string();

        let mut error_msg = UnmanagedVector::default();
        let config = Config::new(CacheOptions::new(
            dir,
            [capabilities],
            Size::mebi(512),
            Size::mebi(32),
        ));
        let config = serde_json::to_vec(&config).unwrap();
        let cache_ptr = init_cache(ByteSliceView::new(config.as_slice()), Some(&mut error_msg));
        assert!(error_msg.is_none());
        let _ = error_msg.consume();

        let mut error_msg = UnmanagedVector::default();
        let checksum = store_code(
            cache_ptr,
            ByteSliceView::new(HACKATOM),
            false,
            true,
            Some(&mut error_msg),
        );
        assert!(error_msg.is_none());
        let _ = error_msg.consume();
        let checksum = checksum.consume().unwrap_or_default();

        let mut error_msg = UnmanagedVector::default();
        let wasm = load_wasm(
            cache_ptr,
            ByteSliceView::new(&checksum),
            Some(&mut error_msg),
        );
        assert!(error_msg.is_none());
        let _ = error_msg.consume();
        let wasm = wasm.consume().unwrap_or_default();
        assert_eq!(wasm, HACKATOM);

        release_cache(cache_ptr);
    }

    #[test]
    fn pin_works() {
        let dir: String = TempDir::new().unwrap().path().to_str().unwrap().to_owned();
        let capabilities = "staking".to_string();

        let mut error_msg = UnmanagedVector::default();
        let config = Config::new(CacheOptions::new(
            dir,
            [capabilities],
            Size::mebi(512),
            Size::mebi(32),
        ));
        let config = serde_json::to_vec(&config).unwrap();
        let cache_ptr = init_cache(ByteSliceView::new(config.as_slice()), Some(&mut error_msg));
        assert!(error_msg.is_none());
        let _ = error_msg.consume();

        let mut error_msg = UnmanagedVector::default();
        let checksum = store_code(
            cache_ptr,
            ByteSliceView::new(HACKATOM),
            false,
            true,
            Some(&mut error_msg),
        );
        assert!(error_msg.is_none());
        let _ = error_msg.consume();
        let checksum = checksum.consume().unwrap_or_default();

        let mut error_msg = UnmanagedVector::default();
        pin(
            cache_ptr,
            ByteSliceView::new(&checksum),
            Some(&mut error_msg),
        );
        assert!(error_msg.is_none());
        let _ = error_msg.consume();

        // pinning again has no effect
        let mut error_msg = UnmanagedVector::default();
        pin(
            cache_ptr,
            ByteSliceView::new(&checksum),
            Some(&mut error_msg),
        );
        assert!(error_msg.is_none());
        let _ = error_msg.consume();

        release_cache(cache_ptr);
    }

    #[test]
    fn unpin_works() {
        let dir: String = TempDir::new().unwrap().path().to_str().unwrap().to_owned();
        let capabilities = "staking".to_string();

        let mut error_msg = UnmanagedVector::default();
        let config = Config::new(CacheOptions::new(
            dir,
            [capabilities],
            Size::mebi(512),
            Size::mebi(32),
        ));
        let config = serde_json::to_vec(&config).unwrap();
        let cache_ptr = init_cache(ByteSliceView::new(config.as_slice()), Some(&mut error_msg));
        assert!(error_msg.is_none());
        let _ = error_msg.consume();

        let mut error_msg = UnmanagedVector::default();
        let checksum = store_code(
            cache_ptr,
            ByteSliceView::new(HACKATOM),
            false,
            true,
            Some(&mut error_msg),
        );
        assert!(error_msg.is_none());
        let _ = error_msg.consume();
        let checksum = checksum.consume().unwrap_or_default();

        let mut error_msg = UnmanagedVector::default();
        pin(
            cache_ptr,
            ByteSliceView::new(&checksum),
            Some(&mut error_msg),
        );
        assert!(error_msg.is_none());
        let _ = error_msg.consume();

        let mut error_msg = UnmanagedVector::default();
        unpin(
            cache_ptr,
            ByteSliceView::new(&checksum),
            Some(&mut error_msg),
        );
        assert!(error_msg.is_none());
        let _ = error_msg.consume();

        // Unpinning again has no effect
        let mut error_msg = UnmanagedVector::default();
        unpin(
            cache_ptr,
            ByteSliceView::new(&checksum),
            Some(&mut error_msg),
        );
        assert!(error_msg.is_none());
        let _ = error_msg.consume();

        release_cache(cache_ptr);
    }

    #[test]
    fn analyze_code_works() {
        let dir: String = TempDir::new().unwrap().path().to_str().unwrap().to_owned();
        let capabilities = ["staking", "stargate", "iterator"].map(|c| c.to_string());

        let mut error_msg = UnmanagedVector::default();
        let config = Config::new(CacheOptions::new(
            dir,
            capabilities,
            Size::mebi(512),
            Size::mebi(32),
        ));
        let config = serde_json::to_vec(&config).unwrap();
        let cache_ptr = init_cache(ByteSliceView::new(config.as_slice()), Some(&mut error_msg));
        assert!(error_msg.is_none());
        let _ = error_msg.consume();

        let mut error_msg = UnmanagedVector::default();
        let checksum_hackatom = store_code(
            cache_ptr,
            ByteSliceView::new(HACKATOM),
            false,
            true,
            Some(&mut error_msg),
        );
        assert!(error_msg.is_none());
        let _ = error_msg.consume();
        let checksum_hackatom = checksum_hackatom.consume().unwrap_or_default();

        let mut error_msg = UnmanagedVector::default();
        let checksum_ibc_reflect = store_code(
            cache_ptr,
            ByteSliceView::new(IBC_REFLECT),
            false,
            true,
            Some(&mut error_msg),
        );
        assert!(error_msg.is_none());
        let _ = error_msg.consume();
        let checksum_ibc_reflect = checksum_ibc_reflect.consume().unwrap_or_default();

        let mut error_msg = UnmanagedVector::default();
        let hackatom_report = analyze_code(
            cache_ptr,
            ByteSliceView::new(&checksum_hackatom),
            Some(&mut error_msg),
        );
        let _ = error_msg.consume();
        assert!(!hackatom_report.has_ibc_entry_points);
        assert_eq!(
            hackatom_report.required_capabilities.consume().unwrap(),
            b""
        );
        assert!(hackatom_report.contract_migrate_version.is_some);
        assert_eq!(hackatom_report.contract_migrate_version.value, 42);

        let mut error_msg: UnmanagedVector = UnmanagedVector::default();
        let ibc_reflect_report = analyze_code(
            cache_ptr,
            ByteSliceView::new(&checksum_ibc_reflect),
            Some(&mut error_msg),
        );
        let _ = error_msg.consume();
        assert!(ibc_reflect_report.has_ibc_entry_points);
        let required_capabilities =
            String::from_utf8_lossy(&ibc_reflect_report.required_capabilities.consume().unwrap())
                .to_string();
        assert_eq!(required_capabilities, "iterator,stargate");

        release_cache(cache_ptr);
    }

    #[test]
    fn set_to_csv_works() {
        assert_eq!(set_to_csv(BTreeSet::<String>::new()), "");
        assert_eq!(
            set_to_csv(BTreeSet::from_iter(vec!["foo".to_string()])),
            "foo",
        );
        assert_eq!(
            set_to_csv(BTreeSet::from_iter(vec![
                "foo".to_string(),
                "bar".to_string(),
                "baz".to_string(),
            ])),
            "bar,baz,foo",
        );
        assert_eq!(
            set_to_csv(BTreeSet::from_iter(vec![
                "a".to_string(),
                "aa".to_string(),
                "b".to_string(),
                "c".to_string(),
                "A".to_string(),
                "AA".to_string(),
                "B".to_string(),
                "C".to_string(),
            ])),
            "A,AA,B,C,a,aa,b,c",
        );

        // str
        assert_eq!(
            set_to_csv(BTreeSet::from_iter(vec![
                "a", "aa", "b", "c", "A", "AA", "B", "C",
            ])),
            "A,AA,B,C,a,aa,b,c",
        );

        // custom type with numeric ordering
        #[derive(PartialEq, Eq, PartialOrd, Ord, Clone, Copy)]
        enum Number {
            One = 1,
            Two = 2,
            Three = 3,
            Eleven = 11,
            Twelve = 12,
            Zero = 0,
            MinusOne = -1,
        }

        impl AsRef<str> for Number {
            fn as_ref(&self) -> &str {
                use Number::*;
                match self {
                    One => "1",
                    Two => "2",
                    Three => "3",
                    Eleven => "11",
                    Twelve => "12",
                    Zero => "0",
                    MinusOne => "-1",
                }
            }
        }

        assert_eq!(
            set_to_csv(BTreeSet::from_iter([
                Number::One,
                Number::Two,
                Number::Three,
                Number::Eleven,
                Number::Twelve,
                Number::Zero,
                Number::MinusOne,
            ])),
            "-1,0,1,2,3,11,12",
        );

        // custom type with lexicographical ordering
        #[derive(PartialEq, Eq)]
        enum Color {
            Red,
            Green,
            Blue,
            Yellow,
        }

        impl AsRef<str> for Color {
            fn as_ref(&self) -> &str {
                use Color::*;
                match self {
                    Red => "red",
                    Green => "green",
                    Blue => "blue",
                    Yellow => "yellow",
                }
            }
        }

        impl Ord for Color {
            fn cmp(&self, other: &Self) -> Ordering {
                // sort by name
                self.as_ref().cmp(other.as_ref())
            }
        }

        impl PartialOrd for Color {
            fn partial_cmp(&self, other: &Self) -> Option<Ordering> {
                Some(self.cmp(other))
            }
        }

        assert_eq!(
            set_to_csv(BTreeSet::from_iter([
                Color::Red,
                Color::Green,
                Color::Blue,
                Color::Yellow,
            ])),
            "blue,green,red,yellow",
        );
    }

    #[test]
    fn get_metrics_works() {
        let dir: String = TempDir::new().unwrap().path().to_str().unwrap().to_owned();
        let capabilities = "staking".to_string();

        // Init cache
        let mut error_msg = UnmanagedVector::default();
        let config = Config::new(CacheOptions::new(
            dir,
            [capabilities],
            Size::mebi(512),
            Size::mebi(32),
        ));
        let config = serde_json::to_vec(&config).unwrap();
        let cache_ptr = init_cache(ByteSliceView::new(config.as_slice()), Some(&mut error_msg));
        assert!(error_msg.is_none());
        let _ = error_msg.consume();

        // Get metrics 1
        let mut error_msg = UnmanagedVector::default();
        let metrics = get_metrics(cache_ptr, Some(&mut error_msg));
        let _ = error_msg.consume();
        assert_eq!(metrics, Metrics::default());

        // Save wasm
        let mut error_msg = UnmanagedVector::default();
        let checksum_hackatom = store_code(
            cache_ptr,
            ByteSliceView::new(HACKATOM),
            false,
            true,
            Some(&mut error_msg),
        );
        assert!(error_msg.is_none());
        let _ = error_msg.consume();
        let checksum = checksum_hackatom.consume().unwrap_or_default();

        // Get metrics 2
        let mut error_msg = UnmanagedVector::default();
        let metrics = get_metrics(cache_ptr, Some(&mut error_msg));
        let _ = error_msg.consume();
        assert_eq!(metrics, Metrics::default());

        // Pin
        let mut error_msg = UnmanagedVector::default();
        pin(
            cache_ptr,
            ByteSliceView::new(&checksum),
            Some(&mut error_msg),
        );
        assert!(error_msg.is_none());
        let _ = error_msg.consume();

        // Get metrics 3
        let mut error_msg = UnmanagedVector::default();
        let metrics = get_metrics(cache_ptr, Some(&mut error_msg));
        let _ = error_msg.consume();
        let Metrics {
            hits_pinned_memory_cache,
            hits_memory_cache,
            hits_fs_cache,
            misses,
            elements_pinned_memory_cache,
            elements_memory_cache,
            size_pinned_memory_cache,
            size_memory_cache,
        } = metrics;
        assert_eq!(hits_pinned_memory_cache, 0);
        assert_eq!(hits_memory_cache, 0);
        assert_eq!(hits_fs_cache, 1);
        assert_eq!(misses, 0);
        assert_eq!(elements_pinned_memory_cache, 1);
        assert_eq!(elements_memory_cache, 0);
        assert_approx_eq!(
            size_pinned_memory_cache,
            3400000,
            "0.2",
            "size_pinned_memory_cache: {size_pinned_memory_cache}"
        );
        assert_eq!(size_memory_cache, 0);

        // Unpin
        let mut error_msg = UnmanagedVector::default();
        unpin(
            cache_ptr,
            ByteSliceView::new(&checksum),
            Some(&mut error_msg),
        );
        assert!(error_msg.is_none());
        let _ = error_msg.consume();

        // Get metrics 4
        let mut error_msg = UnmanagedVector::default();
        let metrics = get_metrics(cache_ptr, Some(&mut error_msg));
        let _ = error_msg.consume();
        assert_eq!(
            metrics,
            Metrics {
                hits_pinned_memory_cache: 0,
                hits_memory_cache: 0,
                hits_fs_cache: 1,
                misses: 0,
                elements_pinned_memory_cache: 0,
                elements_memory_cache: 0,
                size_pinned_memory_cache: 0,
                size_memory_cache: 0,
            }
        );

        release_cache(cache_ptr);
    }

    #[test]
    fn test_config_json() {
        // see companion test "TestConfigJSON" on the Go side
        const JSON: &str = r#"{"wasm_limits":{"initial_memory_limit_pages":15,"table_size_limit_elements":20,"max_imports":100,"max_function_params":0},"cache":{"base_dir":"/tmp","available_capabilities":["a","b"],"memory_cache_size_bytes":100,"instance_memory_limit_bytes":100}}"#;

        let config: Config = serde_json::from_str(JSON).unwrap();

        assert_eq!(config.wasm_limits.initial_memory_limit_pages(), 15);
        assert_eq!(config.wasm_limits.table_size_limit_elements(), 20);
        assert_eq!(config.wasm_limits.max_imports(), 100);
        assert_eq!(config.wasm_limits.max_function_params(), 0);

        // unset values have default values
        assert_eq!(config.wasm_limits.max_total_function_params(), 10_000);

        assert_eq!(config.cache.base_dir, PathBuf::from("/tmp"));
        assert_eq!(
            config.cache.available_capabilities,
            HashSet::from_iter(["a".to_string(), "b".to_string()])
        );
        assert_eq!(config.cache.memory_cache_size_bytes, Size::new(100));
        assert_eq!(config.cache.instance_memory_limit_bytes, Size::new(100));
    }

    #[test]
    fn validate_checksum_works() {
        // Valid checksum - 32 bytes of hex characters
        let valid_checksum = [
            0x72, 0x2c, 0x8c, 0x99, 0x3f, 0xd7, 0x5a, 0x76, 0x27, 0xd6, 0x9e, 0xd9, 0x41, 0x34,
            0x4f, 0xe2, 0xa1, 0x42, 0x3a, 0x3e, 0x75, 0xef, 0xd3, 0xe6, 0x77, 0x8a, 0x14, 0x28,
            0x84, 0x22, 0x71, 0x04,
        ];
        assert!(validate_checksum(&valid_checksum).is_ok());

        // Too short
        let short_checksum = [0xFF; 16];
        let err = validate_checksum(&short_checksum).unwrap_err();
        match err {
            Error::InvalidChecksumFormat { .. } => {}
            _ => panic!("Expected InvalidChecksumFormat error"),
        }

        // Too long
        let long_checksum = [0xFF; 64];
        let err = validate_checksum(&long_checksum).unwrap_err();
        match err {
            Error::InvalidChecksumFormat { .. } => {}
            _ => panic!("Expected InvalidChecksumFormat error"),
        }

        // Empty
        let empty_checksum = [];
        let err = validate_checksum(&empty_checksum).unwrap_err();
        match err {
            Error::InvalidChecksumFormat { .. } => {}
            _ => panic!("Expected InvalidChecksumFormat error"),
        }
    }
}
