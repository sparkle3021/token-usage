## ADDED Requirements

### Requirement: Dashboard data loading
The frontend SHALL load dashboard data from the Go backend via Wails bindings on mount.

#### Scenario: Initial data load
- **WHEN** the App component mounts
- **THEN** `GetDashboardData()` is called and the returned data is stored in component state

#### Scenario: Loading state
- **WHEN** data is being fetched
- **THEN** a loading spinner is displayed

#### Scenario: Error state
- **WHEN** data loading fails
- **THEN** an error message is displayed with a retry button

### Requirement: Filter bar
The dashboard SHALL provide a filter bar with time range selection, source/di/device/model multi-select filters.

#### Scenario: Time range presets
- **WHEN** a user clicks a time range preset (今天/7天/14天/30天/全部)
- **THEN** the displayed data range is updated accordingly

#### Scenario: Source filter pill toggle
- **WHEN** a user clicks a source pill
- **THEN** that source is toggled in/out of the filter set

#### Scenario: Device/Model multi-select
- **WHEN** a user opens the device or model dropdown
- **THEN** checkboxes allow selecting/deselecting multiple values

#### Scenario: Comparison period toggle
- **WHEN** the user enables "对比上一周期"
- **THEN** all KPI delta indicators and some charts show the previous period's data for comparison

#### Scenario: CSV export
- **WHEN** the user clicks the export button
- **THEN** a CSV file of the filtered data is downloaded

### Requirement: KPI cards
The dashboard SHALL display 6 KPI cards: Total Token, Input, Output, Cache, Reasoning, and Estimated Cost.

#### Scenario: KPI values display
- **WHEN** filtered data is available
- **THEN** each KPI shows the aggregated value with Chinese-compact formatting (万/亿)

#### Scenario: Sparkline in KPI
- **WHEN** filtered data covers multiple days
- **THEN** each KPI card shows an SVG sparkline of the daily trend

#### Scenario: Delta indicator
- **WHEN** comparison period is enabled and previous data exists
- **THEN** each KPI shows a delta percentage (↑/↓/·) compared to the previous period

### Requirement: Trend chart (recharts)
The dashboard SHALL display a trend chart of daily token usage with switchable modes: stacked bar, line, and grouped bar.

#### Scenario: Stacked bar mode
- **WHEN** the user selects stacked mode
- **THEN** each source is shown as a filled bar segment per day, composited into a total bar

#### Scenario: Line mode
- **WHEN** the user selects line mode
- **THEN** each source is shown as a separate line with a fill area beneath

#### Scenario: Comparison overlay
- **WHEN** comparison period is enabled
- **THEN** a dashed line overlay shows the previous period's daily totals

#### Scenario: Data zoom
- **WHEN** there are more than 20 data points
- **THEN** a slider zoom control is displayed below the chart

### Requirement: Source donut chart
The dashboard SHALL display a donut chart showing each source's share of total tokens.

#### Scenario: Donut display
- **WHEN** data is available
- **THEN** a donut chart shows source proportions with a center total

#### Scenario: Source focusing
- **WHEN** a user clicks a legend item
- **THEN** the chart focuses on that source, dimming others, and all charts on the page filter to that source

### Requirement: Top models bar chart
The dashboard SHALL display a horizontal bar chart of the top 8 models by total tokens.

#### Scenario: Top models display
- **WHEN** filtered data is available
- **THEN** the top 8 models are shown ranked by total tokens, with a source-colored fill bar and cost

### Requirement: Cache hit rate gauge
The dashboard SHALL display a gauge/arc showing the cache hit rate (cache_read / total).

#### Scenario: Gauge arc
- **WHEN** data is available
- **THEN** an SVG arc from 0 to 180 degrees shows the cache hit percentage

### Requirement: Heatmap (day × hour)
The dashboard SHALL display a synthetic heatmap showing usage intensity across days and hours.

#### Scenario: 24h distribution
- **WHEN** daily totals are available
- **THEN** each day's total is distributed across 24 hours using a standard hourly pattern, shown as a color-coded grid

### Requirement: Growth panel
The dashboard SHALL display WoW (week-over-week), DoD (day-over-day), and MoM (month-over-month) growth statistics.

#### Scenario: Calculate and display deltas
- **WHEN** filtered data spans at least 2 days
- **THEN** DoD, WoW, and MoM deltas are computed and displayed

### Requirement: Tabbed data table
The dashboard SHALL display a sortable, searchable data table with 4 tabs: Sources, Models, Sessions, Collection Runs.

#### Scenario: Source tab aggregation
- **WHEN** the Sources tab is selected
- **THEN** daily data is aggregated by (source, device) pair with totals, shares, and costs

#### Scenario: Model tab aggregation
- **WHEN** the Models tab is selected
- **THEN** daily data is aggregated by (source, model) pair

#### Scenario: Session tab
- **WHEN** the Sessions tab is selected
- **THEN** session data is displayed with project paths, token counts, and activity

#### Scenario: Collection Runs tab
- **WHEN** the Runs tab is selected
- **THEN** collection run history is displayed with timestamps, status badges, and messages

#### Scenario: Column sorting
- **WHEN** a user clicks a column header
- **THEN** the table rows are sorted ascending/descending by that column

#### Scenario: Text search
- **WHEN** a user types in the search box
- **THEN** all columns are filtered to rows matching the search text

### Requirement: Drill-down drawer
The dashboard SHALL display a side drawer with detailed breakdowns when a table row or model bar is clicked.

#### Scenario: Open drill drawer
- **WHEN** a user clicks a table row or model bar
- **THEN** a slide-in drawer from the right shows KPIs, a trend sparkline, and token distribution for that item
