## Context

Usage logs already persist `ip_address`, and the repository selects/scans it for usage records. The admin usage DTO, admin table, and admin export already expose this field for audit. The regular user DTO currently omits `ip_address`, and the C 端 `UsageView` table/CSV therefore cannot show the request IP even though `/usage` list and `/usage/:id` are already scoped to the authenticated user's own records.

## Goals / Non-Goals

**Goals:**

- Expose `ip_address` on regular user usage log responses for records owned by the authenticated user.
- Add an IP column to the C 端 Usage table, matching the admin table's plain monospace display and `-` fallback for missing historical values.
- Add an `IP` column to the C 端 Usage CSV export so exported records match the visible table metadata.
- Preserve existing C 端 billable-first token display and hidden internal billing multiplier behavior.
- Keep admin usage behavior unchanged.

**Non-Goals:**

- No schema migration or usage ingestion change; this assumes existing `usage_logs.ip_address` capture remains the source of truth.
- No new IP filtering, sorting, masking, geolocation, or abuse-analysis workflow.
- No change to billing, usage aggregation, dashboard summaries, or admin export columns.

## Decisions

- Include `ip_address` directly in the regular user `UsageLog` DTO.
  - Rationale: `/usage` list filters by authenticated `user_id`, and `/usage/:id` verifies record ownership before returning data. Reusing the existing DTO keeps frontend list, detail, and CSV export on the same contract.
  - Alternative considered: Add a separate user-only endpoint for IP details. This would duplicate access-control and pagination behavior for a single already-selected field.

- Keep `ip_address` as optional/null in API and frontend types.
  - Rationale: historical rows or failed IP capture may not have a value. The UI should render `-` rather than blocking display or inventing placeholder data.
  - Alternative considered: Backfill or synthesize IP values. There is no reliable source for records that were not captured.

- Add the table column at the far right near other request metadata.
  - Rationale: admin Usage already places IP at the end, and C 端 Usage already uses horizontal scrolling. This preserves the existing scan order of billing/time fields while making request-origin metadata available.
  - Alternative considered: Merge IP into the User-Agent cell. That would reduce column count, but it makes copy/scan harder and diverges from the admin reference screenshot.

- Export IP as a normal request metadata column, not as an internal audit-only field.
  - Rationale: IP helps users reconcile their own API Key usage and is not tied to hidden billing multipliers or raw/billable internal mechanics.
  - Alternative considered: Page-only display. That would make CSV less useful for incident review and inconsistent with the table.

## Risks / Trade-offs

- [Risk] IP addresses can be sensitive personal data. -> Mitigation: expose only records already owned by the authenticated user, rely on existing list/detail ownership checks, and do not add cross-user search/filtering.
- [Risk] Extra table width makes C 端 Usage denser. -> Mitigation: place IP as the last column and use the existing horizontally scrollable table behavior.
- [Risk] Tests may assume the old user DTO never contained `ip_address`. -> Mitigation: update DTO tests to assert user DTO includes IP while still excluding admin-only fields such as account details and account multiplier.
