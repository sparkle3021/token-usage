## ADDED Requirements

### Requirement: System SHALL detect missing historical dates
The system SHALL compare rollup data dates against existing local `daily_usage` records to identify dates that need supplementation.

#### Scenario: Detect missing dates
- **WHEN** rollup data is loaded
- **THEN** for each `(date, source, model)` combination in the rollup data
- **AND** if no corresponding `daily_usage` record exists for that date in the local database
- **THEN** the system SHALL record this as a "missing date" entry for supplementation

#### Scenario: Detect zero-token dates
- **WHEN** a local `daily_usage` record exists with `total_tokens = 0`
- **THEN** the system SHALL treat it as "missing" and attempt to supplement from rollup data
- **AND** if rollup data for that date has non-zero tokens, SHALL update the record

### Requirement: System SHALL supplement missing dates from rollup data
The system SHALL write rollup data into `daily_usage` for dates identified as missing or zero-token.

#### Scenario: Supplement single missing date
- **WHEN** date `D` is identified as missing for source `S` and model `M`
- **AND** rollup data contains a row for `(D, S, M)` with non-zero tokens
- **THEN** the system SHALL call `UpsertDaily()` to insert/update the record in `daily_usage`
- **AND** SHALL NOT modify `hour_usage` or `time_usage`

#### Scenario: Skip present dates
- **WHEN** a date already has valid `daily_usage` data
- **THEN** the system SHALL NOT overwrite or modify that date's data
- **AND** SHALL respect the existing `pricing_locked_at` constraint

### Requirement: System SHALL report reconciliation results
The system SHALL report how many dates were supplemented during reconciliation.

#### Scenario: Report results
- **WHEN** reconciliation completes
- **THEN** the system SHALL log: number of dates checked, number of dates supplemented, number of dates skipped
- **AND** SHALL return this information in the collection result summary
