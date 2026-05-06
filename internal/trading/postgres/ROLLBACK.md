# CB-06 rollback path

To roll back the trading schema introduced by CB-06, remove dependent rows first and then drop the tables in reverse dependency order:

1. Stop the trading worker and disable order endpoints.
2. Delete or archive rows from `reconciliation`, `fills`, and `order_events`.
3. Drop `orders` last.
4. Re-run the service with `TRADING_ENABLED=false` until the schema is recreated.

The migration is intentionally isolated to the trading tables so rollback does not affect the existing portfolio snapshot/session schema.

