# IB Gateway (stub)

Paper trading gateway for Interactive Brokers. Image and credentials are TBD for local homelab use.

- Expose ports `4001` and `4002` when running in Docker (see root `docker-compose.yml`).
- For development without a gateway, set `IBKR_MODE=mock` in `.env`.
