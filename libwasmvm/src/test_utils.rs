/// Asserts that two expressions are approximately equal to each other (using [`PartialOrd`]).
///
/// The ratio argument defines how wide a range of values we accept, and is applied
/// to the **second** argument.
///
/// On panic, this macro will print the values of the expressions with their
/// debug representations, and info about the acceptable range.
///
/// Like [`assert!`], this macro has a second form, where a custom
/// panic message can be provided.
///
/// # Examples
///
/// ```
/// let a = 3;
/// let b = 1 + 2;
/// assert_eq!(a, b);
///
/// assert_eq!(a, b, "we are testing addition with {} and {}", a, b);
/// ```
#[macro_export]
macro_rules! assert_approx_eq {
    ($left:expr, $right:expr, $ratio:expr $(,)?) => {{
        use cosmwasm_std::{Decimal, Uint128};
        use std::str::FromStr as _;

        let left = Uint128::from($left);
        let right = Uint128::from($right);
        let ratio = Decimal::from_str($ratio).unwrap();

        let delta = Uint128::from($right) * ratio;
        let lower_bound = right - delta;
        let upper_bound = right + delta;

        if !(left >= lower_bound && left <= upper_bound) {
            panic!(
                "{} doesn't belong to the expected range of {} - {}",
                left, lower_bound, upper_bound
            );
        }
    }};
    ($left:expr, $right:expr, $ratio:expr, $($args:tt)+) => {{
        use cosmwasm_std::{Decimal, Uint128};
        use std::str::FromStr as _;

        let left = Uint128::from($left);
        let right = Uint128::from($right);
        let ratio = Decimal::from_str($ratio).unwrap();

        let delta = Uint128::from($right) * ratio;
        let lower_bound = right - delta;
        let upper_bound = right + delta;

        if !(left >= lower_bound && left <= upper_bound) {
            panic!(
                "{} doesn't belong to the expected range of {} - {}: {}",
                left, lower_bound, upper_bound, format!($($args)*)
            );
        }
    }};
}

#[cfg(test)]
mod tests {
    #[test]
    fn assert_approx() {
        assert_approx_eq!(9_u32, 10_u32, "0.12");
    }

    #[test]
    #[should_panic(expected = "8 doesn't belong to the expected range of 9 - 11")]
    fn assert_approx_fail() {
        assert_approx_eq!(8_u32, 10_u32, "0.12");
    }

    #[test]
    #[should_panic(
        expected = "8 doesn't belong to the expected range of 9 - 11: some extra info about the error"
    )]
    fn assert_approx_with_custom_panic_msg() {
        assert_approx_eq!(
            8_u32,
            10_u32,
            "0.12",
            "some extra {} about the error",
            "info"
        );
    }
}
