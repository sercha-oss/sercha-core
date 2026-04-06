"use client";

import { VespaNodeMetrics } from "@/lib/api";
import { Server, Database } from "lucide-react";
import { cn } from "@/lib/utils";

interface Props {
  nodes: VespaNodeMetrics[];
}

export function VespaNodesTable({ nodes }: Props) {
  const formatBytes = (bytes: number) => {
    if (bytes === 0) return "0 B";
    const k = 1024;
    const sizes = ["B", "KB", "MB", "GB", "TB"];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return `${(bytes / Math.pow(k, i)).toFixed(1)} ${sizes[i]}`;
  };

  if (!nodes || nodes.length === 0) {
    return (
      <section className="rounded-2xl border-2 border-sercha-silverline bg-white p-6">
        <h3 className="mb-4 text-lg font-semibold text-sercha-ink-slate">
          Cluster Nodes
        </h3>
        <p className="text-sercha-fog-grey">No node metrics available</p>
      </section>
    );
  }

  return (
    <section className="rounded-2xl border-2 border-sercha-silverline bg-white">
      <div className="border-b border-sercha-silverline p-6">
        <h3 className="text-lg font-semibold text-sercha-ink-slate">
          Cluster Nodes ({nodes.length})
        </h3>
      </div>
      <div className="overflow-x-auto">
        <table className="w-full">
          <thead>
            <tr className="border-b border-sercha-silverline bg-sercha-snow">
              <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-sercha-fog-grey">
                Node
              </th>
              <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-sercha-fog-grey">
                Role
              </th>
              <th className="px-6 py-3 text-right text-xs font-medium uppercase tracking-wider text-sercha-fog-grey">
                Documents
              </th>
              <th className="px-6 py-3 text-right text-xs font-medium uppercase tracking-wider text-sercha-fog-grey">
                Disk
              </th>
              <th className="px-6 py-3 text-right text-xs font-medium uppercase tracking-wider text-sercha-fog-grey">
                Memory
              </th>
            </tr>
          </thead>
          <tbody className="divide-y divide-sercha-silverline">
            {nodes.map((node, idx) => (
              <tr key={node.hostname || idx} className="hover:bg-sercha-snow">
                <td className="whitespace-nowrap px-6 py-4">
                  <div className="flex items-center gap-3">
                    <div className="rounded-lg bg-sercha-mist p-2">
                      <Server className="h-4 w-4 text-sercha-fog-grey" />
                    </div>
                    <span className="font-mono text-sm text-sercha-ink-slate">
                      {node.hostname || `Node ${idx + 1}`}
                    </span>
                  </div>
                </td>
                <td className="whitespace-nowrap px-6 py-4">
                  <span
                    className={cn(
                      "inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 text-xs font-medium",
                      node.role === "content"
                        ? "bg-blue-50 text-blue-700"
                        : "bg-green-50 text-green-700"
                    )}
                  >
                    {node.role === "content" ? (
                      <Database className="h-3 w-3" />
                    ) : (
                      <Server className="h-3 w-3" />
                    )}
                    {node.role}
                  </span>
                </td>
                <td className="whitespace-nowrap px-6 py-4 text-right font-mono text-sm text-sercha-ink-slate">
                  {node.document_count.toLocaleString()}
                </td>
                <td className="whitespace-nowrap px-6 py-4 text-right">
                  <div className="flex flex-col items-end gap-1">
                    <span className="text-sm font-medium text-sercha-ink-slate">
                      {node.disk_used_percent.toFixed(1)}%
                    </span>
                    <span className="text-xs text-sercha-fog-grey">
                      {formatBytes(node.disk_used_bytes)}
                    </span>
                  </div>
                </td>
                <td className="whitespace-nowrap px-6 py-4 text-right">
                  <div className="flex flex-col items-end gap-1">
                    <span className="text-sm font-medium text-sercha-ink-slate">
                      {node.memory_used_percent.toFixed(1)}%
                    </span>
                    <span className="text-xs text-sercha-fog-grey">
                      {formatBytes(node.memory_used_bytes)}
                    </span>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  );
}
