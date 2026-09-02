## 1. Data Model And Migration

- [x] 1.1 Add a forward-compatible migration for `api_keys.rate_limit_30d`, `usage_30d`, and `window_30d_start` with zero defaults for existing keys.
- [x] 1.2 Extend the API Key Ent schema with the three monthly-window fields and regenerate Ent code.
- [x] 1.3 Extend service API Key models, repository entity mappings, DTOs, and frontend types with `rate_limit_30d`, `usage_30d`, and the internal window start where applicable.

## 2. API And Persistence

- [x] 2.1 Add optional `rate_limit_30d` handling to the user API Key update request and validate it as finite and non-negative while preserving omitted-field semantics.
- [x] 2.2 Include the monthly limit in API Key list/detail responses without changing existing response fields.
- [x] 2.3 Extend selective repository updates so limit edits persist `rate_limit_30d` but never overwrite `usage_30d` or `window_30d_start` from a stale entity snapshot.
- [x] 2.4 Extend atomic rate-limit usage increments and reads to reset or accumulate the 30-day window in the same transaction as the existing windows.
- [x] 2.5 Extend the existing rate-limit usage reset operation to clear all four counters and window starts.

## 3. Enforcement And Caching

- [x] 3.1 Add `RateLimitWindow30d`, `EffectiveUsage30d`, and the `API_KEY_RATE_30D_EXCEEDED` HTTP 429 error.
- [x] 3.2 Extend gateway/billing preflight checks to reject a key whose effective 30-day usage has reached its positive monthly limit before forwarding upstream.
- [x] 3.3 Extend authentication cache entries and invalidation so monthly limit edits take effect on the next request.
- [x] 3.4 Extend Redis rate-limit cache serialization, fallback hydration, and Lua window updates with `usage_30d` and `window_30d_start`.

## 4. User Interface

- [x] 4.1 Add a “30天限额 (USD)” field below the 7-day field in the user API Key edit modal and load/save `rate_limit_30d` through the existing form flow.
- [x] 4.2 Include the monthly field in the rate-limit toggle state and submit all four limits as zero when the toggle is disabled.
- [x] 4.3 Keep the new control scoped to the edit modal and existing rate-limit toggle.
- [x] 4.4 Add Chinese and English locale strings that describe the limit as a fixed 30-day window.

## 5. Verification

- [x] 5.1 Add handler/service tests for positive, zero, omitted, negative, NaN, and infinite `rate_limit_30d` update inputs.
- [x] 5.2 Add repository tests for concurrent-safe monthly accumulation, exact 30-day expiry, new-window initialization, and four-window manual reset.
- [x] 5.3 Add billing/cache tests proving database, auth-cache, Redis-cache, and fallback paths make identical monthly limit decisions and return the monthly error code.
- [x] 5.4 Add frontend tests for loading, editing, disabling, and submitting the monthly limit.
- [x] 5.5 Regenerate API/Ent artifacts as required, then run focused backend tests, frontend unit tests, type checking, and migration validation.
