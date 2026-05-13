## 1. User CSV Export Columns

- [x] 1.1 Update `frontend/src/views/user/UsageView.vue` CSV headers to remove `Billable...` columns and all `Billing...` internal columns.
- [x] 1.2 Update CSV row mapping so `Input Tokens`, `Output Tokens`, `Cache Read Tokens`, `Cache Creation Tokens`, and image token values use the existing billable-first token breakdown.
- [x] 1.3 Keep `Final Cost` in the user CSV and remove `Billing Base Cost` from user CSV output.

## 2. Regression Tests

- [x] 2.1 Update user UsageView CSV export tests to assert no `Billable...` or `Billing...` headers are present.
- [x] 2.2 Add or update assertions that exported token values match billable-first values rather than raw token values.
- [x] 2.3 Verify admin Usage export/table tests remain unchanged for raw/billable/multiplier audit visibility.

## 3. Validation

- [x] 3.1 Run the targeted frontend UsageView test suite.
- [x] 3.2 Manually inspect generated CSV header expectations for user-facing wording and final-cost consistency.
