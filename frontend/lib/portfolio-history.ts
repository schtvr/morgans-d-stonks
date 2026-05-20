export type PortfolioHistoryRange = "1h" | "1d" | "1w" | "1m" | "3m" | "6m" | "1y";

export type PortfolioHistoryPoint = {
  asOf: string;
  totalValue: number;
  bySymbol?: Record<string, number>;
  currency?: string;
};

export type PortfolioHistoryResponse = {
  range: PortfolioHistoryRange;
  from: string;
  to: string;
  points: PortfolioHistoryPoint[];
};

export const PORTFOLIO_HISTORY_RANGES: { value: PortfolioHistoryRange; label: string }[] = [
  { value: "1h", label: "1h" },
  { value: "1d", label: "1d" },
  { value: "1w", label: "1w" },
  { value: "1m", label: "1m" },
  { value: "3m", label: "3m" },
  { value: "6m", label: "6m" },
  { value: "1y", label: "1y" },
];

export type PortfolioChartRow = {
  asOf: string;
  label: string;
  portfolio: number;
};

export function toPortfolioChartRows(points: PortfolioHistoryPoint[]): PortfolioChartRow[] {
  return points.map((p) => {
    const d = new Date(p.asOf);
    return {
      asOf: p.asOf,
      label: Number.isNaN(d.getTime())
        ? p.asOf
        : d.toLocaleString(undefined, { month: "short", day: "numeric", hour: "2-digit", minute: "2-digit" }),
      portfolio: p.totalValue,
    };
  });
}

/** One OHLCV candle from GET /api/market/candles (Coinbase). */
export type MarketCandlePoint = {
  asOf: string;
  open: number;
  high: number;
  low: number;
  close: number;
  volume: number;
};

export type MarketCandlesResponse = {
  symbol: string;
  productId: string;
  range: PortfolioHistoryRange;
  granularity: string;
  from: string;
  to: string;
  points: MarketCandlePoint[];
};

export type CandleChartRow = {
  asOf: string;
  label: string;
  close: number;
};

export function toCandleChartRows(points: MarketCandlePoint[]): CandleChartRow[] {
  return points.map((p) => {
    const d = new Date(p.asOf);
    return {
      asOf: p.asOf,
      label: Number.isNaN(d.getTime())
        ? p.asOf
        : d.toLocaleString(undefined, { month: "short", day: "numeric", hour: "2-digit", minute: "2-digit" }),
      close: p.close,
    };
  });
}

export function quoteCurrencyFromSymbol(symbol: string): string {
  const s = symbol.trim().toUpperCase();
  const i = s.lastIndexOf("-");
  if (i >= 0 && i < s.length - 1) {
    return s.slice(i + 1);
  }
  return "USD";
}
