## ADDED Requirements

### Requirement: Read Hermes Agent SQLite database
The collector SHALL read token usage data from the Hermes Agent SQLite database at `~/.hermes/state.db` (or `$HERMES_HOME/state.db`).

#### Scenario: Database exists and is readable
- **WHEN** the Hermes SQLite file exists
- **THEN** the collector queries usage data from the appropriate tables

#### Scenario: Database does not exist
- **WHEN** the Hermes SQLite file is not found
- **THEN** the collector returns empty data (no error)

### Requirement: Parse Hermes token usage records
The collector SHALL extract daily token contributions, session data, and model information from the Hermes database.

#### Scenario: Extract token breakdown per day per model
- **WHEN** Hermes DB contains rows with input_tokens, output_tokens, and model info
- **THEN** tokens are aggregated per (date, model) pair

#### Scenario: Time-series events extracted
- **WHEN** Hermes DB contains timestamped events
- **THEN** individual events with token breakdown and timing are returned for the time_usage table
