## ADDED Requirements

### Requirement: Collector interface
The collection engine SHALL define a `Collector` interface that all 6 data collectors implement.

#### Scenario: All collectors implement the interface
- **WHEN** a new collector is added
- **THEN** it MUST implement: `ID() string`, `Collect(ctx, pricing) (*CollectResult, error)`

### Requirement: Sequential execution of all collectors
The collection engine SHALL iterate through all registered collectors and execute them one at a time.

#### Scenario: All collectors succeed
- **WHEN** all 6 collectors return successfully
- **THEN** the engine records an "ok" run for each collector and returns no error

#### Scenario: One collector fails, others continue
- **WHEN** a collector returns an error (e.g., directory not found)
- **THEN** the engine records an "error" run for that collector, continues with the remaining collectors, and sets the overall exit code to error

### Requirement: Transactional per-collector data writing
The engine SHALL wrap each collector's data (daily + session + time) in a SQLite transaction, committing only after all upserts succeed.

#### Scenario: Partial failure rolls back
- **WHEN** an upsert fails mid-way through a collector's data
- **THEN** the transaction is rolled back, and no partial data from that collector persists

### Requirement: Time usage data is replaced, not merged
Before inserting a collector's time usage events, the engine SHALL delete all existing time usage rows for that device+source.

#### Scenario: Fresh time events replace old data
- **WHEN** a collector returns time events
- **THEN** the engine first deletes all time_usage rows matching the device+source, then inserts the new events

### Requirement: Data normalization
The engine SHALL normalize collector output into the standard row format expected by the database layer (token breakdown, date, source, device).

#### Scenario: Token normalization for daily and session
- **WHEN** a collector returns raw token data
- **THEN** the engine extracts input, output, cache_read, cache_write, reasoning into the standard 5-field structure

### Requirement: Cost calculation after normalization
The engine SHALL pass each event's tokens + model through the pricing engine and attach the computed cost before upserting.

#### Scenario: Cost attached to each row
- **WHEN** daily or time data is normalized
- **THEN** `calculateCost(model, tokens, pricingData)` is called and the result populates `cost_usd`

### Requirement: Time event filtering by recency
The engine SHALL only keep time usage events within a configurable recency window (default 90 days).

#### Scenario: Old events are dropped
- **WHEN** events with timestamps older than 90 days are produced by a collector
- **THEN** those events are excluded from the time_usage table but still count toward daily_usage aggregation
