## ADDED Requirements

### Requirement: Scan Codex CLI session directories
The collector SHALL locate Codex CLI session data by scanning `~/.codex/sessions/`, `~/.codex/archived_sessions/`, and optional headless roots.

#### Scenario: Find JSONL files recursively
- **WHEN** the configured session directories exist
- **THEN** all `.jsonl` files are recursively collected

### Requirement: Parse three event types from Codex JSONL
The collector SHALL parse three event types: `session_meta` (workspace), `turn_context` (current model), and `event_msg/token_count` (token usage).

#### Scenario: Token count via last_token_usage
- **WHEN** an `event_msg` with `type: "token_count"` has both `last_token_usage` and `total_token_usage`
- **THEN** the increment is taken from `last_token_usage` (delta since last event)

#### Scenario: Fallback to total_token_usage delta
- **WHEN** `last_token_usage` is absent but `total_token_usage` and a previous total exist
- **THEN** the increment is computed as the difference of cumulative totals

#### Scenario: Deduplicate unchanged totals
- **WHEN** `total_token_usage` is identical to the previous event's total
- **THEN** the event is skipped (no new tokens consumed)

#### Scenario: Handle context reset (regression detection)
- **WHEN** `total_token_usage` goes backwards relative to the previous total (session context reset)
- **THEN** the new total is accepted as the baseline; no negative increment is produced

### Requirement: Cache normalization for Codex
The collector SHALL normalize Codex's cached input tokens into the standard cache_read field, clamped to ≤ input.

#### Scenario: Cached tokens separated from input
- **WHEN** Codex reports `cached_input_tokens` or `cache_read_input_tokens`
- **THEN** the cached portion is moved to `cacheRead` and subtracted from `input`, clamped to not exceed total input
