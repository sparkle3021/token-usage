## ADDED Requirements

### Requirement: System SHALL persist checkpoints in app_config table
The system SHALL use the `app_config` table to store checkpoint values for each sync source.

#### Scenario: Store checkpoint
- **WHEN** a checkpoint needs to be saved
- **THEN** the system SHALL call `SetConfig(key, value)` where key follows the pattern `cc_switch_cursor_<table_name>`
- **AND** the value SHALL be the string representation of the cursor value (UNIX timestamp or date string)

#### Scenario: Read checkpoint
- **WHEN** a checkpoint needs to be retrieved
- **THEN** the system SHALL call `GetConfig(key)` where key is the same pattern used during save
- **AND** SHALL return an empty string if no checkpoint exists (first sync)

### Requirement: System SHALL support multiple independent checkpoints
The system SHALL maintain separate checkpoints for each data source/table.

#### Scenario: Independent checkpoints per source
- **WHEN** both `proxy_request_logs` and `usage_daily_rollups` are synced
- **THEN** checkpoints SHALL be stored under different keys:
  - `cc_switch_cursor_proxy_request_logs` for proxy logs (UNIX timestamp)
  - `cc_switch_rollup_max_date` for daily rollups (ISO date string)
- **AND** each SHALL be read/written independently without affecting the other

### Requirement: System SHALL handle checkpoint staleness gracefully
The system SHALL detect when a checkpoint value is no longer valid and fall back to a full sync.

#### Scenario: Checkpoint exceeds max created_at
- **WHEN** the stored checkpoint value is greater than the current `MAX(created_at)` in the source table
- **THEN** the system SHALL log a warning
- **AND** SHALL perform a full sync (reset checkpoint)
- **AND** SHALL update the checkpoint to the new max value after sync

### Requirement: System SHALL provide a way to reset checkpoints
The system SHALL support forcefully clearing all CC-Switch checkpoints to trigger a full re-sync on next run.

#### Scenario: Reset all checkpoints
- **WHEN** a full re-sync is requested (e.g., `forceFull` mode)
- **THEN** the system SHALL delete all `cc_switch_cursor_*` and `cc_switch_rollup_*` config keys
- **AND** SHALL perform a full sync on the next collection cycle
