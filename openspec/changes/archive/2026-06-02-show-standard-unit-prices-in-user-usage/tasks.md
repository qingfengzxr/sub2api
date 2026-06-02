## 1. Standard Unit Price Source

- [x] 1.1 Inspect the user usage DTO and frontend `UsageLog` type to determine whether user-safe standard unit price fields already exist.
- [x] 1.2 If needed, add backend DTO fields for standard input/output/cache/image unit prices derived from the selected billing model pricing context before multipliers.
- [x] 1.3 Ensure any new user-facing price fields exclude `billing_token_multiplier`, group/user `rate_multiplier`, image multiplier, service tier, and long-context multiplier effects.

## 2. User Usage Tooltip

- [x] 2.1 Update C端 `UsageView` cost tooltip to display input/output unit prices from standard unit price fields or a standard pricing helper, not from `input_cost / input_tokens` or `output_cost / output_tokens`.
- [x] 2.2 Apply the same standard-price-only rule to cache read/write, image, and per-request unit price displays where those prices are shown to ordinary users.
- [x] 2.3 Preserve final cost display, billable-first token display, service tier label, and existing CSV behavior without reintroducing multiplier or raw/billable dual-column wording.
- [x] 2.4 Show an empty placeholder for unavailable standard unit prices instead of falling back to adjusted cost-derived unit prices.

## 3. Tests and Verification

- [x] 3.1 Add or update frontend tests proving a usage record with adjusted `input_cost`/`output_cost` still displays base standard input/output prices.
- [x] 3.2 Add backend DTO tests if new standard unit price fields are introduced.
- [x] 3.3 Run focused frontend tests for `UsageView` and any touched backend tests.
- [x] 3.4 Run `openspec status --change show-standard-unit-prices-in-user-usage` and confirm the change is apply-ready.
