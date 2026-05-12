"use client";

import Link from "next/link";
import { Moon, Sun } from "lucide-react";
import { useTheme } from "next-themes";
import { Button } from "@/components/ui/button";
import { clearSessionMarker } from "@/lib/auth";
import { apiFetch, clearCrossOriginBearerToken } from "@/lib/api";

export function SiteHeader() {
  const { theme, setTheme } = useTheme();

  async function logout() {
    try {
      await apiFetch("/api/auth/logout", { method: "POST" });
    } catch {
      // ignore
    }
    clearSessionMarker();
    clearCrossOriginBearerToken();
    window.location.href = "/login";
  }

  return (
    <header className="border-b bg-card">
      <div className="mx-auto flex max-w-6xl items-center justify-between gap-4 px-4 py-3">
        <Link href="/" className="text-lg font-semibold tracking-tight">
          Morgans D. Stonks
        </Link>
        <div className="flex items-center gap-2">
          <Button
            type="button"
            variant="ghost"
            size="icon"
            aria-label="Toggle theme"
            onClick={() => setTheme(theme === "dark" ? "light" : "dark")}
          >
            <Sun className="h-4 w-4 dark:hidden" />
            <Moon className="hidden h-4 w-4 dark:inline" />
          </Button>
          <Button type="button" variant="outline" onClick={() => void logout()}>
            Log out
          </Button>
        </div>
      </div>
    </header>
  );
}
