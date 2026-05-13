## 1. Backend Settings

- [x] 1.1 Add long-context pricing setting keys for enabled state and threshold tokens with safe defaults.
- [x] 1.2 Extend settings response/update DTOs, parsing, normalization, and validation for the new fields.
- [x] 1.3 Add a backend policy/helper that returns the effective long-context pricing configuration with disabled fallback on invalid or missing settings.

## 2. Billing Logic

- [x] 2.1 Thread the long-context pricing policy into token cost calculation paths without affecting per-request/image billing.
- [x] 2.2 Apply long-context multipliers only when the platform switch is enabled, the billable input + cache-read context exceeds the configured threshold, and model multipliers are valid.
- [x] 2.3 Preserve interval pricing behavior so channel interval pricing does not receive an extra long-context multiplier.

## 3. Admin Frontend

- [x] 3.1 Add Settings → Features form fields for long-context pricing enabled state and threshold tokens.
- [x] 3.2 Add i18n copy and UI hints explaining that the threshold is based on billable input plus cache-read tokens.
- [x] 3.3 Ensure save/load flows include the new fields and use validation-friendly number input constraints.

## 4. Tests and Validation

- [x] 4.1 Add or update backend settings tests for default, save, invalid threshold, and policy fallback behavior.
- [x] 4.2 Add billing tests for disabled, enabled-below-threshold, enabled-above-threshold, and interval-pricing cases.
- [x] 4.3 Add or update frontend settings tests if adjacent coverage exists for feature toggles.
- [x] 4.4 Run targeted backend billing/settings tests and targeted frontend settings tests.
