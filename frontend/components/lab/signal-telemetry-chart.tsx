"use client";

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import type { LabTelemetryPoint } from "@/lib/lab-api";
import { CartesianGrid, Line, LineChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";

type Props = {
  points: LabTelemetryPoint[];
};

export function SignalTelemetryChart({ points }: Props) {
  const data = points.map((point) => ({
    ...point,
    time: new Date(point.bucket).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" }),
  }));

  return (
    <Card className="border-border/70">
      <CardHeader>
        <CardTitle>Signal telemetry</CardTitle>
        <CardDescription>Price moves and thresholds over the selected window.</CardDescription>
      </CardHeader>
      <CardContent>
        {data.length === 0 ? (
          <div className="flex h-[260px] items-center justify-center rounded-lg border border-dashed text-sm text-muted-foreground">
            No telemetry yet. Qualifying signals will draw this chart.
          </div>
        ) : (
          <div className="h-[300px]">
            <ResponsiveContainer width="100%" height="100%">
              <LineChart data={data} margin={{ left: 8, right: 16, top: 8, bottom: 8 }}>
                <CartesianGrid strokeDasharray="3 3" className="stroke-border" />
                <XAxis dataKey="time" tickLine={false} axisLine={false} fontSize={12} />
                <YAxis tickLine={false} axisLine={false} fontSize={12} />
                <Tooltip
                  contentStyle={{
                    background: "hsl(var(--card))",
                    border: "1px solid hsl(var(--border))",
                    borderRadius: "0.5rem",
                  }}
                />
                <Line type="monotone" dataKey="deltaPct" name="Move %" stroke="hsl(var(--chart-1))" strokeWidth={2} dot={false} />
                <Line type="monotone" dataKey="thresholdPct" name="Threshold %" stroke="hsl(var(--chart-2))" strokeWidth={2} dot={false} />
                <Line type="monotone" dataKey="currentPrice" name="Price" stroke="hsl(var(--chart-3))" strokeWidth={1.5} dot={false} yAxisId={0} />
              </LineChart>
            </ResponsiveContainer>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
