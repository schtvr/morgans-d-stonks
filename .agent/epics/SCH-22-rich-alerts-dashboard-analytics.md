# SCH-22: Rich Alerts & Dashboard Analytics

> **Linear**: [SCH-22](https://linear.app/schtvr/issue/SCH-22/epic-p1-rich-alerts-and-dashboard-analytics)
> **Milestone**: P1: First follow-up
> **Wave**: 5 (all P0 complete; parallel with SCH-23)
> **Depends on**: P0 complete — specifically SCH-16 (SignalEvent + Discord client), SCH-18 (snapshot persistence), SCH-17 (dashboard stylekit), SCH-21 (snapshot retention)

## Objective

Upgrade the operator experience from P0's minimal `ticker | signal` Discord messages to rich actionable payloads with rationale, levels, and links. Extend the dashboard with performance charts and historical metrics built on top of P0's snapshot history.

## Scope

### Discord rich payloads (`internal/discord/`)

Extend the P0 Discord client (from SCH-16) to support rich embeds:

**Message format** — Discord embed with structured fields:

```go
type RichAlert struct {
    SignalEvent  signal.SignalEvent  // from P0
    Rationale    string              // why the signal fired (human-readable)
    Levels       AlertLevels         // key price levels
    Links        []AlertLink         // relevant links (chart, news, etc.)
    SizingClass  string              // optional: "small", "medium", "large"
    PortfolioCtx PortfolioContext    // position size, portfolio weight
}

type AlertLevels struct {
    Current    float64
    EntryAvg   float64
    Support    float64  // optional, 0 if unknown
    Resistance float64  // optional, 0 if unknown
}

type AlertLink struct {
    Label string
    URL   string
}

type PortfolioContext struct {
    PositionSize  float64  // market value
    PortfolioWeight float64  // percentage of net liq
    DayPL         float64
    TotalPL       float64
}
```

**Discord embed structure**:

```
┌──────────────────────────────────────────┐
│ 🔴 AAPL | 5% Price Drop                 │
│                                          │
│ **Rationale**: Position dropped 5.2%     │
│ from avg cost of $150.25                 │
│                                          │
│ Current   │ Entry Avg │ Day P&L          │
│ $142.40   │ $150.25   │ -$782.50         │
│                                          │
│ Portfolio weight: 14.2% ($14,240)        │
│                                          │
│ 📊 Chart | 📰 News                       │
├──────────────────────────────────────────┤
│ morgans-d-stonks • 2026-04-15 15:30 ET   │
└──────────────────────────────────────────┘
```

**Requirements**:

- Stay within Discord embed limits (title: 256 chars, description: 4096 chars, total embed: 6000 chars).
- Redaction rules: never include account ID, credentials, or internal API URLs in messages.
- Rate limiting: respect Discord's 5 req/2s per webhook (already handled in P0 client — verify).
- Template system: use Go templates or a formatter interface so message layout is configurable without code changes.

### Portfolio API — time-series endpoints

Extend the portfolio API (SCH-18) with new endpoints for chart data:

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/api/portfolio/history` | Session | Portfolio value over time |
| `GET` | `/api/portfolio/positions/:symbol/history` | Session | Per-position price/value history |
| `GET` | `/api/portfolio/metrics` | Session | Computed performance metrics |

**History endpoint** query params:

- `interval`: `1h`, `1d`, `1w`, `1m` (aggregation interval)
- `from`, `to`: ISO-8601 timestamps (default: last 30 days)

**Response shape** (`/api/portfolio/history`):

```json
{
  "series": [
    { "timestamp": "2026-04-15T15:30:00Z", "netLiquidation": 125000.00, "totalCash": 25000.00 },
    { "timestamp": "2026-04-15T15:20:00Z", "netLiquidation": 124800.00, "totalCash": 25000.00 }
  ],
  "interval": "1h",
  "from": "2026-03-15T00:00:00Z",
  "to": "2026-04-15T23:59:59Z"
}
```

**Metrics endpoint** response:

```json
{
  "periodReturns": {
    "day": 0.015,
    "week": -0.023,
    "month": 0.045,
    "ytd": 0.12
  },
  "drawdown": {
    "current": -0.032,
    "max": -0.085,
    "maxDate": "2026-03-10T00:00:00Z"
  },
  "asOf": "2026-04-15T15:30:00Z"
}
```

### Persistence — time-series queries

Add repository methods to query snapshot history (extends SCH-18's repository):

```go
type PortfolioHistory interface {
    // GetValueSeries returns aggregated portfolio values over time.
    GetValueSeries(ctx context.Context, from, to time.Time, interval string) ([]ValuePoint, error)

    // GetPositionHistory returns price/value history for a symbol.
    GetPositionHistory(ctx context.Context, symbol string, from, to time.Time) ([]PositionPoint, error)

    // GetMetrics computes performance metrics from stored snapshots.
    GetMetrics(ctx context.Context) (*PerformanceMetrics, error)
}
```

Query against the existing `snapshots` table from P0. Add indexes if needed for time-range queries.

### Dashboard — charts & metrics (`apps/web/`)

Extend the P0 dashboard with new pages/components:

**Charting library**: Use [Recharts](https://recharts.org/) or [Lightweight Charts](https://tradingview.github.io/lightweight-charts/) — pick one based on bundle size and feature needs. Document the choice.

**New components**:

- **Portfolio value chart** (`/` or `/dashboard`): line chart of net liquidation over time.
  - Time range selector: 1D, 1W, 1M, 3M, YTD, ALL.
  - Tooltip with exact values on hover.
- **Position detail view** (`/positions/:symbol`): per-position chart + stats.
  - Price history line chart.
  - Position stats card (entry, current, P&L, weight).
- **Metrics dashboard** (`/metrics` or sidebar widget):
  - Period returns (day, week, month, YTD).
  - Current drawdown + max drawdown.
  - Color-coded (green/red) like the P0 positions table.

**Layout updates**:

- Add navigation: Dashboard (positions table) | Charts | Metrics.
- Responsive: charts should resize properly; use aspect-ratio containers.

### Caching

- Time-series queries can be expensive. Add a simple in-memory or Redis cache for computed metrics (TTL: 1 minute during market hours, 10 minutes outside).
- Cache key: `metrics:{interval}:{from}:{to}` or similar.

## Do NOT

- Implement auto-trading or order placement (P2, SCH-24).
- Replace or modify P0's deterministic signal rules (SCH-16 owns those).
- Add benchmark overlays (parked — can revisit later).
- Build a mobile native app.
- Implement real-time websocket streaming (polling is acceptable for P1).

## Acceptance criteria

- [ ] Discord alert with rich embed fires on a replayed/fixture signal event.
- [ ] Rich alert includes rationale, levels, portfolio context, and links.
- [ ] Message respects Discord embed size limits (verified with long text).
- [ ] Secrets/internal URLs are redacted from Discord messages.
- [ ] `/api/portfolio/history` returns time-series data with interval aggregation.
- [ ] `/api/portfolio/positions/:symbol/history` returns per-position data.
- [ ] `/api/portfolio/metrics` returns computed period returns and drawdown.
- [ ] Dashboard shows portfolio value chart with time range selector.
- [ ] Dashboard shows at least one multi-interval performance view.
- [ ] Dashboard navigation updated with Charts/Metrics sections.
- [ ] Documented limits: Discord message size, chart data cardinality.
- [ ] Charting library choice documented in README or ADR.

## Shared contracts

This epic **consumes**:

- **SCH-16** `SignalEvent` type — the base event enriched into `RichAlert`.
- **SCH-16** Discord client — extended with embed support.
- **SCH-18** `snapshots` table — queried for time-series data.
- **SCH-18** Portfolio API — extended with new endpoints.
- **SCH-17** Dashboard stylekit — charts use the same theme tokens.
- **SCH-21** Snapshot retention — depends on accumulated history.

This epic **produces**:

- Rich Discord embeds — end-user notifications (replaces P0 minimal format).
- Time-series API endpoints — consumed by the dashboard.
- `RichAlert` type — may be consumed by SCH-23 (OpenClaw) for context.

## Files to create/modify

| File | Action |
|------|--------|
| `internal/discord/rich.go` | Create (rich alert formatter) |
| `internal/discord/rich_test.go` | Create (embed size validation) |
| `internal/discord/templates/` | Create (message templates) |
| `internal/portfolio/history.go` | Create (time-series repository interface) |
| `internal/portfolio/postgres/history.go` | Create (time-series queries) |
| `internal/portfolio/metrics.go` | Create (metrics computation) |
| `internal/portfolio/metrics_test.go` | Create (with fixture snapshots) |
| `cmd/portfolio-api/main.go` | Modify (register new routes) |
| `apps/web/app/dashboard/page.tsx` | Create or modify (chart view) |
| `apps/web/app/positions/[symbol]/page.tsx` | Create (position detail) |
| `apps/web/app/metrics/page.tsx` | Create (metrics view) |
| `apps/web/components/portfolio-chart.tsx` | Create |
| `apps/web/components/position-chart.tsx` | Create |
| `apps/web/components/metrics-cards.tsx` | Create |
| `apps/web/components/time-range-selector.tsx` | Create |
| `apps/web/components/nav.tsx` | Modify (add navigation items) |
