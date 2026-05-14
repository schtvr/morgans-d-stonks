"use client";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { apiFetch } from "@/lib/api";
import { fetchSignalSettingsHistory, revertSignalSettings, type SignalSettings, type SignalSettingsVersion } from "@/lib/lab-api";
import { useEffect, useState } from "react";

type Props = {
  onChanged: () => void;
};

export function SignalSettingsLabCard({ onChanged }: Props) {
  const [settings, setSettings] = useState<SignalSettings | null>(null);
  const [versions, setVersions] = useState<SignalSettingsVersion[]>([]);
  const [threshold, setThreshold] = useState("1.0");
  const [cooldown, setCooldown] = useState("15m");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function load() {
    setError(null);
    try {
      const [settingsRes, history] = await Promise.all([apiFetch("/api/trading/alert-settings"), fetchSignalSettingsHistory()]);
      if (!settingsRes.ok) throw new Error(await settingsRes.text());
      const next = (await settingsRes.json()) as SignalSettings;
      setSettings(next);
      setThreshold(String(next.moveThresholdPct ?? 1));
      setCooldown(next.cooldown || "15m");
      setVersions(history.versions ?? []);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load signal settings");
    }
  }

  useEffect(() => {
    void load();
  }, []);

  async function save() {
    setSaving(true);
    setError(null);
    try {
      const res = await apiFetch("/api/trading/alert-settings", {
        method: "PUT",
        body: JSON.stringify({ moveThresholdPct: Number(threshold), cooldown }),
      });
      if (!res.ok) throw new Error(await res.text());
      setSettings((await res.json()) as SignalSettings);
      await load();
      onChanged();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to save signal settings");
    } finally {
      setSaving(false);
    }
  }

  async function revert(version: SignalSettingsVersion) {
    if (!window.confirm(`Revert to ${version.moveThresholdPct}% / ${version.cooldown}?`)) return;
    setSaving(true);
    setError(null);
    try {
      const next = await revertSignalSettings(version.id);
      setSettings(next);
      setThreshold(String(next.moveThresholdPct));
      setCooldown(next.cooldown);
      await load();
      onChanged();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to revert signal settings");
    } finally {
      setSaving(false);
    }
  }

  return (
    <Card className="border-border/70">
      <CardHeader>
        <CardTitle>Signal filters</CardTitle>
        <CardDescription>Live settings used by the signals service on each tick.</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="grid gap-4 sm:grid-cols-2">
          <label className="space-y-2">
            <span className="text-sm font-medium">Minimum move</span>
            <Input type="number" step="0.1" min="0.1" value={threshold} onChange={(e) => setThreshold(e.target.value)} />
          </label>
          <label className="space-y-2">
            <span className="text-sm font-medium">Cooldown</span>
            <Input value={cooldown} onChange={(e) => setCooldown(e.target.value)} placeholder="15m" />
          </label>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Button type="button" disabled={saving} onClick={() => void save()}>
            {saving ? "Saving..." : "Save filters"}
          </Button>
          <Button type="button" variant="outline" disabled={saving} onClick={() => void load()}>
            Refresh
          </Button>
          {settings?.updatedAt ? <span className="text-xs text-muted-foreground">Updated {new Date(settings.updatedAt).toLocaleString()}</span> : null}
        </div>
        {error ? <p className="text-sm text-destructive">{error}</p> : null}
        <div className="rounded-lg border bg-muted/20 p-3">
          <p className="text-sm font-medium">Last known versions</p>
          {versions.length === 0 ? (
            <p className="mt-1 text-sm text-muted-foreground">No saved versions yet.</p>
          ) : (
            <ul className="mt-2 space-y-2">
              {versions.slice(0, 5).map((version) => (
                <li key={version.id} className="flex items-center justify-between gap-3 text-sm">
                  <span>
                    {version.moveThresholdPct}% / {version.cooldown}
                    <span className="ml-2 text-xs text-muted-foreground">{new Date(version.createdAt).toLocaleString()}</span>
                  </span>
                  <Button type="button" size="sm" variant="outline" disabled={saving} onClick={() => void revert(version)}>
                    Revert
                  </Button>
                </li>
              ))}
            </ul>
          )}
        </div>
      </CardContent>
    </Card>
  );
}
