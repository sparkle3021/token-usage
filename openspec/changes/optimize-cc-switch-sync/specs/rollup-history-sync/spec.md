## ADDED Requirements

### Requirement: System SHALL read usage_daily_rollups from CC-Switch database
The system SHALL query the `usage_daily_rollups` table in the CC-Switch database to obtain historical daily aggregated usage data.

#### Scenario: Connect and query rollups
- **WHEN** the system executes a rollup history sync
- **THEN** it SHALL open the CC-Switch SQLite database at the configured path
- **AND** SHALL query `usage_daily_rollups` for all rows with `date < date('now', 'localtime')`
- **AND** SHALL select columns: `date`, `app_type`, `model`, `input_tokens`, `output_tokens`, `cache_read_tokens`, `cache_creation_tokens`, `total_cost_usd`
- **AND** SHALL return an error if the table does not exist

### Requirement: System SHALL write rollup data to daily_usage
The system SHALL convert rollup rows to `daily_usage` records and upsert them into the local database.

#### Scenario: Upsert rollup rows
- **WHEN** rollup rows are fetched
- **THEN** the system SHALL map `app_type` to source name
- **AND** SHALL normalize model names
- **AND** SHALL call `UpsertDaily()` for each accumulated `(source, date, model)` group
- **AND** SHALL NOT modify `hour_usage` (rollup data lacks hour granularity)

### Requirement: System SHALL track last synced date range for rollups
The system SHALL record the maximum date synced from `usage_daily_rollups` as a checkpoint to avoid redundant re-processing.

#### Scenario: Record rollup checkpoint
- **WHEN** a rollup sync completes
- **THEN** the system SHALL record `max(usage_date)` synced into the checkpoint store under key `cc_switch_rollup_max_date`
- **AND** on subsequent syncs SHALL only query rollup rows with `date > checkpoint_date`
