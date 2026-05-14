import { readCollapseState, toggleCollapseState, writeCollapseState } from "@/lib/dashboard-collapse";
import { describe, expect, it } from "vitest";

class MemoryStorage implements Pick<Storage, "getItem" | "setItem" | "removeItem"> {
  private values = new Map<string, string>();

  getItem(key: string) {
    return this.values.get(key) ?? null;
  }

  setItem(key: string, value: string) {
    this.values.set(key, value);
  }

  removeItem(key: string) {
    this.values.delete(key);
  }
}

describe("dashboard collapse state", () => {
  it("persists and toggles section state by id", () => {
    const storage = new MemoryStorage();

    expect(readCollapseState(storage, "dashboard")).toEqual({});

    writeCollapseState(storage, "dashboard", { positions: true });
    expect(readCollapseState(storage, "dashboard")).toEqual({ positions: true });

    expect(toggleCollapseState(storage, "dashboard", "positions")).toBe(false);
    expect(readCollapseState(storage, "dashboard")).toEqual({ positions: false });
  });

  it("ignores malformed stored data", () => {
    const storage = new MemoryStorage();
    storage.setItem("dashboard", "[1,2,3]");

    expect(readCollapseState(storage, "dashboard")).toEqual({});
  });
});
