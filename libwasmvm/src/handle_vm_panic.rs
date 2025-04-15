use std::any::Any;

/// A function to process cases in which the VM panics.
///
/// We want to provide as much debug information as possible
/// as those cases are not expected to happen during healthy operations.
pub fn handle_vm_panic(what: &str, err: Box<dyn Any + Send + 'static>) {
    eprintln!("Panic in {what}:");
    eprintln!("{err:?}"); // Does not show useful information, see https://users.rust-lang.org/t/return-value-from-catch-unwind-is-a-useless-any/89134/6
    eprintln!(
        "This indicates a panic in during the operations of libwasmvm/cosmwasm-vm.
Such panics must not happen and are considered bugs. If you see this in any real-world or
close-to-real-world usage of wasmvm, please consider filing a security report,
no matter if it can be abused or not:
(https://github.com/CosmWasm/advisories/blob/main/SECURITY.md#reporting-a-vulnerability).
Thank you for your help keeping CosmWasm safe and secure 💚"
    );
}

#[cfg(test)]
mod tests {
    use super::*;

    use std::panic::catch_unwind;

    #[test]
    fn handle_vm_panic_works() {
        fn nice_try() {
            panic!("oh no!");
        }
        let err = catch_unwind(nice_try).unwrap_err();
        handle_vm_panic("nice_try", err);
    }
}
