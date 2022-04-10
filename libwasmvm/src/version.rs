/// Returns a version number of this library in the form 0xEEEEXXXXYYYYZZZZ
/// with two-byte hexadecimal values EEEE, XXXX, YYYY, ZZZZ from 0 to 65535 each.
///
/// EEEE represents the error value with 0 meaning no error.
/// XXXX is the major version, YYYY is the minor version and ZZZZ is the patch version.
///
/// ## Examples
///
/// The version number can be decomposed like this:
///
/// ```
/// # use wasmvm::version_number;
/// let version = version_number();
/// let patch = version >> 0 & 0xFFFF;
/// let minor = version >> 16 & 0xFFFF;
/// let major = version >> 32 & 0xFFFF;
/// let error = version >> 48 & 0xFFFF;
/// assert_eq!(error, 0);
/// assert_eq!(major, 1);
/// assert!(minor < 70);
/// assert!(patch < 70);
/// ```
///
/// And compared like this:
///
/// ```
/// # use wasmvm::{make_version_number, version_number};
/// let min_version = make_version_number(0, 17, 25);
/// let version = version_number();
/// let error = version >> 48 & 0xFFFF;
/// assert_eq!(error, 0);
/// assert!(version >= min_version);
/// ```
#[no_mangle]
pub extern "C" fn version_number() -> u64 {
    match version_number_impl() {
        Ok([major, minor, patch]) => make_version_number(major, minor, patch),
        Err(err) => {
            let error = err as u16;
            let [b0, b1] = error.to_be_bytes();
            u64::from_be_bytes([b0, b1, 0, 0, 0, 0, 0, 0])
        }
    }
}

/// Creates a version number from the three components major.minor.patch.
///
/// See [`version_number`] for more details.
pub fn make_version_number(major: u16, minor: u16, patch: u16) -> u64 {
    let [b2, b3] = major.to_be_bytes();
    let [b4, b5] = minor.to_be_bytes();
    let [b6, b7] = patch.to_be_bytes();
    u64::from_be_bytes([0, 0, b2, b3, b4, b5, b6, b7])
}

// Errors will be converted to u16 and passed over the FFI
// using big endian encoding.
enum VersionNumberError {
    CannotParseMajor = 1,
    CannotParseMinor,
    CannotParsePatch,
}

fn version_number_impl() -> Result<[u16; 3], VersionNumberError> {
    let major: u16 = env!("CARGO_PKG_VERSION_MAJOR")
        .parse()
        .map_err(|_| VersionNumberError::CannotParseMajor)?;
    let minor: u16 = env!("CARGO_PKG_VERSION_MINOR")
        .parse()
        .map_err(|_| VersionNumberError::CannotParseMinor)?;
    let patch: u16 = env!("CARGO_PKG_VERSION_PATCH")
        .parse()
        .map_err(|_| VersionNumberError::CannotParsePatch)?;
    Ok([major, minor, patch])
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn version_number_works() {
        let version = version_number();
        let patch = version >> 0 & 0xFFFF;
        let minor = version >> 16 & 0xFFFF;
        let major = version >> 32 & 0xFFFF;
        let error = version >> 48 & 0xFFFF;
        assert_eq!(error, 0);
        assert_eq!(major, 1);
        assert!(minor < 70);
        assert!(patch < 70);
    }
}
