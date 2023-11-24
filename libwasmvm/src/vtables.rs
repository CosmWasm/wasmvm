/// Vtables are a collection of free function. Those functions are created in Go and
/// then assigned via function pointers that Rust can call. The functions do not
/// change during the lifetime of the object.
///
/// Since the functions are free, a single function is used for multiple instances.
/// In fact, in Go we create a single global vtable variable holding and owning the
/// function the vtables in Rust point to.
///
/// The `Vtable` trait is created to find those vtables troughout the codebase.
///
/// ## Nullability
///
/// Since all functions are represented as a function pointer, they are naturally
/// nullable. I.e. in Go we can always set them to `nil`. For this reason the functions
/// are all wrapped in Options, e.g.
///
/// ```
/// # use wasmvm::UnmanagedVector;
/// # struct iterator_t;
/// # struct gas_meter_t;
/// pub struct Iterator_vtable {
///     pub next: Option<
///         extern "C" fn(
///             iterator_t,
///             *mut gas_meter_t,
///             *mut u64,
///             *mut UnmanagedVector, // key output
///             *mut UnmanagedVector, // value output
///             *mut UnmanagedVector, // error message output
///         ) -> i32,
///     >,
///     // ...
/// }
/// ```
///
/// Since all vtable fields are nullable, we can easily demand them
/// to have a Default implementation. This sometimes is handy when a
/// type is created in Rust and then filled in Go.
///
/// Missing vtable fields always indicate a lifecycle bug in wasmvm and
/// should be treated with a panic.
pub trait Vtable: Default {}
