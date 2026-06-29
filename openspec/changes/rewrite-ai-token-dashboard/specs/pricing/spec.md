## ADDED Requirements

### Requirement: Load LiteLLM pricing from bundled JSON
The pricing engine SHALL load and parse the LiteLLM model pricing JSON file bundled with the application at startup.

#### Scenario: Load valid pricing JSON
- **WHEN** the bundled `pricing-litellm.json` file exists and is valid JSON
- **THEN** all model entries are loaded into an in-memory lookup map

#### Scenario: File missing or corrupted
- **WHEN** the bundled pricing file is missing or invalid
- **THEN** the engine falls back gracefully (returns zero cost for all models)

### Requirement: Load OpenRouter pricing from bundled JSON
The pricing engine SHALL also load the OpenRouter model pricing JSON as a supplementary data source.

#### Scenario: OpenRouter data augments LiteLLM data
- **WHEN** both LiteLLM and OpenRouter pricing files are loaded
- **THEN** OpenRouter entries are merged into the lookup, with LiteLLM taking priority for exact matches

### Requirement: Model pricing lookup with fallback chain
The pricing engine SHALL resolve a model ID to per-token prices using a priority chain: exact match → provider-prefixed match → fuzzy match → hardcoded overrides → zero cost.

#### Scenario: Exact model ID match in LiteLLM
- **WHEN** the model ID directly matches a LiteLLM entry (e.g., "claude-sonnet-4-20250514")
- **THEN** the exact entry's prices are returned

#### Scenario: Provider-prefixed match
- **WHEN** raw model ID does not match but provider/model prefix matches (e.g., "anthropic/claude-sonnet-4-20250514")
- **THEN** the prefixed entry's prices are returned

#### Scenario: Fallback to Cursor overrides
- **WHEN** no LiteLLM or OpenRouter entry matches
- **THEN** the engine checks the hardcoded Cursor override table (for model IDs like "gpt-5.3-codex", "composer-2")

#### Scenario: Fallback to DeepSeek overrides
- **WHEN** no upstream entry matches a DeepSeek model
- **THEN** the engine checks the hardcoded DeepSeek override table

#### Scenario: Unknown model returns zero cost
- **WHEN** no match is found in any data source or override
- **THEN** a zero cost is returned (no error)

### Requirement: Tiered pricing for large context windows
The pricing engine SHALL support tiered pricing where models have different per-token prices above certain token thresholds (128K, 200K, 256K, 272K tokens).

#### Scenario: Below first threshold uses base price
- **WHEN** token count is at or below 128K
- **THEN** the base per-token price applies for all tokens

#### Scenario: Above threshold splits into tiers
- **WHEN** token count is 150K and a model has tiers at 128K
- **THEN** first 128K tokens are priced at the base rate, remaining 22K at the above-128K rate

### Requirement: Cost calculation with token breakdown
The pricing engine SHALL calculate USD cost from a token breakdown (input, output, cache_read, cache_write, reasoning).

#### Scenario: Standard cost calculation
- **WHEN** a model, token breakdown, and pricing data are provided
- **THEN** the cost is calculated as: input_tokens × input_price + output_tokens × output_price + cache_read × cache_read_price + reasoning × output_price

#### Scenario: Cache write pricing
- **WHEN** cache_write tokens are present and the model has a cache_write price
- **THEN** those tokens are multiplied by the cache_write price and added to the total

### Requirement: In-memory lookup cache
The pricing engine SHALL cache resolved pricing entries to avoid repeated fuzzy matching on the same model ID.

#### Scenario: Repeated lookup for same model
- **WHEN** the same model ID is queried multiple times
- **THEN** the result is returned from the in-memory cache after the first lookup, skipping the full resolution chain
