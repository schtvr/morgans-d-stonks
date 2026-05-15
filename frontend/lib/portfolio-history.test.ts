import { describe, expect, it } from "vitest";
import { toChartRows, type PortfolioHistoryPoint } from "./portfolio-history";

describe("toChartRows", () => {
  it("maps portfolio and symbol when present", () => {
    const pts: PortfolioHistoryPoint[] = [
      { asOf: "2024-01-01T12:00:00.000Z", totalValue: 100, bySymbol: { AAPL: 40, MSFT: 60 } },
    ];
    const rows = toChartRows(pts, "AAPL");
    expect(rows[0].portfolio).toBe(100);
    expect(rows[0].symbolValue).toBe(40);
  });

  it("uses null when symbol missing in snapshot", () => {
    const pts: PortfolioHistoryPoint[] = [{ asOf: "2024-01-01T12:00:00.000Z", totalValue: 100, bySymbol: { MSFT: 60 } }];
    const rows = toChartRows(pts, "AAPL");
    expect(rows[0].symbolValue).toBeNull();
  });

  it("clears symbol line when no highlight", () => {
    const pts: PortfolioHistoryPoint[] = [{ asOf: "2024-01-01T12:00:00.000Z", totalValue: 100, bySymbol: { AAPL: 40 } }];
    expect(toChartRows(pts, null)[0].symbolValue).toBeNull();
  });
});
