"use client";

import { VespaMetrics } from "@/lib/api";
import { Upload, CheckCircle, XCircle, Clock, Loader2 } from "lucide-react";

interface Props {
  metrics: VespaMetrics;
}

export function VespaFeedStats({ metrics }: Props) {
  const { feed } = metrics;
  const successRate =
    feed.total_operations > 0
      ? ((feed.succeeded_operations / feed.total_operations) * 100).toFixed(1)
      : "100.0";

  return (
    <section className="rounded-2xl border-2 border-sercha-silverline bg-white p-6">
      <h3 className="mb-4 text-lg font-semibold text-sercha-ink-slate">
        Feed/Indexing
      </h3>
      <div className="space-y-4">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2 text-sercha-fog-grey">
            <Upload className="h-4 w-4" />
            <span className="text-sm">Total Operations</span>
          </div>
          <span className="font-mono text-sm font-medium text-sercha-ink-slate">
            {feed.total_operations.toLocaleString()}
          </span>
        </div>
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2 text-emerald-500">
            <CheckCircle className="h-4 w-4" />
            <span className="text-sm">Succeeded</span>
          </div>
          <span className="font-mono text-sm font-medium text-emerald-500">
            {feed.succeeded_operations.toLocaleString()} ({successRate}%)
          </span>
        </div>
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2 text-red-500">
            <XCircle className="h-4 w-4" />
            <span className="text-sm">Failed</span>
          </div>
          <span className="font-mono text-sm font-medium text-red-500">
            {feed.failed_operations.toLocaleString()}
          </span>
        </div>
        <hr className="border-sercha-silverline" />
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2 text-sercha-fog-grey">
            <Loader2 className="h-4 w-4" />
            <span className="text-sm">Pending</span>
          </div>
          <span className="font-mono text-sm font-medium text-sercha-ink-slate">
            {feed.pending_operations.toLocaleString()}
          </span>
        </div>
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2 text-sercha-fog-grey">
            <Clock className="h-4 w-4" />
            <span className="text-sm">Avg Latency</span>
          </div>
          <span className="font-mono text-sm font-medium text-sercha-ink-slate">
            {feed.avg_latency_ms.toFixed(1)} ms
          </span>
        </div>
      </div>
    </section>
  );
}
