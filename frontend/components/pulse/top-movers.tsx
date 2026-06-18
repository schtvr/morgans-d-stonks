import { type PositionRow } from "@/components/positions-table";

type PositionWithPct = PositionRow & { dayPct: number };

function fmtPct(v: number) {
  return (v >= 0 ? "+" : "") + v.toFixed(2) + "%";
}

function fmtMoney(v: number, currency: string) {
  return new Intl.NumberFormat(undefined, {
    style: "currency",
    currency: currency || "USD",
    maximumFractionDigits: 0,
  }).format(v);
}

function MoverRow({ p, rank }: { p: PositionWithPct; rank: number }) {
  const ticker = p.symbol.replace(/-USD$/, "").replace(/-USDC$/, "");
  const isUp = p.dayPct >= 0;

  return (
    <div className="flex items-center gap-3 py-2.5">
      <span className="w-5 shrink-0 text-center text-xs font-bold text-muted-foreground">
        #{rank}
      </span>
      <span className="flex-1 font-mono text-sm font-semibold">{ticker}</span>
      <span className="tabular-nums text-xs text-muted-foreground">
        {fmtMoney(p.marketValue, p.currency)}
      </span>
      <span
        className={[
          "w-16 text-right font-mono text-sm font-bold tabular-nums",
          isUp ? "text-green-500" : "text-red-500",
        ].join(" ")}
      >
        {fmtPct(p.dayPct)}
      </span>
    </div>
  );
}

type Props = { positions: PositionRow[] };

export function TopMovers({ positions }: Props) {
  const withPct: PositionWithPct[] = positions
    .filter((p) => p.marketValue > 0)
    .map((p) => {
      const cost = p.marketValue - p.dayPL;
      const dayPct = cost !== 0 ? (p.dayPL / Math.abs(cost)) * 100 : 0;
      return { ...p, dayPct };
    });

  const sorted = [...withPct].sort((a, b) => b.dayPct - a.dayPct);

  const n = Math.min(3, Math.ceil(sorted.length / 2));
  const winners = sorted.slice(0, n);
  const losers = [...sorted].reverse().slice(0, n);

  const NoData = () => (
    <p className="py-4 text-center text-sm text-muted-foreground">No positions</p>
  );

  return (
    <div className="grid gap-4 md:grid-cols-2">
      <div>
        <p className="mb-2 flex items-center gap-1.5 text-sm font-semibold text-green-500">
          <span>▲</span> Top Gainers
        </p>
        <div className="divide-y rounded-xl border bg-card px-3">
          {winners.length > 0 ? (
            winners.map((p, i) => <MoverRow key={p.symbol} p={p} rank={i + 1} />)
          ) : (
            <NoData />
          )}
        </div>
      </div>
      <div>
        <p className="mb-2 flex items-center gap-1.5 text-sm font-semibold text-red-500">
          <span>▼</span> Top Losers
        </p>
        <div className="divide-y rounded-xl border bg-card px-3">
          {losers.length > 0 ? (
            losers.map((p, i) => <MoverRow key={p.symbol} p={p} rank={i + 1} />)
          ) : (
            <NoData />
          )}
        </div>
      </div>
    </div>
  );
}
