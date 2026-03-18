"use client";

import { useState, useEffect, useCallback, Suspense } from "react";
import { useSearchParams, useRouter } from "next/navigation";
import Image from "next/image";
import Link from "next/link";
import {
  ArrowLeft,
  RefreshCw,
  Trash2,
  Power,
  PowerOff,
  Loader2,
  AlertCircle,
  FileText,
  Clock,
  ExternalLink,
  ChevronLeft,
  ChevronRight,
  Database,
  CheckCircle,
} from "lucide-react";
import { AdminLayout } from "@/components/layout";
import {
  getSourceSyncState,
  triggerSync,
  deleteSource,
  enableSource,
  disableSource,
  getSourceDocuments,
  getDocumentURL,
  listSources,
  SourceSummary,
  SyncState,
  Document,
} from "@/lib/api";
import { getProviderIcon } from "@/lib/providers";

function formatRelativeTime(dateStr?: string): string {
  if (!dateStr) return "Never";

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

function formatProviderName(providerType: string): string {
  const names: Record<string, string> = {
    github: "GitHub",
    gitlab: "GitLab",
    notion: "Notion",
    google_drive: "Google Drive",
    dropbox: "Dropbox",
    confluence: "Confluence",
    jira: "Jira",
    asana: "Asana",
    linear: "Linear",
    figma: "Figma",
    miro: "Miro",
    onedrive: "OneDrive",
    sharepoint: "SharePoint",
  };
  return names[providerType] || providerType;
}

const PAGE_SIZE = 20;

function SourceDetailContent() {
  const searchParams = useSearchParams();
  const router = useRouter();
  const sourceId = searchParams.get("id");

  // State
  const [source, setSource] = useState<SourceSummary | null>(null);
  const [syncState, setSyncState] = useState<SyncState | null>(null);
  const [documents, setDocuments] = useState<Document[]>([]);
  const [totalDocs, setTotalDocs] = useState(0);
  const [page, setPage] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [syncing, setSyncing] = useState(false);
  const [toggling, setToggling] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [openingDoc, setOpeningDoc] = useState<string | null>(null);
  const [loadingDocs, setLoadingDocs] = useState(false);

  // Fetch source data
  const fetchSource = useCallback(async () => {
    if (!sourceId) return;
    try {
      const sources = await listSources();
      const found = sources.find((s) => s.id === sourceId);
      if (!found) {
        setError("Source not found");
        return;
      }
      setSource(found);
    } catch (err) {
      console.error("Failed to fetch source:", err);
      setError("Failed to load source");
    }
  }, [sourceId]);

  // Fetch sync state
  const fetchSyncState = useCallback(async () => {
    if (!sourceId) return;
    try {
      const state = await getSourceSyncState(sourceId);
      setSyncState(state);
    } catch (err) {
      console.error("Failed to fetch sync state:", err);
    }
  }, [sourceId]);

  // Fetch documents
  const fetchDocuments = useCallback(async () => {
    if (!sourceId) return;
    try {
      setLoadingDocs(true);
      const response = await getSourceDocuments(sourceId, {
        limit: PAGE_SIZE,
        offset: page * PAGE_SIZE,
      });
      setDocuments(response.documents || []);
      setTotalDocs(response.total || 0);
    } catch (err) {
      console.error("Failed to fetch documents:", err);
    } finally {
      setLoadingDocs(false);
    }
  }, [sourceId, page]);

  // Initial load
  useEffect(() => {
    if (!sourceId) {
      setError("No source ID provided");
      setLoading(false);
      return;
    }
    const loadAll = async () => {
      setLoading(true);
      await fetchSource();
      await fetchSyncState();
      await fetchDocuments();
      setLoading(false);
    };
    loadAll();
  }, [sourceId, fetchSource, fetchSyncState, fetchDocuments]);

  // Refetch documents when page changes
  useEffect(() => {
    if (!loading && sourceId) {
      fetchDocuments();
    }
  }, [page, fetchDocuments, loading, sourceId]);

  // Auto-refresh sync state while syncing
  useEffect(() => {
    if (source?.status === "syncing" || syncState?.status === "syncing") {
      const interval = setInterval(() => {
        fetchSource();
        fetchSyncState();
      }, 5000);
      return () => clearInterval(interval);
    }
  }, [source?.status, syncState?.status, fetchSource, fetchSyncState]);

  // Handlers
  const handleSync = async () => {
    if (!sourceId) return;
    setSyncing(true);
    try {
      await triggerSync(sourceId);
      setTimeout(() => {
        fetchSource();
        fetchSyncState();
      }, 1000);
    } catch (err) {
      console.error("Failed to trigger sync:", err);
    } finally {
      setSyncing(false);
    }
  };

  const handleToggleEnabled = async () => {
    if (!source || !sourceId) return;
    setToggling(true);
    try {
      if (source.enabled) {
        await disableSource(sourceId);
      } else {
        await enableSource(sourceId);
      }
      await fetchSource();
    } catch (err) {
      console.error("Failed to toggle source:", err);
    } finally {
      setToggling(false);
    }
  };

  const handleDelete = async () => {
    if (!sourceId) return;
    if (!confirm("Are you sure you want to delete this source? This will remove all indexed documents and cannot be undone.")) {
      return;
    }
    setDeleting(true);
    try {
      await deleteSource(sourceId);
      router.push("/admin/sources");
    } catch (err) {
      console.error("Failed to delete source:", err);
      setDeleting(false);
    }
  };

  const handleOpenDocument = async (docId: string) => {
    setOpeningDoc(docId);
    try {
      const url = await getDocumentURL(docId);
      window.open(url, "_blank");
    } catch (err) {
      console.error("Failed to open document:", err);
    } finally {
      setOpeningDoc(null);
    }
  };

  // Loading state
  if (loading) {
    return (
      <AdminLayout title="Source" description="Loading...">
        <div className="flex items-center justify-center py-12">
          <Loader2 className="h-8 w-8 animate-spin text-sercha-indigo" />
        </div>
      </AdminLayout>
    );
  }

  // Error state
  if (error || !source) {
    return (
      <AdminLayout title="Source" description="Error">
        <div className="flex flex-col items-center justify-center rounded-2xl border border-red-200 bg-red-50 py-12">
          <AlertCircle className="mb-4 h-8 w-8 text-red-500" />
          <p className="text-red-700">{error || "Source not found"}</p>
          <Link
            href="/admin/sources"
            className="mt-4 text-sm text-sercha-indigo hover:underline"
          >
            Back to Sources
          </Link>
        </div>
      </AdminLayout>
    );
  }

  const isSyncing = source.status === "syncing" || syncState?.status === "syncing" || syncing;
  const totalPages = Math.ceil(totalDocs / PAGE_SIZE);
  const startItem = page * PAGE_SIZE + 1;
  const endItem = Math.min((page + 1) * PAGE_SIZE, totalDocs);

  return (
    <AdminLayout
      title={source.name}
      description={`Manage ${formatProviderName(source.provider_type)} source`}
    >
      <div className="space-y-6">
        {/* Back Link */}
        <Link
          href="/admin/sources"
          className="inline-flex items-center gap-1 text-sm text-sercha-fog-grey hover:text-sercha-indigo"
        >
          <ArrowLeft size={16} />
          Back to Sources
        </Link>

        {/* Header */}
        <div className="flex items-start justify-between rounded-2xl border border-sercha-silverline bg-white p-6">
          <div className="flex items-center gap-4">
            <div className="flex h-16 w-16 items-center justify-center rounded-xl bg-sercha-snow p-3">
              <Image
                src={getProviderIcon(source.provider_type)}
                alt={source.provider_type}
                width={40}
                height={40}
                className="h-10 w-10 object-contain"
              />
            </div>
            <div>
              <div className="flex items-center gap-3">
                <h2 className="text-xl font-bold text-sercha-ink-slate">
                  {source.name}
                </h2>
                {isSyncing && (
                  <span className="flex items-center gap-1.5 rounded-full bg-sercha-indigo-soft px-2.5 py-1 text-xs font-medium text-sercha-indigo">
                    <RefreshCw size={12} className="animate-spin" />
                    Syncing
                  </span>
                )}
                {source.status === "error" && (
                  <span className="flex items-center gap-1.5 rounded-full bg-red-100 px-2.5 py-1 text-xs font-medium text-red-600">
                    <AlertCircle size={12} />
                    Error
                  </span>
                )}
              </div>
              <p className="mt-1 text-sm text-sercha-fog-grey">
                {formatProviderName(source.provider_type)} •{" "}
                {source.enabled ? (
                  <span className="text-emerald-600">Enabled</span>
                ) : (
                  <span className="text-sercha-fog-grey">Disabled</span>
                )}
              </p>
            </div>
          </div>

          {/* Action Buttons */}
          <div className="flex items-center gap-2">
            <button
              onClick={handleSync}
              disabled={isSyncing}
              className="inline-flex items-center gap-2 rounded-lg border border-sercha-silverline bg-white px-4 py-2 text-sm font-medium text-sercha-ink-slate transition-all hover:bg-sercha-mist disabled:opacity-50"
            >
              <RefreshCw size={16} className={isSyncing ? "animate-spin" : ""} />
              {isSyncing ? "Syncing..." : "Sync Now"}
            </button>
            <button
              onClick={handleToggleEnabled}
              disabled={toggling}
              className={`inline-flex items-center gap-2 rounded-lg border px-4 py-2 text-sm font-medium transition-all disabled:opacity-50 ${
                source.enabled
                  ? "border-sercha-silverline bg-white text-sercha-ink-slate hover:bg-sercha-mist"
                  : "border-emerald-200 bg-emerald-50 text-emerald-700 hover:bg-emerald-100"
              }`}
            >
              {toggling ? (
                <Loader2 size={16} className="animate-spin" />
              ) : source.enabled ? (
                <PowerOff size={16} />
              ) : (
                <Power size={16} />
              )}
              {source.enabled ? "Disable" : "Enable"}
            </button>
            <button
              onClick={handleDelete}
              disabled={deleting}
              className="inline-flex items-center gap-2 rounded-lg border border-red-200 bg-white px-4 py-2 text-sm font-medium text-red-600 transition-all hover:bg-red-50 disabled:opacity-50"
            >
              {deleting ? (
                <Loader2 size={16} className="animate-spin" />
              ) : (
                <Trash2 size={16} />
              )}
              Delete
            </button>
          </div>
        </div>

        {/* Stats Cards */}
        <div className="grid grid-cols-3 gap-4">
          <div className="rounded-xl border border-sercha-silverline bg-white p-4">
            <div className="flex items-center gap-3">
              <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-sercha-indigo-soft">
                <Database size={20} className="text-sercha-indigo" />
              </div>
              <div>
                <p className="text-2xl font-bold text-sercha-ink-slate">
                  {source.document_count.toLocaleString()}
                </p>
                <p className="text-sm text-sercha-fog-grey">Documents</p>
              </div>
            </div>
          </div>
          <div className="rounded-xl border border-sercha-silverline bg-white p-4">
            <div className="flex items-center gap-3">
              <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-sercha-indigo-soft">
                <Clock size={20} className="text-sercha-indigo" />
              </div>
              <div>
                <p className="text-2xl font-bold text-sercha-ink-slate">
                  {formatRelativeTime(source.last_synced)}
                </p>
                <p className="text-sm text-sercha-fog-grey">Last Synced</p>
              </div>
            </div>
          </div>
          <div className="rounded-xl border border-sercha-silverline bg-white p-4">
            <div className="flex items-center gap-3">
              <div className={`flex h-10 w-10 items-center justify-center rounded-lg ${
                source.status === "error" ? "bg-red-100" : "bg-emerald-100"
              }`}>
                {source.status === "error" ? (
                  <AlertCircle size={20} className="text-red-600" />
                ) : (
                  <CheckCircle size={20} className="text-emerald-600" />
                )}
              </div>
              <div>
                <p className={`text-2xl font-bold ${
                  source.status === "error" ? "text-red-600" : "text-emerald-600"
                }`}>
                  {source.status === "error" ? "Error" : "Healthy"}
                </p>
                <p className="text-sm text-sercha-fog-grey">Status</p>
              </div>
            </div>
          </div>
        </div>

        {/* Sync Error Alert */}
        {syncState?.status === "error" && syncState.error_message && (
          <div className="flex items-start gap-3 rounded-xl border border-red-200 bg-red-50 p-4">
            <AlertCircle size={20} className="mt-0.5 flex-shrink-0 text-red-600" />
            <div>
              <p className="font-medium text-red-800">Sync Error</p>
              <p className="mt-1 text-sm text-red-700">{syncState.error_message}</p>
            </div>
          </div>
        )}

        {/* Sync Progress Alert */}
        {isSyncing && syncState && (
          <div className="flex items-start gap-3 rounded-xl border border-sercha-indigo/20 bg-sercha-indigo-soft p-4">
            <Loader2 size={20} className="mt-0.5 flex-shrink-0 animate-spin text-sercha-indigo" />
            <div>
              <p className="font-medium text-sercha-indigo">Sync in Progress</p>
              <p className="mt-1 text-sm text-sercha-indigo/80">
                {syncState.documents_synced > 0
                  ? `${syncState.documents_synced} documents synced so far...`
                  : "Starting sync..."}
              </p>
            </div>
          </div>
        )}

        {/* Documents Section */}
        <div className="rounded-2xl border border-sercha-silverline bg-white">
          <div className="flex items-center justify-between border-b border-sercha-silverline px-6 py-4">
            <h3 className="font-semibold text-sercha-ink-slate">
              Documents ({totalDocs.toLocaleString()})
            </h3>
            {totalDocs > 0 && (
              <p className="text-sm text-sercha-fog-grey">
                Showing {startItem}-{endItem} of {totalDocs}
              </p>
            )}
          </div>

          {loadingDocs ? (
            <div className="flex items-center justify-center py-12">
              <Loader2 className="h-6 w-6 animate-spin text-sercha-indigo" />
            </div>
          ) : documents.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-12">
              <FileText className="mb-3 h-10 w-10 text-sercha-silverline" />
              <p className="text-sercha-fog-grey">No documents indexed yet</p>
              {source.enabled && (
                <button
                  onClick={handleSync}
                  disabled={isSyncing}
                  className="mt-4 text-sm text-sercha-indigo hover:underline"
                >
                  Trigger a sync to index documents
                </button>
              )}
            </div>
          ) : (
            <>
              <div className="divide-y divide-sercha-mist">
                {documents.map((doc) => (
                  <div
                    key={doc.id}
                    className="group flex items-center justify-between px-6 py-3 hover:bg-sercha-snow"
                  >
                    <div className="min-w-0 flex-1">
                      <button
                        onClick={() => handleOpenDocument(doc.id)}
                        className="flex items-center gap-2 text-left text-sm font-medium text-sercha-ink-slate hover:text-sercha-indigo"
                      >
                        <FileText size={16} className="flex-shrink-0 text-sercha-fog-grey" />
                        <span className="truncate">{doc.title}</span>
                        {openingDoc === doc.id ? (
                          <Loader2 size={14} className="animate-spin text-sercha-fog-grey" />
                        ) : (
                          <ExternalLink size={14} className="opacity-0 group-hover:opacity-50" />
                        )}
                      </button>
                      {doc.url && (
                        <p className="ml-6 mt-0.5 truncate text-xs text-sercha-fog-grey font-mono">
                          {doc.url}
                        </p>
                      )}
                    </div>
                    <div className="flex items-center gap-4 text-xs text-sercha-fog-grey">
                      <span className="rounded bg-sercha-mist px-2 py-0.5">
                        {doc.content_type || "unknown"}
                      </span>
                      <span>{formatRelativeTime(doc.updated_at)}</span>
                    </div>
                  </div>
                ))}
              </div>

              {/* Pagination */}
              {totalPages > 1 && (
                <div className="flex items-center justify-between border-t border-sercha-silverline px-6 py-4">
                  <button
                    onClick={() => setPage((p) => Math.max(0, p - 1))}
                    disabled={page === 0}
                    className="inline-flex items-center gap-1 rounded-lg px-3 py-1.5 text-sm text-sercha-fog-grey hover:bg-sercha-mist disabled:opacity-50"
                  >
                    <ChevronLeft size={16} />
                    Previous
                  </button>
                  <span className="text-sm text-sercha-fog-grey">
                    Page {page + 1} of {totalPages}
                  </span>
                  <button
                    onClick={() => setPage((p) => Math.min(totalPages - 1, p + 1))}
                    disabled={page >= totalPages - 1}
                    className="inline-flex items-center gap-1 rounded-lg px-3 py-1.5 text-sm text-sercha-fog-grey hover:bg-sercha-mist disabled:opacity-50"
                  >
                    Next
                    <ChevronRight size={16} />
                  </button>
                </div>
              )}
            </>
          )}
        </div>
      </div>
    </AdminLayout>
  );
}

export default function SourceDetailPage() {
  return (
    <Suspense
      fallback={
        <AdminLayout title="Source" description="Loading...">
          <div className="flex items-center justify-center py-12">
            <Loader2 className="h-8 w-8 animate-spin text-sercha-indigo" />
          </div>
        </AdminLayout>
      }
    >
      <SourceDetailContent />
    </Suspense>
  );
}
