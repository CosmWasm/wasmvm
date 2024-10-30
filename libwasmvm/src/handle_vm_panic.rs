/// A function to process cases in which the VM panics.
///
/// We want to provide as much debug information as possible
/// as those cases are not expated to happen during healthy operations.
pub fn handle_vm_panic() {
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

    #[test]
    fn handle_vm_panic_works() {
        handle_vm_panic();
    }
}
