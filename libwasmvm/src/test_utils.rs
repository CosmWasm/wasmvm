#![cfg(test)]

use cosmwasm_std::{Decimal, Uint128};
use std::str::FromStr as _;

/// Asserts that two expressions are approximately equal to each other.
///
/// The ratio argument defines how wide a range of values we accept, and is applied
/// to the **second** (`right`) argument.
///
/// On panic, this macro will print the values of the expressions with their
/// debug representations, and info about the acceptable range.
///
/// Like [`assert_eq!`], this macro has a second form, where a custom
/// panic message can be provided.
#[macro_export]
macro_rules! assert_approx_eq {
    ($left:expr, $right:expr, $ratio:expr $(,)?) => {{
        $crate::test_utils::assert_approx_eq_impl($left, $right, $ratio, None);
    }};
    ($left:expr, $right:expr, $ratio:expr, $($args:tt)+) => {{
        $crate::test_utils::assert_approx_eq_impl($left, $right, $ratio, Some(format!($($args)*)));
    }};
}

#[track_caller]
fn assert_approx_eq_impl(
    left: impl Into<Uint128>,
    right: impl Into<Uint128>,
    ratio: &str,
    panic_msg: Option<String>,
) {
    let left = left.into();
    let right = right.into();
    let ratio = Decimal::from_str(ratio).unwrap();

    let delta = right * ratio;
    let lower_bound = right - delta;
    let upper_bound = right + delta;

    if !(left >= lower_bound && left <= upper_bound) {
        match panic_msg {
            Some(panic_msg) => panic!(
                "{} doesn't belong to the expected range of {} - {}: {}",
                left, lower_bound, upper_bound, panic_msg
            ),
            None => panic!(
                "{} doesn't belong to the expected range of {} - {}",
                left, lower_bound, upper_bound
            ),
        }
    }
}

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
