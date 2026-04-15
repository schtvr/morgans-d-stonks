"use client";

import { Button } from "@/components/ui/button";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";

export type PositionRow = {
  symbol: string;
  quantity: number;
  avgCost: number;
  lastPrice: number;
  marketValue: number;
  dayPL: number;
  totalPL: number;
  currency: string;
};

function fmtMoney(n: number, currency: string) {
  return new Intl.NumberFormat(undefined, { style: "currency", currency: currency || "USD" }).format(n);
}

function plClass(n: number) {
  if (n > 0) return "text-emerald-600 dark:text-emerald-400";
  if (n < 0) return "text-red-600 dark:text-red-400";
  return "text-muted-foreground";
}

export function PositionsTable({
  positions,
  loading,
  error,
  onRetry,
}: {
  positions: PositionRow[];
  loading: boolean;
  error: string | null;
  onRetry: () => void;
}) {
  if (loading) {
    return (
      <div className="space-y-2">
        <div className="h-10 w-full animate-pulse rounded bg-muted" />
        <div className="h-40 w-full animate-pulse rounded bg-muted" />
      </div>
    );
  }
  if (error) {
    return (
      <div className="rounded-md border border-destructive/40 bg-destructive/10 p-4">
        <p className="text-sm text-destructive">{error}</p>
        <Button className="mt-3" variant="outline" onClick={onRetry}>
          Retry
        </Button>
      </div>
    );
  }
  if (positions.length === 0) {
    return <p className="text-sm text-muted-foreground">No positions</p>;
  }
  return (
    <div className="overflow-x-auto rounded-md border">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Symbol</TableHead>
            <TableHead className="text-right">Shares</TableHead>
            <TableHead className="text-right">Last</TableHead>
            <TableHead className="text-right">Market value</TableHead>
            <TableHead className="text-right">Day P&amp;L</TableHead>
            <TableHead className="text-right">Total P&amp;L</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {positions.map((p) => (
            <TableRow key={p.symbol}>
              <TableCell className="font-medium">{p.symbol}</TableCell>
              <TableCell className="text-right">{p.quantity.toLocaleString()}</TableCell>
              <TableCell className="text-right">{fmtMoney(p.lastPrice, p.currency)}</TableCell>
              <TableCell className="text-right">{fmtMoney(p.marketValue, p.currency)}</TableCell>
              <TableCell className={`text-right ${plClass(p.dayPL)}`}>{fmtMoney(p.dayPL, p.currency)}</TableCell>
              <TableCell className={`text-right ${plClass(p.totalPL)}`}>{fmtMoney(p.totalPL, p.currency)}</TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}
