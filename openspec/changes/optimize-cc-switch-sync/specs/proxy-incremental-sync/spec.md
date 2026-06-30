## ADDED Requirements

### Requirement: System SHALL read proxy_request_logs from CC-Switch database
The system SHALL connect to the CC-Switch SQLite database at the configured path and query the `proxy_request_logs` table for usage data.

#### Scenario: Connect to cc-switch database
- **WHEN** the system starts a proxy incremental sync
- **THEN** it SHALL open the SQLite database at the path stored in `app_config.cc_switch_db_path`
- **AND** SHALL return an error if the database file does not exist or cannot be opened

#### Scenario: Query proxy_request_logs with filters
- **WHEN** the system queries `proxy_request_logs`
- **THEN** it SHALL filter by `app_type = 'claude'` and `data_source = 'proxy'` and `status_code BETWEEN 200 AND 299`
- **AND** SHALL select columns: `created_at`, `app_type`, `model`, `input_tokens`, `output_tokens`, `cache_read_tokens`, `cache_creation_tokens`, `total_cost_usd`

### Requirement: System SHALL support incremental sync via created_at cursor
The system SHALL read the last synchronized `MAX(created_at)` timestamp from the checkpoint store, and only query records with `created_at > cursor_value`.

#### Scenario: First sync (no checkpoint)
- **WHEN** no checkpoint exists for `proxy_request_logs`
- **THEN** the system SHALL query ALL records (no `created_at` filter)
- **AND** SHALL record the `MAX(created_at)` as checkpoint after completion

#### Scenario: Subsequent sync (checkpoint exists)
- **WHEN** a checkpoint value `T` exists for `proxy_request_logs`
- **THEN** the system SHALL add `WHERE created_at > T` to the query
- **AND** SHALL update the checkpoint to `MAX(created_at)` of the new results after completion

### Requirement: System SHALL aggregate proxy records into hour_usage
The system SHALL group the queried records by `(date, hour, source, model)` and aggregate token/cost fields before writing to `hour_usage`.

#### Scenario: Aggregate and write
- **WHEN** records are fetched from `proxy_request_logs`
- **THEN** the system SHALL map `app_type` to source name via `sourceFromAppType()`
- **AND** SHALL normalize model names via `NormalizeModelForGrouping()`
- **AND** SHALL group by `(usage_date, hour, source, model)` and sum tokens and cost
- **AND** SHALL call `BulkUpsertHourUsage()` to write aggregated data
- **AND** SHALL call `BuildDailyFromHourUsage()` to rebuild daily_usage
