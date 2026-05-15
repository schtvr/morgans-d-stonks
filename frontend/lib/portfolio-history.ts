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

export type ChartRow = {
  asOf: string;
  label: string;
  portfolio: number;
  /** Market value for the highlighted symbol, or null when absent in that snapshot. */
  symbolValue: number | null;
};

export function toChartRows(points: PortfolioHistoryPoint[], highlightSymbol: string | null): ChartRow[] {
  return points.map((p) => {
    let symbolValue: number | null = null;
    if (highlightSymbol && p.bySymbol && Object.prototype.hasOwnProperty.call(p.bySymbol, highlightSymbol)) {
      symbolValue = p.bySymbol[highlightSymbol]!;
    }
    const d = new Date(p.asOf);
    return {
      asOf: p.asOf,
      label: Number.isNaN(d.getTime())
        ? p.asOf
        : d.toLocaleString(undefined, { month: "short", day: "numeric", hour: "2-digit", minute: "2-digit" }),
      portfolio: p.totalValue,
      symbolValue,
    };
  });
}
