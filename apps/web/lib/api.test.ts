import { afterEach, describe, expect, it, vi } from "vitest";
import { apiBaseUrl, isCrossOriginPublicAPI } from "./api";

describe("apiBaseUrl", () => {
  it("trims trailing slash from NEXT_PUBLIC_API_URL", () => {
    const prev = process.env.NEXT_PUBLIC_API_URL;
    process.env.NEXT_PUBLIC_API_URL = "http://example.com:8080/";
    expect(apiBaseUrl()).toBe("http://example.com:8080");
    process.env.NEXT_PUBLIC_API_URL = prev;
  });
});

describe("isCrossOriginPublicAPI", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    delete process.env.NEXT_PUBLIC_API_URL;
  });

  it("returns false when NEXT_PUBLIC_API_URL is unset", () => {
    vi.stubGlobal("window", { location: { origin: "http://localhost:3000" } });
    expect(isCrossOriginPublicAPI()).toBe(false);
  });

  it("returns true when public API origin differs from window", () => {
    process.env.NEXT_PUBLIC_API_URL = "http://127.0.0.1:8080";
    vi.stubGlobal("window", { location: { origin: "http://localhost:3000" } });
    expect(isCrossOriginPublicAPI()).toBe(true);
  });
});
