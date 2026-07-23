## 1. Data Model and Domain Policy

- [x] 1.1 Add an idempotent migration and Ent user field for `overdraft_limit` with database default `0` and a non-negative CHECK constraint, then regenerate Ent code.
- [x] 1.2 Map `overdraft_limit` through the service user model and existing user repository create/read/update paths.
- [x] 1.3 Add the isolated balance eligibility policy and focused table tests for zero-limit compatibility, minimum reserve, positive overdraft, exact boundary, and invalid negative stored values.

## 2. Admin Configuration

- [x] 2.1 Extend the existing admin user DTO and update request/input with `overdraft_limit`, validate it as a non-negative finite value, and keep ordinary user responses unchanged.
- [x] 2.2 Persist overdraft changes through the existing admin user update flow and include changes in per-user auth cache invalidation.
- [x] 2.3 Add the overdraft amount input to `UserEditModal`, pass it through the existing admin users API type, and add concise Chinese and English labels/hints describing the soft-limit behavior.
- [x] 2.4 Add focused backend and frontend tests for loading, validating, saving, clearing, and defaulting the admin value.

## 3. Billing Enforcement

- [x] 3.1 Add `overdraft_limit` to the API Key user auth snapshot, bump its schema version, and cover snapshot round trips and legacy snapshot rejection.
- [x] 3.2 Replace the API Key middleware balance threshold calculation with the shared eligibility policy without changing unrelated middleware behavior.
- [x] 3.3 Make `BillingCacheService` use the shared policy with cached balance and the existing minimum reserve, without changing any gateway handler signatures.
- [x] 3.4 Apply the same user-aware threshold when synchronizing balance cache after deduction so eligible negative balances remain on the incremental cache path.
- [x] 3.5 Update batch image reserve SQL to atomically require `balance - hold_amount >= -overdraft_limit`, leaving capture and release behavior unchanged.

## 4. Verification

- [x] 4.1 Add focused tests proving default zero-limit behavior is unchanged in auth middleware, billing cache, and deduction paths.
- [x] 4.2 Add tests for negative balances inside, at, and beyond the soft limit, including minimum reserve interaction and configuration cache invalidation.
- [x] 4.3 Add repository tests for batch image holds inside and beyond the limit, including concurrent/atomic behavior and unchanged balances after rejection.
- [x] 4.4 Run focused backend tests, frontend tests, migration/schema checks, and OpenSpec validation; confirm subscription and API Key quota behavior remains unchanged.
