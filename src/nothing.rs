/// A "no data" type that works with handle_c_error
pub struct Nothing;

impl From<()> for Nothing {
    fn from(_original: ()) -> Self {
        Nothing
    }
}

impl From<Nothing> for Vec<u8> {
    fn from(_original: Nothing) -> Self {
        Vec::new() // does not allocate as long as capacity is zero
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn convert_unit_to_nothing() {
        let _var = Nothing::from(());
    }

    #[test]
    fn convert_nothing_to_vector() {
        let binary = Vec::<u8>::from(Nothing);
        assert_eq!(binary, Vec::<u8>::new());
    }
}
