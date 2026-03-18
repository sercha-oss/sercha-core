"use client";

import { useState, useEffect } from "react";
import Image from "next/image";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Plus, Loader2, RefreshCw, Trash2, MoreVertical, AlertCircle } from "lucide-react";
import { AdminLayout } from "@/components/layout";
import { listSources, deleteSource, triggerSync, SourceSummary } from "@/lib/api";
import { getProviderIcon } from "@/lib/providers";

function formatLastSync(dateStr?: string): string {
  if (!dateStr) return "Never synced";

  const date = new Date(dateStr);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffMins = Math.floor(diffMs / 60000);
  const diffHours = Math.floor(diffMins / 60);
  const diffDays = Math.floor(diffHours / 24);

  if (diffMins < 1) return "Just now";
  if (diffMins < 60) return `${diffMins} min ago`;
  if (diffHours < 24) return `${diffHours} hour${diffHours > 1 ? "s" : ""} ago`;
  return `${diffDays} day${diffDays > 1 ? "s" : ""} ago`;
}

function SourceCard({
  source,
  onSync,
  onDelete,
  syncing,
  onClick,
}: {
  source: SourceSummary;
  onSync: () => void;
  onDelete: () => void;
  syncing: boolean;
  onClick: () => void;
}) {
  const [showMenu, setShowMenu] = useState(false);

  return (
    <div
      onClick={onClick}
      className="group flex cursor-pointer items-center gap-4 rounded-xl border border-sercha-silverline bg-white p-4 transition-all hover:border-sercha-indigo/30 hover:shadow-sm"
    >
      {/* Icon */}
      <div className="flex h-12 w-12 flex-shrink-0 items-center justify-center rounded-lg bg-sercha-snow p-2">
        <Image
          src={getProviderIcon(source.provider_type)}
          alt={source.provider_type}
          width={32}
          height={32}
          className="h-8 w-8 object-contain"
        />
      </div>

      {/* Info */}
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <p className="text-sm font-semibold text-sercha-ink-slate group-hover:text-sercha-indigo">
            {source.name}
          </p>
          {source.status === "syncing" && (
            <span className="h-2 w-2 animate-pulse rounded-full bg-sercha-indigo" />
          )}
          {source.status === "error" && (
            <AlertCircle size={14} className="text-red-500" />
          )}
        </div>
        <p className="text-xs text-sercha-fog-grey">
          {source.document_count.toLocaleString()} documents • {source.provider_type}
        </p>
      </div>

      {/* Last Sync */}
      <div className="text-right">
        <p className="text-xs text-sercha-fog-grey">{formatLastSync(source.last_synced)}</p>
        <p className={`text-xs ${source.enabled ? "text-emerald-600" : "text-sercha-fog-grey"}`}>
          {source.enabled ? "Enabled" : "Disabled"}
        </p>
      </div>

      {/* Actions */}
      <div className="relative" onClick={(e) => e.stopPropagation()}>
        <button
          onClick={() => setShowMenu(!showMenu)}
          className="rounded-lg p-2 text-sercha-fog-grey hover:bg-sercha-mist hover:text-sercha-ink-slate"
        >
          <MoreVertical size={18} />
        </button>

        {showMenu && (
          <>
            <div
              className="fixed inset-0 z-10"
              onClick={() => setShowMenu(false)}
            />
            <div className="absolute right-0 top-full z-20 mt-1 w-40 rounded-lg border border-sercha-silverline bg-white py-1 shadow-lg">
              <button
                onClick={() => {
                  onSync();
                  setShowMenu(false);
                }}
                disabled={syncing}
                className="flex w-full items-center gap-2 px-3 py-2 text-sm text-sercha-ink-slate hover:bg-sercha-mist disabled:opacity-50"
              >
                <RefreshCw size={14} className={syncing ? "animate-spin" : ""} />
                {syncing ? "Syncing..." : "Sync Now"}
              </button>
              <button
                onClick={() => {
                  onDelete();
                  setShowMenu(false);
                }}
                className="flex w-full items-center gap-2 px-3 py-2 text-sm text-red-600 hover:bg-red-50"
              >
                <Trash2 size={14} />
                Delete
              </button>
            </div>
          </>
        )}
      </div>
    </div>
  );
}

export default function SourcesPage() {
  const router = useRouter();
  const [sources, setSources] = useState<SourceSummary[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [syncingIds, setSyncingIds] = useState<Set<string>>(new Set());

  const fetchSources = async () => {
    try {
      setLoading(true);
      setError(null);
      const data = await listSources();
      setSources(data);
    } catch (err) {
      setError("Failed to load sources");
      console.error("Failed to fetch sources:", err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchSources();
  }, []);

  const handleSync = async (id: string) => {
    try {
      setSyncingIds((prev) => new Set(prev).add(id));
      await triggerSync(id);
      // Refresh after a short delay to show new status
      setTimeout(fetchSources, 1000);
    } catch (err) {
      console.error("Failed to trigger sync:", err);
    } finally {
      setSyncingIds((prev) => {
        const next = new Set(prev);
        next.delete(id);
        return next;
      });
    }
  };

  const handleDelete = async (id: string) => {
    if (!confirm("Are you sure you want to delete this source? This will remove all indexed documents.")) {
      return;
    }

    try {
      await deleteSource(id);
      setSources((prev) => prev.filter((s) => s.id !== id));
    } catch (err) {
      console.error("Failed to delete source:", err);
    }
  };

  if (loading) {
    return (
      <AdminLayout title="Sources" description="Manage your data sources">
        <div className="flex items-center justify-center py-12">
          <Loader2 className="h-8 w-8 animate-spin text-sercha-indigo" />
        </div>
      </AdminLayout>
    );
  }

  if (error) {
    return (
      <AdminLayout title="Sources" description="Manage your data sources">
        <div className="flex flex-col items-center justify-center rounded-2xl border border-red-200 bg-red-50 py-12">
          <AlertCircle className="mb-4 h-8 w-8 text-red-500" />
          <p className="text-red-700">{error}</p>
          <button
            onClick={fetchSources}
            className="mt-4 text-sm text-sercha-indigo hover:underline"
          >
            Try again
          </button>
        </div>
      </AdminLayout>
    );
  }

  return (
    <AdminLayout title="Sources" description="Manage your data sources">
      <div className="space-y-6">
        {/* Header with Add button */}
        <div className="flex items-center justify-between">
          <p className="text-sm text-sercha-fog-grey">
            {sources.length} source{sources.length !== 1 ? "s" : ""} connected
          </p>
          <Link
            href="/admin/sources/new"
            className="inline-flex items-center gap-2 rounded-full bg-sercha-indigo px-5 py-2.5 text-sm font-semibold text-white transition-all hover:bg-sercha-indigo/90 hover:shadow-lg"
          >
            <Plus size={18} />
            Add Source
          </Link>
        </div>

        {sources.length === 0 ? (
          /* Empty State */
          <div className="flex flex-col items-center justify-center rounded-2xl border-2 border-dashed border-sercha-silverline bg-white py-16">
            <div className="mb-4 rounded-full bg-sercha-indigo-soft p-4">
              <Plus className="h-8 w-8 text-sercha-indigo" />
            </div>
            <h3 className="mb-2 text-lg font-semibold text-sercha-ink-slate">
              No sources yet
            </h3>
            <p className="mb-6 text-sercha-fog-grey">
              Connect your first data source to start indexing
            </p>
            <Link
              href="/admin/sources/new"
              className="inline-flex items-center gap-2 rounded-full bg-sercha-indigo px-6 py-3 text-sm font-semibold text-white transition-all hover:bg-sercha-indigo/90 hover:shadow-lg"
            >
              <Plus size={18} />
              Add Your First Source
            </Link>
          </div>
        ) : (
          /* Sources List */
          <div className="space-y-3">
            {sources.map((source) => (
              <SourceCard
                key={source.id}
                source={source}
                onSync={() => handleSync(source.id)}
                onDelete={() => handleDelete(source.id)}
                syncing={syncingIds.has(source.id)}
                onClick={() => router.push(`/admin/sources/view?id=${source.id}`)}
              />
            ))}
          </div>
        )}
      </div>
    </AdminLayout>
  );
}
