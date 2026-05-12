"use client";

import { useCallback, useEffect, useState } from "react";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { apiFetch } from "@/lib/api";

type TradeOrder = {
  id: string;
  symbol: string;
  side: string;
  quantity: number;
  notional: number;
  status: string;
  reason?: string;
  createdAt: string;
};

export function TradeActivityCard() {
  const [orders, setOrders] = useState<TradeOrder[]>([]);

  const load = useCallback(async () => {
    const res = await apiFetch("/api/trading/orders/open");
    if (!res.ok) return;
    const json = (await res.json()) as { orders: TradeOrder[] };
    setOrders(json.orders ?? []);
  }, []);

  useEffect(() => {
    void load();
    const id = window.setInterval(() => void load(), 30_000);
    return () => window.clearInterval(id);
  }, [load]);

  return (
    <Card>
      <CardHeader>
        <CardTitle>Trade activity</CardTitle>
        <CardDescription>Open MCP-tracked orders.</CardDescription>
      </CardHeader>
      <CardContent className="space-y-2 text-sm">
        {orders.length === 0 ? <p className="text-muted-foreground">No open orders.</p> : null}
        {orders.map((o) => (
          <div key={o.id} className="rounded border p-2">
            <p className="font-medium">{o.symbol} • {o.side.toUpperCase()} • {o.status}</p>
            <p className="text-muted-foreground">Qty {o.quantity} • Notional {o.notional.toFixed(2)}</p>
          </div>
        ))}
      </CardContent>
    </Card>
  );
}
