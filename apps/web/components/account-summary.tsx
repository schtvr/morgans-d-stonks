"use client";

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";

export type Summary = {
  accountId: string;
  netLiquidation: number;
  totalCash: number;
  buyingPower: number;
  currency: string;
  asOf?: string;
};

function fmtMoney(n: number, currency: string) {
  return new Intl.NumberFormat(undefined, { style: "currency", currency: currency || "USD" }).format(n);
}

export function AccountSummaryBar({ summary, loading }: { summary: Summary | null; loading: boolean }) {
  if (loading) {
    return (
      <div className="grid gap-3 sm:grid-cols-3">
        {["Net liq", "Cash", "Buying power"].map((t) => (
          <Card key={t}>
            <CardHeader className="pb-2">
              <CardTitle className="text-sm font-medium text-muted-foreground">{t}</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="h-7 w-32 animate-pulse rounded bg-muted" />
            </CardContent>
          </Card>
        ))}
      </div>
    );
  }
  if (!summary) {
    return null;
  }
  return (
    <div className="grid gap-3 sm:grid-cols-3">
      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-sm font-medium text-muted-foreground">Net liquidation</CardTitle>
        </CardHeader>
        <CardContent className="text-2xl font-semibold">{fmtMoney(summary.netLiquidation, summary.currency)}</CardContent>
      </Card>
      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-sm font-medium text-muted-foreground">Total cash</CardTitle>
        </CardHeader>
        <CardContent className="text-2xl font-semibold">{fmtMoney(summary.totalCash, summary.currency)}</CardContent>
      </Card>
      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-sm font-medium text-muted-foreground">Buying power</CardTitle>
        </CardHeader>
        <CardContent className="text-2xl font-semibold">{fmtMoney(summary.buyingPower, summary.currency)}</CardContent>
      </Card>
    </div>
  );
}
