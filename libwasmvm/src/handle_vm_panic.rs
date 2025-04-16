use std::any::Any;

/// A function to process cases in which the VM panics.
///
/// We want to provide as much debug information as possible
/// as those cases are not expated to happen during healthy operations.
pub fn handle_vm_panic(what: &str, err: Box<dyn Any + Send + 'static>) {
    let err = match (err.downcast_ref::<&str>(), err.downcast_ref::<String>()) {
        (Some(str), ..) => *str,
        (.., Some(str)) => str,
        (None, None) => "[unusable panic payload]",
    };

    eprintln!("Panic in {what}:");
    eprintln!("{err:?}");
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

    #[test]
    fn can_handle_random_payloads() {
        fn nice_try() {
            std::panic::panic_any(());
        }
        let err = catch_unwind(nice_try).unwrap_err();
        handle_vm_panic("nice_try", err);
    }
}
