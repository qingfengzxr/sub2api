## 1. Backend User Usage Response

- [x] 1.1 Add `ip_address` to the regular user usage log DTO in `backend/internal/handler/dto/types.go`.
- [x] 1.2 Map `service.UsageLog.IPAddress` into the regular user usage DTO in `backend/internal/handler/dto/mappers.go`, while keeping admin-only fields excluded.
- [x] 1.3 Update DTO tests to assert regular user usage DTOs include `ip_address` for owned records and admin DTO behavior remains unchanged.

## 2. Frontend Usage Table and Export

- [x] 2.1 Add optional `ip_address` to the frontend `UsageLog` type.
- [x] 2.2 Add an IP column to `frontend/src/views/user/UsageView.vue` with monospace display and `-` fallback for missing values.
- [x] 2.3 Add `IP` to the user Usage CSV headers and map each exported row to `log.ip_address || ''`.
- [x] 2.4 Keep existing billable-first token display/export behavior and continue excluding `Billable...` / `Billing...` internal columns from user CSV.

## 3. Regression Tests and Validation

- [x] 3.1 Update or add `frontend/src/views/user/__tests__/UsageView.spec.ts` coverage for rendering the IP column and fallback state.
- [x] 3.2 Update or add user Usage CSV export assertions that the `IP` column is present and row values match `ip_address`.
- [x] 3.3 Run targeted backend DTO tests.
- [x] 3.4 Run targeted frontend UsageView tests.
