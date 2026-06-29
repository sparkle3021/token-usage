## ADDED Requirements

### Requirement: Scan Claude Code project directories
The collector SHALL locate Claude Code session data by scanning configured directory roots (`~/.claude/projects/`, `~/.config/claude/projects/`, and optionally macOS local-agent-mode-sessions).

#### Scenario: Default paths exist
- **WHEN** `~/.claude/projects/` or `~/.config/claude/projects/` contains subdirectories with JSONL files
- **THEN** all `.jsonl` files are recursively collected for parsing

#### Scenario: Custom config path via CLAUDE_CONFIG_DIR
- **WHEN** the environment variable `CLAUDE_CONFIG_DIR` is set
- **THEN** the collector uses that path instead of the defaults

### Requirement: Parse JSONL assistant turn records
The collector SHALL parse each JSONL file, extracting assistant-turn records with usage information.

#### Scenario: Extract assistant turn usage
- **WHEN** a JSONL line has `type: "assistant"` and `message.usage` is present
- **THEN** the tokens (input_tokens, output_tokens, cache_read_input_tokens, cache_creation_input_tokens, reasoning_tokens) are extracted along with model name and timestamp

#### Scenario: Deduplicate by message.id and requestId
- **WHEN** multiple JSONL lines share the same `message.id` (or `message.id + requestId`)
- **THEN** they are collapsed into one record, taking the maximum value for each token field

### Requirement: Aggregate into daily and session data
The collector SHALL aggregate parsed events into daily contributions and workspace+model summaries.

#### Scenario: Daily aggregation by date and model
- **WHEN** events are parsed with dates and model IDs
- **THEN** tokens are summed per (date, model) pair

#### Scenario: Session aggregation by workspace key and model
- **WHEN** events are parsed with workspace keys (decoded project directory names)
- **THEN** tokens are summed per (workspace, model) pair

#### Scenario: Workspace label decoded from URL encoding
- **WHEN** a project directory name is URL-encoded (e.g., `%2FUsers%2Fproject`)
- **THEN** it is decoded to the absolute path, falling back to the raw name on failure
