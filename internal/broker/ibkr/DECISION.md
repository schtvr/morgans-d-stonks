# IBKR client: TWS API vs Client Portal Web API

## Choice

This project uses the **Interactive Brokers Client Portal Web API** (HTTPS REST on port **5000** by default) for `paper` / `live` modes when a session is available.

## Rationale

- **TWS socket API** (ports 4001/4002) requires implementing the full IB socket protocol, request IDs, and pacing in-process. That is appropriate for a dedicated trading workstation integration but is heavy for a homelab MVP.
- **Client Portal Web API** exposes portfolio and account data over HTTPS with JSON, which maps cleanly to Go `net/http`, keeps wire types isolated in `mapper.go`, and allows TLS to the gateway host.

## Trade-offs

- Client Portal must be enabled and authenticated on the gateway host; session handling may require browser login or scripted auth depending on IBKR version.
- `IBKR_GATEWAY_PORT` remains documented for TWS; REST traffic uses `IBKR_CLIENT_PORTAL_PORT` (default `5000`).
- When Client Portal is unavailable, operators should use `IBKR_MODE=mock` for development and CI.

## References

- IBKR Client Portal Web API documentation (Interactive Brokers)
- TWS API (socket) remains an alternative if we later need streaming market data without REST.
