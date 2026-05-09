"use client";

import { cn } from "@/lib/utils";

interface SearchModeToggleProps {
  mode: "text" | "document";
  onChange: (mode: "text" | "document") => void;
}

export function SearchModeToggle({ mode, onChange }: SearchModeToggleProps) {
  return (
    <div className="inline-flex rounded-lg border border-sercha-silverline bg-white p-1">
      <button
        type="button"
        onClick={() => onChange("text")}
        className={cn(
          "rounded-md px-4 py-2 text-sm font-medium transition-all",
          mode === "text"
            ? "bg-sercha-indigo text-white shadow-sm"
            : "text-sercha-fog-grey hover:text-sercha-ink-slate"
        )}
      >
        Text Search
      </button>
      <button
        type="button"
        onClick={() => onChange("document")}
        className={cn(
          "rounded-md px-4 py-2 text-sm font-medium transition-all",
          mode === "document"
            ? "bg-sercha-indigo text-white shadow-sm"
            : "text-sercha-fog-grey hover:text-sercha-ink-slate"
        )}
      >
        Document Search
      </button>
    </div>
  );
}
