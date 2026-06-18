import { describe, expect, it } from "vitest";
import {
  quoteCurrencyFromSymbol,
  toCandleChartRows,
  toPortfolioChartRows,
  type MarketCandlePoint,
  type PortfolioHistoryPoint,
} from "./portfolio-history";

describe("toPortfolioChartRows", () => {
  it("maps portfolio totals", () => {
    const pts: PortfolioHistoryPoint[] = [{ asOf: "2024-01-01T12:00:00.000Z", totalValue: 100, bySymbol: { "BTC-USD": 40 } }];
    const rows = toPortfolioChartRows(pts);
    expect(rows[0].portfolio).toBe(100);
    expect(rows[0].label).toMatch(/Jan/);
  });
});

describe("toCandleChartRows", () => {
  it("maps close for chart", () => {
    const pts: MarketCandlePoint[] = [
      { asOf: "2024-01-01T12:00:00.000Z", open: 1, high: 2, low: 0.5, close: 1.5, volume: 10 },
    ];
    expect(toCandleChartRows(pts)[0].close).toBe(1.5);
  });
});

describe("quoteCurrencyFromSymbol", () => {
  it("parses quote leg", () => {
    expect(quoteCurrencyFromSymbol("BTC-USD")).toBe("USD");
    expect(quoteCurrencyFromSymbol("ETH-USDC")).toBe("USDC");
  });
});
