"use client";

import { cn } from "@/lib/utils";

interface CapabilityCardProps {
  name: string;
  description?: string;
  backend: string;
  phase: "indexing" | "search";
  available: boolean;
  enabled: boolean;
  dependsOn?: string;
  dependencyMet?: boolean;
  onToggle: (enabled: boolean) => void;
  disabled?: boolean;
}

// Toggle switch component
function Toggle({
  enabled,
  onChange,
  disabled,
}: {
  enabled: boolean;
  onChange: (enabled: boolean) => void;
  disabled?: boolean;
}) {
  return (
    <button
      type="button"
      onClick={() => !disabled && onChange(!enabled)}
      disabled={disabled}
      className={cn(
        "relative h-6 w-11 rounded-full transition-colors",
        disabled
          ? "cursor-not-allowed bg-sercha-mist"
          : enabled
            ? "bg-sercha-indigo"
            : "bg-sercha-silverline hover:bg-sercha-fog-grey"
      )}
    >
      <span
        className={cn(
          "absolute top-0.5 h-5 w-5 rounded-full bg-white shadow transition-transform",
          enabled ? "left-[22px]" : "left-0.5"
        )}
      />
    </button>
  );
}

// Health status indicator
function StatusIndicator({ available }: { available: boolean }) {
  return (
    <span className="flex items-center gap-1.5">
      <span
        className={cn(
          "h-2 w-2 rounded-full",
          available ? "bg-emerald-500" : "bg-red-500"
        )}
      />
      <span className="text-xs text-sercha-fog-grey">
        {available ? "Available" : "Unavailable"}
      </span>
    </span>
  );
}

export function CapabilityCard({
  name,
  description,
  backend,
  phase,
  available,
  enabled,
  dependsOn,
  dependencyMet = true,
  onToggle,
  disabled,
}: CapabilityCardProps) {
  const isDisabled = disabled || !available || (dependsOn !== undefined && !dependencyMet);
  const disabledReason = !available
    ? `${backend} unavailable — check backend configuration`
    : dependsOn && !dependencyMet
      ? `Requires ${dependsOn}`
      : undefined;

  return (
    <div
      className={cn(
        "rounded-xl border bg-white p-4 transition-all",
        isDisabled
          ? "border-sercha-mist bg-sercha-snow opacity-75"
          : "border-sercha-silverline hover:border-sercha-indigo hover:shadow-sm"
      )}
    >
      {/* Header with name and toggle */}
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0 flex-1">
          <h3 className="text-sm font-semibold text-sercha-ink-slate">{name}</h3>
          {description && (
            <p className="mt-0.5 text-xs text-sercha-fog-grey">{description}</p>
          )}
        </div>
        <Toggle enabled={enabled} onChange={onToggle} disabled={isDisabled} />
      </div>

      {/* Details */}
      <div className="mt-3 space-y-2">
        {/* Backend */}
        <div className="flex items-center justify-between text-xs">
          <span className="text-sercha-fog-grey">Backend:</span>
          <span className="font-medium text-sercha-ink-slate">{backend}</span>
        </div>

        {/* Status */}
        <div className="flex items-center justify-between text-xs">
          <span className="text-sercha-fog-grey">Status:</span>
          <StatusIndicator available={available} />
        </div>

        {/* Phase badge */}
        <div className="flex items-center justify-between text-xs">
          <span className="text-sercha-fog-grey">Phase:</span>
          <span
            className={cn(
              "rounded-full px-2 py-0.5 text-xs font-medium",
              phase === "indexing"
                ? "bg-purple-100 text-purple-700"
                : "bg-blue-100 text-blue-700"
            )}
          >
            {phase === "indexing" ? "Indexing" : "Search"}
          </span>
        </div>

        {/* Dependency note */}
        {dependsOn && (
          <div className="flex items-center justify-between text-xs">
            <span className="text-sercha-fog-grey">Requires:</span>
            <span
              className={cn(
                "font-medium",
                dependencyMet ? "text-emerald-600" : "text-amber-600"
              )}
            >
              {dependsOn}
            </span>
          </div>
        )}
      </div>

      {/* Disabled reason */}
      {isDisabled && disabledReason && (
        <p className="mt-3 rounded-lg bg-amber-50 px-2 py-1.5 text-xs text-amber-700">
          {disabledReason}
        </p>
      )}
    </div>
  );
}
