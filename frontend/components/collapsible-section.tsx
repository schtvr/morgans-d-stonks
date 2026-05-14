"use client";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { readCollapseState, toggleCollapseState } from "@/lib/dashboard-collapse";
import { cn } from "@/lib/utils";
import { ChevronDown, ChevronRight } from "lucide-react";
import { useEffect, useState } from "react";

type CollapsibleSectionProps = {
  storageKey: string;
  sectionId: string;
  title: string;
  description?: string;
  children: React.ReactNode;
  className?: string;
  contentClassName?: string;
  defaultCollapsed?: boolean;
};

export function CollapsibleSection({
  storageKey,
  sectionId,
  title,
  description,
  children,
  className,
  contentClassName,
  defaultCollapsed = false,
}: CollapsibleSectionProps) {
  const [collapsed, setCollapsed] = useState(defaultCollapsed);

  useEffect(() => {
    setCollapsed(readCollapseState(window.localStorage, storageKey)[sectionId] ?? defaultCollapsed);
  }, [defaultCollapsed, sectionId, storageKey]);

  function toggle() {
    setCollapsed(toggleCollapseState(window.localStorage, storageKey, sectionId));
  }

  return (
    <Card className={className}>
      <CardHeader className="flex-row items-start justify-between gap-4 space-y-0">
        <div className="space-y-1.5">
          <CardTitle>{title}</CardTitle>
          {description ? <CardDescription>{description}</CardDescription> : null}
        </div>
        <Button type="button" variant="ghost" size="sm" onClick={toggle} aria-expanded={!collapsed} aria-controls={`${sectionId}-content`}>
          {collapsed ? <ChevronRight className="h-4 w-4" /> : <ChevronDown className="h-4 w-4" />}
          {collapsed ? "Show" : "Hide"}
        </Button>
      </CardHeader>
      {!collapsed ? (
        <CardContent id={`${sectionId}-content`} className={cn("space-y-4", contentClassName)}>
          {children}
        </CardContent>
      ) : null}
    </Card>
  );
}
