"use client";

import { VespaStatus, VespaMetrics } from "@/lib/api";
import { CheckCircle2, XCircle, HardDrive, Cpu, FileText, Info, Database } from "lucide-react";
import { cn } from "@/lib/utils";

interface Props {
  status: VespaStatus;
  metrics: VespaMetrics;
}

export function VespaOverviewCards({ status, metrics }: Props) {
  const formatBytes = (bytes: number) => {
    if (bytes === 0) return "0 B";
    const k = 1024;
    const sizes = ["B", "KB", "MB", "GB", "TB"];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return `${(bytes / Math.pow(k, i)).toFixed(1)} ${sizes[i]}`;
  };

  return (
    <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-5">
      {/* Connection Status */}
      <div className="rounded-2xl border-2 border-sercha-silverline bg-white p-6">
        <div className="flex items-start justify-between">
          <div>
            <p className="text-sm font-medium text-sercha-fog-grey">Status</p>
            <p
              className={cn(
                "mt-2 text-xl font-bold",
                status.healthy ? "text-emerald-500" : "text-red-500"
              )}
            >
              {status.healthy ? "Connected" : "Disconnected"}
            </p>
            <p className="mt-1 text-sm text-sercha-fog-grey">
              {status.schema_mode === "hybrid" ? "Hybrid Search" : "BM25 Only"}
            </p>
          </div>
          <div
            className={cn(
              "rounded-xl p-3",
              status.healthy ? "bg-emerald-50" : "bg-red-50"
            )}
          >
            {status.healthy ? (
              <CheckCircle2 className="h-6 w-6 text-emerald-500" />
            ) : (
              <XCircle className="h-6 w-6 text-red-500" />
            )}
          </div>
        </div>
      </div>

      {/* Documents */}
      <div className="rounded-2xl border-2 border-sercha-silverline bg-white p-6">
        <div className="flex items-start justify-between">
          <div>
            <div className="flex items-center gap-1">
              <p className="text-sm font-medium text-sercha-fog-grey">Documents</p>
              <div className="group relative">
                <Info className="h-3.5 w-3.5 cursor-help text-sercha-silverline hover:text-sercha-fog-grey" />
                <div className="pointer-events-none absolute bottom-full left-1/2 z-50 mb-2 -translate-x-1/2 opacity-0 transition-opacity group-hover:opacity-100">
                  <div className="whitespace-nowrap rounded-lg bg-sercha-ink-slate px-3 py-2 text-xs text-white shadow-lg">
                    Count of indexed chunks in Vespa, not source files
                  </div>
                  <div className="absolute left-1/2 top-full -translate-x-1/2 border-4 border-transparent border-t-sercha-ink-slate" />
                </div>
              </div>
            </div>
            <p className="mt-2 text-3xl font-bold text-sercha-ink-slate">
              {metrics.documents.total.toLocaleString()}
            </p>
            <p className="mt-1 text-sm text-sercha-fog-grey">
              {metrics.documents.ready.toLocaleString()} ready
            </p>
          </div>
          <div className="rounded-xl bg-sercha-indigo-soft p-3">
            <FileText className="h-6 w-6 text-sercha-indigo" />
          </div>
        </div>
      </div>

      {/* Data Size */}
      <div className="rounded-2xl border-2 border-sercha-silverline bg-white p-6">
        <div className="flex items-start justify-between">
          <div>
            <div className="flex items-center gap-1">
              <p className="text-sm font-medium text-sercha-fog-grey">Data Size</p>
              <div className="group relative">
                <Info className="h-3.5 w-3.5 cursor-help text-sercha-silverline hover:text-sercha-fog-grey" />
                <div className="pointer-events-none absolute bottom-full left-1/2 z-50 mb-2 -translate-x-1/2 opacity-0 transition-opacity group-hover:opacity-100">
                  <div className="whitespace-nowrap rounded-lg bg-sercha-ink-slate px-3 py-2 text-xs text-white shadow-lg">
                    Actual storage used by indexed documents
                  </div>
                  <div className="absolute left-1/2 top-full -translate-x-1/2 border-4 border-transparent border-t-sercha-ink-slate" />
                </div>
              </div>
            </div>
            <p className="mt-2 text-3xl font-bold text-sercha-ink-slate">
              {formatBytes(metrics.storage.data_size_bytes ?? 0)}
            </p>
            <p className="mt-1 text-sm text-sercha-fog-grey">
              indexed data
            </p>
          </div>
          <div className="rounded-xl bg-blue-50 p-3">
            <Database className="h-6 w-6 text-blue-500" />
          </div>
        </div>
      </div>

      {/* Host Disk */}
      <div className="rounded-2xl border-2 border-sercha-silverline bg-white p-6">
        <div className="flex items-start justify-between">
          <div>
            <div className="flex items-center gap-1">
              <p className="text-sm font-medium text-sercha-fog-grey">Host Disk</p>
              <div className="group relative">
                <Info className="h-3.5 w-3.5 cursor-help text-sercha-silverline hover:text-sercha-fog-grey" />
                <div className="pointer-events-none absolute bottom-full left-1/2 z-50 mb-2 -translate-x-1/2 opacity-0 transition-opacity group-hover:opacity-100">
                  <div className="whitespace-nowrap rounded-lg bg-sercha-ink-slate px-3 py-2 text-xs text-white shadow-lg">
                    Filesystem usage where Vespa stores data
                  </div>
                  <div className="absolute left-1/2 top-full -translate-x-1/2 border-4 border-transparent border-t-sercha-ink-slate" />
                </div>
              </div>
            </div>
            <p className="mt-2 text-3xl font-bold text-sercha-ink-slate">
              {metrics.storage.disk_used_percent.toFixed(1)}%
            </p>
            <p className="mt-1 text-sm text-sercha-fog-grey">
              disk pressure
            </p>
          </div>
          <div className="rounded-xl bg-amber-50 p-3">
            <HardDrive className="h-6 w-6 text-amber-500" />
          </div>
        </div>
      </div>

      {/* Memory Usage */}
      <div className="rounded-2xl border-2 border-sercha-silverline bg-white p-6">
        <div className="flex items-start justify-between">
          <div>
            <div className="flex items-center gap-1">
              <p className="text-sm font-medium text-sercha-fog-grey">Memory</p>
              <div className="group relative">
                <Info className="h-3.5 w-3.5 cursor-help text-sercha-silverline hover:text-sercha-fog-grey" />
                <div className="pointer-events-none absolute bottom-full left-1/2 z-50 mb-2 -translate-x-1/2 opacity-0 transition-opacity group-hover:opacity-100">
                  <div className="whitespace-nowrap rounded-lg bg-sercha-ink-slate px-3 py-2 text-xs text-white shadow-lg">
                    Memory used by search indexes and caches
                  </div>
                  <div className="absolute left-1/2 top-full -translate-x-1/2 border-4 border-transparent border-t-sercha-ink-slate" />
                </div>
              </div>
            </div>
            <p className="mt-2 text-3xl font-bold text-sercha-ink-slate">
              {metrics.storage.memory_used_percent.toFixed(1)}%
            </p>
            <p className="mt-1 text-sm text-sercha-fog-grey">
              {formatBytes(metrics.storage.memory_used_bytes)}
            </p>
          </div>
          <div className="rounded-xl bg-purple-50 p-3">
            <Cpu className="h-6 w-6 text-purple-500" />
          </div>
        </div>
      </div>
    </div>
  );
}
