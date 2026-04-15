# SCH-17: Dashboard (Stylekit, Auth, Positions Table)

> **Linear**: [SCH-17](https://linear.app/schtvr/issue/SCH-17/epic-p0-dashboard-stylekit-auth-positions-table)
> **Milestone**: P0: MVP
> **Wave**: 2–3 (stylekit/shell can start with Wave 2; data integration needs SCH-18 API)
> **Depends on**: SCH-19 (layout), SCH-18 (API contract for auth + positions)

## Objective

Build a minimal, well-styled Next.js dashboard with login and a positions table. Adopt a stylekit early so P1 visuals extend without a redesign.

## Scope

### Tech stack

- **Next.js 14+** with App Router and TypeScript.
- **Tailwind CSS** + **shadcn/ui** as the stylekit (preferred; document if you choose otherwise).
- Located in `apps/web/`.

### Stylekit foundation

- Configure Tailwind with a custom theme: color tokens, typography scale, spacing.
- Install and configure shadcn/ui components as needed (Button, Input, Table, Card, etc.).
- Create a base layout with:
  - Dark/light mode support (system preference default).
  - Consistent spacing, max-width container.
  - Navigation header (logo/app name, logout button when authenticated).

### Auth pages

- **Login page** (`/login`):
  - Username + password form.
  - Calls `POST /api/auth/login` on the portfolio API.
  - Stores session token (cookie or localStorage — cookie preferred for httpOnly).
  - Redirects to `/` on success; shows error on failure.
- **Auth middleware/guard**:
  - Unauthenticated users redirect to `/login`.
  - Expired sessions redirect to `/login` with a message.
- **Logout**: calls API, clears session, redirects to `/login`.

### Positions table

- **Route**: `/` (home/dashboard root).
- Fetch positions from `GET /api/portfolio/positions` (portfolio API).
- Table columns: Symbol, Shares, Last Price, Market Value, Day P&L, Total P&L.
- States: loading skeleton, empty state ("No positions"), error state with retry.
- Format numbers: currency with 2 decimals, P&L with color (green positive, red negative).
- Responsive: stack or scroll horizontally on mobile.

### Account summary bar

- Fetch from `GET /api/portfolio/summary`.
- Display: Net Liquidation, Total Cash, Buying Power — as a summary card row above the table.

### API integration

- Use `fetch` or a thin wrapper; no heavy data-fetching library required for P0.
- API base URL from `NEXT_PUBLIC_API_URL` env var (default `http://localhost:8080`).
- Handle 401 responses globally: redirect to login.

### Polling (not websockets)

- Poll positions endpoint every 30–60 seconds while the page is visible.
- Use `visibilitychange` to pause/resume polling.

## Do NOT

- Implement server-side auth logic (SCH-18 owns that).
- Add charts, historical data, or analytics (P1, SCH-22).
- Add real-time websocket connections.
- Build user management or registration flows.

## Acceptance criteria

- [ ] Login page authenticates against the portfolio API.
- [ ] Unauthenticated users cannot see portfolio data.
- [ ] Positions table renders with loading, empty, and error states.
- [ ] Account summary displays above the table.
- [ ] P&L values are color-coded (green/red).
- [ ] Responsive layout works on mobile widths.
- [ ] Stylekit documented: README section on how to add components.
- [ ] Logout works and clears session.

## Shared contracts

This epic consumes APIs defined by **SCH-18** (Portfolio Service):

- `POST /api/auth/login` — `{ username, password }` → `{ token }` or Set-Cookie
- `GET /api/portfolio/positions` — requires session → positions array
- `GET /api/portfolio/summary` — requires session → account summary
- `POST /api/auth/logout` — requires session

Coordinate with SCH-18 if the response shapes change.

## Files to create/modify

| File | Action |
|------|--------|
| `apps/web/package.json` | Configure deps (next, tailwind, shadcn) |
| `apps/web/tailwind.config.ts` | Theme tokens |
| `apps/web/app/layout.tsx` | Root layout with theme provider |
| `apps/web/app/page.tsx` | Dashboard home (positions table) |
| `apps/web/app/login/page.tsx` | Login page |
| `apps/web/components/positions-table.tsx` | Table component |
| `apps/web/components/account-summary.tsx` | Summary cards |
| `apps/web/lib/api.ts` | API client wrapper |
| `apps/web/lib/auth.ts` | Auth helpers (token storage, guards) |
| `apps/web/middleware.ts` | Next.js middleware for auth redirect |
