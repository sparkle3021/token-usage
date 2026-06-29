## ADDED Requirements

### Requirement: Read OpenCode SQLite database
The collector SHALL read token usage data from the OpenCode state database at `~/.local/share/opencode/` (configurable).

#### Scenario: Database exists
- **WHEN** the OpenCode SQLite file exists at the configured path
- **THEN** the collector queries usage data from the relevant tables

#### Scenario: Database does not exist
- **WHEN** the OpenCode SQLite file is not found
- **THEN** the collector returns empty data (no error)

### Requirement: Parse OpenCode usage records
The collector SHALL extract daily and session-level token consumption from the OpenCode database schema.

#### Scenario: Daily aggregation
- **WHEN** the OpenCode DB contains usage records with dates and models
- **THEN** tokens are aggregated per (date, model) pair

#### Scenario: Session/workspace aggregation
- **WHEN** the OpenCode DB contains project or session identifiers
- **THEN** tokens are aggregated per (workspace, model) pair
