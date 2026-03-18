"use client";

import Image from "next/image";
import Link from "next/link";
import { Plus } from "lucide-react";

// Source type definition
interface Source {
  id: string;
  name: string;
  icon: string;
  documents: number;
  status: "synced" | "syncing" | "error";
  lastSync: string;
}

// Mock connected sources - in real app, this comes from API
const mockSources: Source[] = [
  {
    id: "1",
    name: "GitHub",
    icon: "/logos/github/github_icon.png",
    documents: 892,
    status: "syncing",
    lastSync: "2 min ago",
  },
  {
    id: "2",
    name: "GitLab",
    icon: "/logos/gitlab/gitlab_logo.png",
    documents: 245,
    status: "synced",
    lastSync: "1 hour ago",
  },
  {
    id: "3",
    name: "Notion",
    icon: "/logos/notion/notion_icon.png",
    documents: 97,
    status: "synced",
    lastSync: "3 hours ago",
  },
];

function SourceCard({ source }: { source: Source }) {
  return (
    <Link
      href={`/admin/sources/${source.id}`}
      className="group flex items-center gap-4 rounded-xl border border-sercha-silverline bg-white p-4 transition-all hover:border-sercha-indigo hover:shadow-md"
    >
      {/* Icon */}
      <div className="flex h-12 w-12 flex-shrink-0 items-center justify-center rounded-lg bg-sercha-snow p-2">
        <Image
          src={source.icon}
          alt={source.name}
          width={32}
          height={32}
          className="h-8 w-8 object-contain"
        />
      </div>

      {/* Info */}
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <p className="text-sm font-semibold text-sercha-ink-slate">
            {source.name}
          </p>
          {source.status === "syncing" && (
            <span className="h-2 w-2 animate-pulse rounded-full bg-sercha-indigo" />
          )}
        </div>
        <p className="text-xs text-sercha-fog-grey">
          {source.documents.toLocaleString()} documents
        </p>
      </div>

      {/* Status */}
      <div className="text-right">
        <p className="text-xs text-sercha-fog-grey">{source.lastSync}</p>
      </div>
    </Link>
  );
}

export function ConnectedSources() {
  return (
    <div className="rounded-2xl border border-sercha-silverline bg-white p-6">
      <div className="mb-4 flex items-center justify-between">
        <h3 className="text-sm font-medium text-sercha-fog-grey">
          Connected Sources
        </h3>
        <Link
          href="/admin/sources"
          className="text-xs text-sercha-indigo hover:underline"
        >
          View all
        </Link>
      </div>

      <div className="space-y-3">
        {mockSources.map((source) => (
          <SourceCard key={source.id} source={source} />
        ))}

        {/* Add Source Button */}
        <Link
          href="/admin/sources/new"
          className="flex items-center gap-4 rounded-xl border-2 border-dashed border-sercha-silverline p-4 transition-all hover:border-sercha-indigo hover:bg-sercha-mist"
        >
          <div className="flex h-12 w-12 flex-shrink-0 items-center justify-center rounded-lg bg-sercha-mist">
            <Plus size={24} className="text-sercha-fog-grey" />
          </div>
          <div>
            <p className="text-sm font-medium text-sercha-fog-grey">
              Add Source
            </p>
            <p className="text-xs text-sercha-silverline">
              Connect a new data source
            </p>
          </div>
        </Link>
      </div>
    </div>
  );
}
