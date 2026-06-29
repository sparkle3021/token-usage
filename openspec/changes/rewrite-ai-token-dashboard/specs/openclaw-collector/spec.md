## ADDED Requirements

### Requirement: Scan OpenClaw agent directories
The collector SHALL locate OpenClaw session data by scanning configured agent roots (e.g., `~/.openclaw/agents/`, `~/.clawdbot/agents/`).

#### Scenario: Agent directories exist
- **WHEN** any configured agent root directory exists
- **THEN** all session files within are recursively collected

### Requirement: Parse OpenClaw JSONL session files
The collector SHALL parse OpenClaw JSONL session files to extract assistant-turn token usage.

#### Scenario: Extract assistant turn usage
- **WHEN** a JSONL line contains assistant response data with token usage
- **THEN** the tokens (input, output, cache) are extracted along with model and timestamp

#### Scenario: Aggregate daily and session data
- **WHEN** events are parsed
- **THEN** tokens are aggregated per (date, model) for daily data and per (workspace, model) for session data
