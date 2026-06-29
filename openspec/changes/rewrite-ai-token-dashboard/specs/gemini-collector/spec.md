## ADDED Requirements

### Requirement: Parse Gemini CLI session files in two formats
The collector SHALL parse both JSON (full-session and headless-stats) and JSONL (streaming) session files from `~/.gemini/tmp/`.

#### Scenario: Full-session JSON format
- **WHEN** a JSON file with `sessionId` and `messages[]` is found
- **THEN** each message with `type: "gemini"` and a `tokens` object is parsed

#### Scenario: Headless stats JSON format
- **WHEN** a JSON file with a `stats` object (no session wrapper) is found
- **THEN** the model breakdown or flat token counts are extracted

#### Scenario: JSONL streaming format
- **WHEN** a `.jsonl` file is found
- **THEN** each line is parsed as a standalone JSON object, handling `init`, `gemini` turn, and `stats` line types

### Requirement: Cache-inclusive input normalization
Gemini's "input" token count includes the cached portion. The collector SHALL separate them: `net_input = input - cached` (clamped ≥ 0), `cache_read = cached`.

#### Scenario: Inclusive formula detected via total field
- **WHEN** a session object has `tokens.total` that matches `input + output + reasoning + tool` (cache still inside input)
- **THEN** the cached portion is subtracted from input

#### Scenario: Exclusive formula detected via total field
- **WHEN** `tokens.total` matches `input + output + reasoning + tool + cached` (input already net)
- **THEN** input is used as-is and cached is moved to cache_read

#### Scenario: No total field — fall back to headless logic
- **WHEN** no `tokens.total` is provided
- **THEN** cached is subtracted from input with clamping (same as headless path)

### Requirement: Deduplicate JSONL gemini events by message ID
When a JSONL stream emits the same gemini event ID twice (updated data), the collector SHALL keep only the latest version.

#### Scenario: Updated event replaces previous
- **WHEN** a JSONL line has a `type: "gemini"` with an `id` field that matches a previously parsed event
- **THEN** the previous event is replaced with the new data
