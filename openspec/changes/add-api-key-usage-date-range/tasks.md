## 1. Backend

- [x] 1.1 Extend the API key batch usage request DTO with optional `start_date` and `end_date` fields.
- [x] 1.2 Parse and validate optional date range values in `DashboardAPIKeysUsage`, including invalid format and end-before-start errors.
- [x] 1.3 Convert valid dates into application-timezone natural day boundaries and pass them to `GetBatchAPIKeyUsageStats`.
- [x] 1.4 Preserve ownership filtering, maximum key count validation, empty-list behavior, and existing response shape.

## 2. Frontend

- [x] 2.1 Add DateRangePicker state to the API Keys page with a default last-30-days range.
- [x] 2.2 Render the DateRangePicker in the API Keys filter area using the existing visual pattern.
- [x] 2.3 Pass the selected `start_date` and `end_date` to `usageAPI.getDashboardApiKeysUsage`.
- [x] 2.4 Update the usage column label so the range line shows the selected preset or custom date range instead of fixed “近30天”.
- [x] 2.5 Reload API key usage stats when the selected date range changes while keeping the existing today line unchanged.

## 3. Tests

- [x] 3.1 Add backend handler tests for default range compatibility, valid custom range, invalid dates, and today usage independence.
- [x] 3.2 Add frontend API tests or mocks verifying the date range payload sent by `getDashboardApiKeysUsage`.
- [x] 3.3 Add API Keys page tests for default last-30-days label, changing date range, and preserving today usage display.
- [x] 3.4 Run the focused backend and frontend test suites for the touched areas.
