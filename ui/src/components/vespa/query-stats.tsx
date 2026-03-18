"use client";

import { VespaMetrics } from "@/lib/api";
import { Search, Clock, XCircle, AlertTriangle } from "lucide-react";

interface Props {
  metrics: VespaMetrics;
}

export function VespaQueryStats({ metrics }: Props) {
  const { query_performance: qp } = metrics;

  return (
    <section className="rounded-2xl border-2 border-sercha-silverline bg-white p-6">
      <h3 className="mb-4 text-lg font-semibold text-sercha-ink-slate">
        Query Performance
      </h3>
      <div className="space-y-4">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2 text-sercha-fog-grey">
            <Search className="h-4 w-4" />
            <span className="text-sm">Total Queries</span>
          </div>
          <span className="font-mono text-sm font-medium text-sercha-ink-slate">
            {qp.total_queries.toLocaleString()}
          </span>
        </div>
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2 text-sercha-fog-grey">
            <Clock className="h-4 w-4" />
            <span className="text-sm">Avg Latency</span>
          </div>
          <span className="font-mono text-sm font-medium text-sercha-ink-slate">
            {qp.avg_latency_ms.toFixed(1)} ms
          </span>
        </div>
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2 text-sercha-fog-grey">
            <Search className="h-4 w-4" />
            <span className="text-sm">QPS (Peak)</span>
          </div>
          <span className="font-mono text-sm font-medium text-sercha-ink-slate">
            {qp.queries_per_second.toFixed(2)}
          </span>
        </div>
        <hr className="border-sercha-silverline" />
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2 text-red-500">
            <XCircle className="h-4 w-4" />
            <span className="text-sm">Failed</span>
          </div>
          <span className="font-mono text-sm font-medium text-red-500">
            {qp.failed_queries.toLocaleString()}
          </span>
        </div>
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2 text-amber-500">
            <AlertTriangle className="h-4 w-4" />
            <span className="text-sm">Degraded</span>
          </div>
          <span className="font-mono text-sm font-medium text-amber-500">
            {qp.degraded_queries.toLocaleString()}
          </span>
        </div>
      </div>
    </section>
  );
}
