## ADDED Requirements

### Requirement: Database schema matches source project
The database layer SHALL maintain 4 tables identical in structure to the source project's SQLite schema: `collection_runs`, `daily_usage`, `session_usage`, `time_usage`.

#### Scenario: Schema initialization on first open
- **WHEN** the application starts and no SQLite database exists
- **THEN** all 4 tables are created with the correct columns, types, primary keys, and indexes

#### Scenario: WAL mode enabled
- **WHEN** the database is opened
- **THEN** `PRAGMA journal_mode = WAL` is set for concurrent read performance

#### Scenario: busy_timeout configured
- **WHEN** the database is opened
- **THEN** `PRAGMA busy_timeout = 10000` is set to prevent SQLITE_BUSY errors

### Requirement: Upsert operations for all usage tables
The database SHALL provide upsert (INSERT OR REPLACE) operations for `daily_usage`, `session_usage`, and `time_usage`.

#### Scenario: Upsert daily_usage merges by composite key
- **WHEN** daily usage data is inserted with the same `(device, source, usage_date, model)` as an existing row
- **THEN** the token fields and cost are summed/updated, and `updated_at` is refreshed

#### Scenario: Past dates lock pricing on upsert
- **WHEN** daily usage data for a past date (before today) is upserted
- **THEN** `cost_usd` and `pricing_locked_at` are NOT overwritten; only token counts update

#### Scenario: Session upsert merges by composite key
- **WHEN** session data with the same `(device, source, session_id)` is inserted
- **THEN** all token/cost fields are replaced and `updated_at` is refreshed

#### Scenario: Time usage upsert merges by composite key
- **WHEN** time event data with the same `(device, source, event_key)` is inserted
- **THEN** all fields are replaced and `updated_at` is refreshed

### Requirement: Collection runs tracking
The database SHALL record each collector's execution result in `collection_runs`.

#### Scenario: Record successful collection run
- **WHEN** a collector completes successfully
- **THEN** a row is inserted with device, source, status="ok", message, and timestamp

#### Scenario: Record failed collection run
- **WHEN** a collector returns an error
- **THEN** a row is inserted with status="error" and the error message

#### Scenario: Prune old collection runs
- **WHEN** the database is opened, or after inserting a new run
- **THEN** only the most recent 500 runs are retained; older rows are deleted

### Requirement: Time usage cleanup
The database SHALL support deleting all time usage records for a given device and source before re-inserting fresh data.

#### Scenario: Delete by device and source
- **WHEN** collection starts for a specific device+source
- **THEN** all matching rows in `time_usage` are deleted before new data is inserted
