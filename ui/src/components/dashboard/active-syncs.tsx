"use client";

import Image from "next/image";
import { RefreshCw } from "lucide-react";
import { cn } from "@/lib/utils";

type SyncStatus = "syncing" | "queued" | "completed" | "failed";

interface SyncJob {
  id: string;
  sourceName: string;
  sourceIcon: string;
  status: SyncStatus;
  progress: number;
  documentsProcessed: number;
  totalDocuments: number;
  startedAt: string;
}

// Mock sync jobs - in real app, this comes from API
const mockSyncJobs: SyncJob[] = [
  {
    id: "1",
    sourceName: "GitHub",
    sourceIcon: "/logos/github/github_icon.png",
    status: "syncing",
    progress: 67,
    documentsProcessed: 402,
    totalDocuments: 600,
    startedAt: "2 min ago",
  },
  {
    id: "2",
    sourceName: "GitLab",
    sourceIcon: "/logos/gitlab/gitlab_logo.png",
    status: "queued",
    progress: 0,
    documentsProcessed: 0,
    totalDocuments: 150,
    startedAt: "Queued",
  },
];

const statusConfig: Record<SyncStatus, { label: string; color: string; bgColor: string }> = {
  syncing: { label: "Syncing", color: "text-sercha-indigo", bgColor: "bg-sercha-indigo" },
  queued: { label: "Queued", color: "text-sercha-fog-grey", bgColor: "bg-sercha-silverline" },
  completed: { label: "Completed", color: "text-emerald-500", bgColor: "bg-emerald-500" },
  failed: { label: "Failed", color: "text-red-500", bgColor: "bg-red-500" },
};

export function ActiveSyncs() {
  const hasActiveSyncs = mockSyncJobs.length > 0;

  return (
    <div className="rounded-2xl border border-sercha-silverline bg-white p-6">
      <div className="mb-4 flex items-center justify-between">
        <h3 className="text-sm font-medium text-sercha-fog-grey">Active Syncs</h3>
        {hasActiveSyncs && (
          <span className="flex items-center gap-1 text-xs text-sercha-indigo">
            <RefreshCw size={12} className="animate-spin" />
            {mockSyncJobs.filter((j) => j.status === "syncing").length} running
          </span>
        )}
      </div>

      {!hasActiveSyncs ? (
        <div className="flex flex-col items-center justify-center py-8 text-center">
          <RefreshCw size={32} className="mb-2 text-sercha-silverline" />
          <p className="text-sm text-sercha-fog-grey">No active syncs</p>
          <p className="text-xs text-sercha-silverline">
            All sources are up to date
          </p>
        </div>
      ) : (
        <div className="space-y-4">
          {mockSyncJobs.map((job) => {
            const config = statusConfig[job.status];
            return (
              <div key={job.id} className="space-y-2">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <Image
                      src={job.sourceIcon}
                      alt={job.sourceName}
                      width={20}
                      height={20}
                      className="h-5 w-5"
                    />
                    <span className="text-sm font-medium text-sercha-ink-slate">
                      {job.sourceName}
                    </span>
                  </div>
                  <div className="flex items-center gap-2">
                    <span className={cn("text-xs", config.color)}>
                      {config.label}
                    </span>
                    <span className="text-xs text-sercha-silverline">
                      {job.startedAt}
                    </span>
                  </div>
                </div>

                {/* Progress bar */}
                <div className="relative h-2 overflow-hidden rounded-full bg-sercha-mist">
                  <div
                    className={cn(
                      "absolute left-0 top-0 h-full rounded-full transition-all duration-500",
                      config.bgColor,
                      job.status === "syncing" && "animate-pulse"
                    )}
                    style={{ width: `${job.progress}%` }}
                  />
                </div>

                <div className="flex items-center justify-between text-xs text-sercha-fog-grey">
                  <span>
                    {job.documentsProcessed.toLocaleString()} / {job.totalDocuments.toLocaleString()} documents
                  </span>
                  <span>{job.progress}%</span>
                </div>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
