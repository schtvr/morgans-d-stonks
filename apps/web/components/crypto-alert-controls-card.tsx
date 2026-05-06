"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { apiFetch } from "@/lib/api";

type SignalSettings = {
  moveThresholdPct: number;
  cooldown: string;
  updatedAt?: string;
};

function fmtPct(n: number) {
  return `${n.toFixed(2)}%`;
}

export function CryptoAlertControlsCard() {
  const [settings, setSettings] = useState<SignalSettings | null>(null);
  const [threshold, setThreshold] = useState("1.0");
  const [cooldown, setCooldown] = useState("15m");
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  const load = useCallback(async () => {
    setError(null);
    try {
      const res = await apiFetch("/api/trading/alert-settings");
      if (!res.ok) throw new Error(await res.text());
      const json = (await res.json()) as SignalSettings;
      setSettings(json);
      setThreshold(String(json.moveThresholdPct ?? 1.0));
      setCooldown(json.cooldown || "15m");
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load alert settings");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const scopeCards = useMemo(
    () => [
      { label: "Scope", value: "Coinbase crypto only", hint: "Followed symbols" },
      { label: "Storage", value: "signal_settings table", hint: "Persisted in Postgres" },
    ],
    [],
  );

  async function save() {
    setSaving(true);
    setError(null);
    setSuccess(null);
    try {
      const res = await apiFetch("/api/trading/alert-settings", {
        method: "PUT",
        body: JSON.stringify({
          moveThresholdPct: Number(threshold),
          cooldown,
        }),
      });
      if (!res.ok) throw new Error(await res.text());
      const json = (await res.json()) as SignalSettings;
      setSettings(json);
      setThreshold(String(json.moveThresholdPct ?? threshold));
      setCooldown(json.cooldown || cooldown);
      setSuccess("Alert settings saved");
      window.setTimeout(() => setSuccess(null), 2500);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to save alert settings");
    } finally {
      setSaving(false);
    }
  }

  return (
    <Card className="relative overflow-hidden border-border/70">
      <div className="absolute inset-x-0 top-0 h-1 bg-gradient-to-r from-emerald-400 via-cyan-400 to-sky-500" />
      <CardHeader className="space-y-2">
        <CardTitle>Alert controls</CardTitle>
        <CardDescription>
          Tune how chatty the Coinbase alert loop is without touching the service config.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-5">
        <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-1">
          {scopeCards.map((control) => (
            <div key={control.label} className="rounded-lg border bg-muted/30 p-4">
              <p className="text-xs font-medium uppercase tracking-[0.16em] text-muted-foreground">{control.label}</p>
              <p className="mt-2 text-lg font-semibold">{control.value}</p>
              <p className="mt-1 text-xs text-muted-foreground">{control.hint}</p>
            </div>
          ))}
        </div>

        <div className="grid gap-4 sm:grid-cols-2">
          <label className="space-y-2">
            <span className="text-sm font-medium">Minimum move</span>
            <Input
              type="number"
              step="0.1"
              min="0.1"
              value={threshold}
              onChange={(e) => setThreshold(e.target.value)}
              aria-label="Minimum move threshold"
            />
            <p className="text-xs text-muted-foreground">Current: {settings ? fmtPct(settings.moveThresholdPct) : "loading..."}</p>
          </label>
          <label className="space-y-2">
            <span className="text-sm font-medium">Repeat cooldown</span>
            <Input
              value={cooldown}
              onChange={(e) => setCooldown(e.target.value)}
              placeholder="15m"
              aria-label="Repeat cooldown"
            />
            <p className="text-xs text-muted-foreground">Use Go duration syntax like `15m` or `1h`.</p>
          </label>
        </div>

        <div className="flex flex-wrap items-center gap-3">
          <Button type="button" onClick={() => void save()} disabled={saving || loading}>
            {saving ? "Saving..." : "Save settings"}
          </Button>
          <Button type="button" variant="outline" onClick={() => void load()} disabled={loading || saving}>
            Refresh
          </Button>
          {settings?.updatedAt ? <span className="text-xs text-muted-foreground">Updated: {new Date(settings.updatedAt).toLocaleString()}</span> : null}
        </div>

        {error ? <p className="text-sm text-destructive">{error}</p> : null}
        {success ? <p className="text-sm text-emerald-600 dark:text-emerald-400">{success}</p> : null}

        <div className="rounded-lg border border-dashed bg-background/60 p-4">
          <p className="text-sm font-medium">How it works</p>
          <p className="mt-1 text-sm text-muted-foreground">
            The signals service reads these saved values on each tick. If the settings API is unavailable, it falls back to the env defaults.
          </p>
        </div>
      </CardContent>
    </Card>
  );
}
