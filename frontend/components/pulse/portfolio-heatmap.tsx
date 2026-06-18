"use client";

import { type PositionRow } from "@/components/positions-table";
import { useMemo } from "react";
import { ResponsiveContainer, Treemap } from "recharts";

type TreeNode = {
  name: string;
  size: number;
  dayPct: number;
  marketValue: number;
  currency: string;
};

function plColor(pct: number): string {
  if (pct >= 8) return "hsl(142, 72%, 29%)";
  if (pct >= 4) return "hsl(142, 71%, 38%)";
  if (pct >= 1) return "hsl(142, 65%, 48%)";
  if (pct > -1) return "hsl(220, 13%, 26%)";
  if (pct > -4) return "hsl(0, 65%, 42%)";
  if (pct > -8) return "hsl(0, 72%, 34%)";
  return "hsl(0, 76%, 26%)";
}

type CellProps = {
  x?: number;
  y?: number;
  width?: number;
  height?: number;
  name?: string;
  // Recharts passes through all data fields on the node
  [key: string]: unknown;
};

function makeContentRenderer(lookup: Map<string, TreeNode>) {
  return function HeatmapCell(props: CellProps) {
    const { x = 0, y = 0, width = 0, height = 0, name = "" } = props;

    // Always look up from our map — reliable regardless of Recharts cloneElement behavior
    const node = lookup.get(name as string);
    const dayPct = node?.dayPct ?? 0;
    const marketValue = node?.marketValue ?? 0;
    const currency = node?.currency ?? "USD";

    const bg = plColor(dayPct);
    const fmtVal = new Intl.NumberFormat(undefined, {
      style: "currency",
      currency,
      maximumFractionDigits: 0,
    }).format(marketValue);

    const sign = dayPct >= 0 ? "+" : "";
    const showLabel = (width as number) > 44 && (height as number) > 36;
    const showPct = (width as number) > 50 && (height as number) > 48;
    const showValue = (width as number) > 70 && (height as number) > 58;

    const ticker = (name as string).replace(/-USD$/, "").replace(/-USDC$/, "");
    const w = width as number;
    const h = height as number;
    const fontSize = Math.min(14, Math.max(9, w / 6));
    const subFontSize = Math.max(8, fontSize - 2);

    return (
      <g>
        <rect
          x={(x as number) + 1}
          y={(y as number) + 1}
          width={Math.max(0, w - 2)}
          height={Math.max(0, h - 2)}
          rx={4}
          fill={bg}
          style={{ transition: "fill 1.2s ease" }}
        />
        {showLabel && (
          <text
            x={(x as number) + w / 2}
            y={(y as number) + h / 2 - (showPct ? 9 : 4)}
            textAnchor="middle"
            fill="rgba(255,255,255,0.95)"
            fontSize={fontSize}
            fontWeight={700}
            fontFamily="ui-monospace, monospace"
          >
            {ticker}
          </text>
        )}
        {showPct && (
          <text
            x={(x as number) + w / 2}
            y={(y as number) + h / 2 + 8}
            textAnchor="middle"
            fill="rgba(255,255,255,0.8)"
            fontSize={subFontSize}
            fontFamily="ui-monospace, monospace"
          >
            {sign}
            {dayPct.toFixed(2)}%
          </text>
        )}
        {showValue && (
          <text
            x={(x as number) + w / 2}
            y={(y as number) + h / 2 + 23}
            textAnchor="middle"
            fill="rgba(255,255,255,0.5)"
            fontSize={Math.max(8, subFontSize - 1)}
            fontFamily="ui-monospace, monospace"
          >
            {fmtVal}
          </text>
        )}
      </g>
    );
  };
}

type Props = {
  positions: PositionRow[];
  /** Candle-derived day % per symbol from SparklineGrid. Used instead of dayPL (which is always 0 for Coinbase). */
  dayPctMap?: Record<string, number>;
};

export function PortfolioHeatmap({ positions, dayPctMap }: Props) {
  const data = useMemo<TreeNode[]>(
    () =>
      positions
        .filter((p) => p.marketValue > 0)
        .map((p) => {
          // Prefer candle-derived %, fall back to dayPL-derived (likely 0 for Coinbase)
          const dayPct =
            dayPctMap?.[p.symbol] ??
            (() => {
              const cost = p.marketValue - p.dayPL;
              return cost !== 0 ? (p.dayPL / Math.abs(cost)) * 100 : 0;
            })();
          return {
            name: p.symbol,
            size: Math.max(p.marketValue, 1),
            dayPct,
            marketValue: p.marketValue,
            currency: p.currency,
          };
        })
        .sort((a, b) => b.size - a.size),
    [positions, dayPctMap],
  );

  // Build a name→node lookup that the content renderer closes over.
  // This is the reliable path: Recharts `name` prop is always passed to content.
  const lookup = useMemo(() => new Map(data.map((n) => [n.name, n])), [data]);

  // Recreate the renderer whenever the lookup changes so it closes over fresh data.
  const ContentRenderer = useMemo(() => makeContentRenderer(lookup), [lookup]);

  if (data.length === 0) {
    return (
      <div className="flex h-64 items-center justify-center text-sm text-muted-foreground">
        No positions to display
      </div>
    );
  }

  return (
    <div className="h-72 w-full">
      <ResponsiveContainer width="100%" height="100%">
        <Treemap
          data={data}
          dataKey="size"
          content={<ContentRenderer />}
          isAnimationActive={false}
        />
      </ResponsiveContainer>
    </div>
  );
}
