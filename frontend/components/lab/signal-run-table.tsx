"use client";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import type { LabOpenClawRun, LabSignalEvent } from "@/lib/lab-api";

type Props = {
  signals: LabSignalEvent[];
  runs: LabOpenClawRun[];
  onSelectRun: (run: LabOpenClawRun) => void;
};

export function SignalRunTable({ signals, runs, onSelectRun }: Props) {
  const runBySignal = new Map(runs.map((run) => [run.signalId, run]));

  return (
    <Card className="border-border/70">
      <CardHeader>
        <CardTitle>Signals and responses</CardTitle>
        <CardDescription>Qualifying signals, their current OpenClaw run state, and the Discord-gated alert history.</CardDescription>
      </CardHeader>
      <CardContent>
        <div className="rounded-md border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Time</TableHead>
                <TableHead>Symbol</TableHead>
                <TableHead>Move</TableHead>
                <TableHead>Threshold</TableHead>
                <TableHead>OpenClaw</TableHead>
                <TableHead className="text-right">Action</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {signals.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={6} className="py-8 text-center text-sm text-muted-foreground">
                    No Lab signals yet.
                  </TableCell>
                </TableRow>
              ) : (
                signals.map((signal) => {
                  const run = runBySignal.get(signal.id);
                  return (
                    <TableRow key={signal.id}>
                      <TableCell className="whitespace-nowrap text-xs text-muted-foreground">{new Date(signal.firedAt).toLocaleString()}</TableCell>
                      <TableCell className="font-medium">{signal.symbol}</TableCell>
                      <TableCell>{fmtPct(signal.deltaPct)}</TableCell>
                      <TableCell>{fmtPct(signal.thresholdPct)}</TableCell>
                      <TableCell>
                        <span className="rounded-full border px-2 py-1 text-xs">{run?.status ?? "not queued"}</span>
                      </TableCell>
                      <TableCell className="text-right">
                        {run ? (
                          <Button type="button" variant="outline" size="sm" onClick={() => onSelectRun(run)}>
                            Details
                          </Button>
                        ) : null}
                      </TableCell>
                    </TableRow>
                  );
                })
              )}
            </TableBody>
          </Table>
        </div>
      </CardContent>
    </Card>
  );
}

function fmtPct(value: number) {
  return `${value >= 0 ? "+" : ""}${value.toFixed(2)}%`;
}
