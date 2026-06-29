## ADDED Requirements

### Requirement: Dashboard data endpoint
The `App` struct SHALL expose a `GetDashboardData()` method that returns all data needed for the main dashboard view.

#### Scenario: Return daily, session, and run data
- **WHEN** `GetDashboardData()` is called from the frontend
- **THEN** it returns a struct containing: `daily` (all daily_usage rows ordered by date DESC), `sessions` (all session_usage rows ordered by total_tokens DESC), `runs` (recent 500 collection_runs ordered by id DESC)

#### Scenario: Enrich daily rows with project path
- **WHEN** building the response
- **THEN** each daily row gets a `projectPath` field derived from the most-token-heavy session for that (device, source) pair

#### Scenario: Normalize run messages
- **WHEN** returning run data
- **THEN** newlines in run messages are stripped and whitespace is collapsed

### Requirement: Time-series data endpoint
The `App` struct SHALL expose a `GetTimeSeriesData()` method that returns per-event time usage data.

#### Scenario: Return all time_usage rows
- **WHEN** `GetTimeSeriesData()` is called
- **THEN** it returns all time_usage rows ordered by event_time DESC

### Requirement: Collection trigger
The `App` struct SHALL expose a `StartCollection()` method that triggers a local data collection run asynchronously.

#### Scenario: Start collection
- **WHEN** `StartCollection()` is called
- **THEN** a goroutine is spawned that runs all collectors sequentially; the method returns immediately

#### Scenario: Collection already running
- **WHEN** `StartCollection()` is called while a collection is already in progress
- **THEN** returns false to indicate no new collection was started

### Requirement: Collection status
The `App` struct SHALL expose a `CollectStatus()` method that returns the current collection state.

#### Scenario: Idle state
- **WHEN** no collection has ever been run
- **THEN** returns status "idle"

#### Scenario: Running state
- **WHEN** a collection is in progress
- **THEN** returns status "running" with a message and start time

#### Scenario: Completed state
- **WHEN** a collection has finished
- **THEN** returns status "ok" or "error" with exit code, stdout excerpt, and timestamps

### Requirement: Collection progress events
The `App` SHALL emit Wails runtime events during collection to update the frontend in real-time.

#### Scenario: Progress update emitted
- **WHEN** each collector starts and completes
- **THEN** a Wails event is emitted with the collector name, status, and progress info
