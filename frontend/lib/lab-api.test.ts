import type { LabSignalEvent } from "@/lib/lab-api";
import { describe, expect, it } from "vitest";

describe("lab api types", () => {
  it("keeps durable signal fields explicit", () => {
    const signal: LabSignalEvent = {
      id: 1,
      type: "crypto_price_move",
      symbol: "BTC-USD",
      currentPrice: 100,
      deltaPct: 1.25,
      thresholdPct: 1,
      firedAt: "2026-01-01T00:00:00Z",
      discordStatus: "signal_only",
      createdAt: "2026-01-01T00:00:01Z",
    };

    expect(signal.symbol).toBe("BTC-USD");
    expect(signal.discordStatus).toBe("signal_only");
  });
});
