"use client";

import { useState, useEffect, useCallback, Suspense } from "react";
import { createPortal } from "react-dom";
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
  User,
  Key,
  Calendar,
  X,
  Search,
} from "lucide-react";
import { AdminLayout } from "@/components/layout";
import {
  getSource,
  getSourceSyncState,
  triggerSync,
  deleteSource,
  enableSource,
  disableSource,
  getSourceDocuments,
  getDocumentURL,
  getConnection,
  getConnectionContainers,
  updateSourceContainers,
  Source,
  ConnectionSummary,
  SyncState,
  Document,
  Container,
} from "@/lib/api";
import { getProviderIcon, getProviderName } from "@/lib/providers";

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

function formatDate(dateStr?: string): string {
  if (!dateStr) return "Unknown";
  const date = new Date(dateStr);
  return date.toLocaleDateString("en-US", {
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

const PAGE_SIZE = 20;

function SourceDetailContent() {
  const searchParams = useSearchParams();
  const router = useRouter();
  const sourceId = searchParams.get("id");

  // State
  const [source, setSource] = useState<Source | null>(null);
  const [connection, setConnection] = useState<ConnectionSummary | null>(null);
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
  const [updatingContainers, setUpdatingContainers] = useState(false);

  // Container picker modal state
  const [showContainerPicker, setShowContainerPicker] = useState(false);
  const [availableContainers, setAvailableContainers] = useState<Container[]>([]);
  const [selectedContainerIds, setSelectedContainerIds] = useState<Set<string>>(new Set());
  const [loadingContainers, setLoadingContainers] = useState(false);
  const [containerSearchQuery, setContainerSearchQuery] = useState("");

  // Fetch source and connection data
  const fetchSource = useCallback(async () => {
    if (!sourceId) return;
    try {
      const sourceData = await getSource(sourceId);
      setSource(sourceData);

      // Fetch connection details if available
      if (sourceData.connection_id) {
        try {
          const connectionData = await getConnection(sourceData.connection_id);
          setConnection(connectionData);
        } catch (err) {
          console.error("Failed to fetch connection:", err);
        }
      }
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

  const handleOpenContainerPicker = async () => {
    if (!source || !connection) return;

    setLoadingContainers(true);
    setShowContainerPicker(true);

    try {
      const result = await getConnectionContainers(connection.id);
      setAvailableContainers(result.containers);
      // Pre-select currently synced containers
      if (source.containers && source.containers.length > 0) {
        setSelectedContainerIds(new Set(source.containers.map(c => c.id)));
      } else {
        setSelectedContainerIds(new Set());
      }
    } catch (err) {
      console.error("Failed to load containers:", err);
      setShowContainerPicker(false);
    } finally {
      setLoadingContainers(false);
    }
  };

  const handleSaveContainerSelection = async () => {
    if (!sourceId) return;

    setUpdatingContainers(true);
    try {
      // If no containers selected, this means "sync all" mode (empty array)
      // If containers are selected, pass the selected Container objects
      const selectedContainers = selectedContainerIds.size === 0
        ? []
        : availableContainers.filter(c => selectedContainerIds.has(c.id));
      await updateSourceContainers(sourceId, { containers: selectedContainers });
      await fetchSource();
      setShowContainerPicker(false);
    } catch (err) {
      console.error("Failed to update containers:", err);
    } finally {
      setUpdatingContainers(false);
    }
  };

  const toggleContainerSelection = (containerId: string) => {
    setSelectedContainerIds(prev => {
      const newSet = new Set(prev);
      if (newSet.has(containerId)) {
        newSet.delete(containerId);
      } else {
        newSet.add(containerId);
      }
      return newSet;
    });
  };

  const filteredAvailableContainers = availableContainers.filter(c =>
    c.name.toLowerCase().includes(containerSearchQuery.toLowerCase())
  );

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

  // Derive status from syncState since getSource() doesn't include it
  const sourceStatus = syncState?.status === "error" ? "error" : syncState?.status === "syncing" ? "syncing" : "healthy";
  const isSyncing = syncState?.status === "syncing" || syncing;
  const totalPages = Math.ceil(totalDocs / PAGE_SIZE);
  const startItem = page * PAGE_SIZE + 1;
  const endItem = Math.min((page + 1) * PAGE_SIZE, totalDocs);

  return (
    <AdminLayout
      title={source.name}
      description={`Manage ${getProviderName(source.provider_type)} source`}
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
                {sourceStatus === "error" && (
                  <span className="flex items-center gap-1.5 rounded-full bg-red-100 px-2.5 py-1 text-xs font-medium text-red-600">
                    <AlertCircle size={12} />
                    Error
                  </span>
                )}
              </div>
              <p className="mt-1 text-sm text-sercha-fog-grey">
                {getProviderName(source.provider_type)} •{" "}
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
                  {totalDocs.toLocaleString()}
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
                  {formatRelativeTime(syncState?.last_sync_time)}
                </p>
                <p className="text-sm text-sercha-fog-grey">Last Synced</p>
              </div>
            </div>
          </div>
          <div className="rounded-xl border border-sercha-silverline bg-white p-4">
            <div className="flex items-center gap-3">
              <div className={`flex h-10 w-10 items-center justify-center rounded-lg ${
                sourceStatus === "error" ? "bg-red-100" : "bg-emerald-100"
              }`}>
                {sourceStatus === "error" ? (
                  <AlertCircle size={20} className="text-red-600" />
                ) : (
                  <CheckCircle size={20} className="text-emerald-600" />
                )}
              </div>
              <div>
                <p className={`text-2xl font-bold ${
                  sourceStatus === "error" ? "text-red-600" : "text-emerald-600"
                }`}>
                  {sourceStatus === "error" ? "Error" : "Healthy"}
                </p>
                <p className="text-sm text-sercha-fog-grey">Status</p>
              </div>
            </div>
          </div>
        </div>

        {/* Connection Info */}
        {connection && (
          <div className="rounded-2xl border border-sercha-silverline bg-white p-6">
            <h3 className="mb-4 font-semibold text-sercha-ink-slate">Connection Details</h3>
            <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
              <div className="flex items-start gap-3">
                <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-sercha-snow">
                  <User size={16} className="text-sercha-fog-grey" />
                </div>
                <div>
                  <p className="text-xs text-sercha-fog-grey">Account</p>
                  <p className="text-sm font-medium text-sercha-ink-slate">{connection.account_id}</p>
                </div>
              </div>
              <div className="flex items-start gap-3">
                <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-sercha-snow">
                  <Key size={16} className="text-sercha-fog-grey" />
                </div>
                <div>
                  <p className="text-xs text-sercha-fog-grey">Auth Method</p>
                  <p className="text-sm font-medium text-sercha-ink-slate capitalize">{connection.auth_method}</p>
                </div>
              </div>
              <div className="flex items-start gap-3">
                <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-sercha-snow">
                  <Calendar size={16} className="text-sercha-fog-grey" />
                </div>
                <div>
                  <p className="text-xs text-sercha-fog-grey">Connected</p>
                  <p className="text-sm font-medium text-sercha-ink-slate">{formatDate(connection.created_at)}</p>
                </div>
              </div>
              {connection.oauth_expiry && (
                <div className="flex items-start gap-3">
                  <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-sercha-snow">
                    <Clock size={16} className="text-sercha-fog-grey" />
                  </div>
                  <div>
                    <p className="text-xs text-sercha-fog-grey">Token Expires</p>
                    <p className="text-sm font-medium text-sercha-ink-slate">{formatDate(connection.oauth_expiry)}</p>
                  </div>
                </div>
              )}
            </div>
            <div className="mt-4 pt-4 border-t border-sercha-mist">
              <p className="text-xs text-sercha-fog-grey">
                Connection ID: <code className="rounded bg-sercha-snow px-1.5 py-0.5 font-mono text-xs">{connection.id}</code>
              </p>
            </div>
          </div>
        )}

        {/* Container Sync Mode */}
        <div className="rounded-2xl border border-sercha-silverline bg-white p-6">
          <div className="flex items-start justify-between">
            <div className="flex-1">
              <h3 className="mb-2 font-semibold text-sercha-ink-slate">Container Sync Mode</h3>
              {!source?.containers || source.containers.length === 0 ? (
                <>
                  <div className="flex items-center gap-2 mb-2">
                    <span className="rounded-full bg-sercha-indigo-soft px-2.5 py-1 text-xs font-medium text-sercha-indigo">
                      Sync All Containers
                    </span>
                  </div>
                  <p className="text-sm text-sercha-fog-grey">
                    Automatically syncing all accessible containers including future items (auto-discover mode).
                  </p>
                </>
              ) : (
                <>
                  <div className="flex items-center gap-2 mb-2">
                    <span className="rounded-full bg-sercha-snow px-2.5 py-1 text-xs font-medium text-sercha-ink-slate">
                      Specific Containers
                    </span>
                  </div>
                  <p className="text-sm text-sercha-fog-grey">
                    Syncing {source.containers.length} specific {source.containers.length === 1 ? "container" : "containers"}.
                  </p>
                </>
              )}
            </div>
            <button
              onClick={handleOpenContainerPicker}
              disabled={updatingContainers}
              className="inline-flex items-center gap-2 rounded-lg border border-sercha-silverline bg-white px-4 py-2 text-sm font-medium text-sercha-ink-slate transition-all hover:bg-sercha-mist disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {updatingContainers ? (
                <>
                  <Loader2 size={16} className="animate-spin" />
                  Updating...
                </>
              ) : (
                "Manage Containers"
              )}
            </button>
          </div>
        </div>

        {/* Container Picker Modal - rendered via portal to escape stacking context */}
        {showContainerPicker && typeof document !== 'undefined' && createPortal(
          <>
            {/* Backdrop */}
            <div
              className="fixed inset-0 z-[9998] bg-black/50"
              onClick={() => setShowContainerPicker(false)}
            />
            {/* Modal */}
            <div className="fixed top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 z-[9999] w-full max-w-lg rounded-2xl bg-white p-6 shadow-xl mx-4 max-h-[80vh] flex flex-col">
              <div className="flex items-center justify-between mb-4">
                <h3 className="text-lg font-semibold text-sercha-ink-slate">Manage Containers</h3>
                <button
                  onClick={() => setShowContainerPicker(false)}
                  className="rounded-lg p-1 text-sercha-fog-grey hover:bg-sercha-mist"
                >
                  <X size={20} />
                </button>
              </div>

              <p className="text-sm text-sercha-fog-grey mb-4">
                Choose how to sync containers for this source.
              </p>

              {/* Sync Mode Selection */}
              <div className="mb-4 space-y-3">
                {/* Sync All Option */}
                <button
                  type="button"
                  onClick={() => {
                    // Clear selection to indicate sync-all mode
                    setSelectedContainerIds(new Set());
                  }}
                  disabled={updatingContainers}
                  className={`w-full rounded-lg border-2 p-3 text-left transition-colors ${
                    selectedContainerIds.size === 0
                      ? "border-sercha-indigo bg-sercha-indigo/5"
                      : "border-sercha-silverline bg-white hover:border-sercha-fog-grey"
                  }`}
                >
                  <div className="flex items-center gap-3">
                    <div className={`h-4 w-4 shrink-0 rounded-full border-2 flex items-center justify-center ${
                      selectedContainerIds.size === 0 ? "border-sercha-indigo" : "border-sercha-fog-grey"
                    }`}>
                      {selectedContainerIds.size === 0 && <div className="h-2 w-2 rounded-full bg-sercha-indigo" />}
                    </div>
                    <div>
                      <span className="font-medium text-sercha-ink-slate text-sm">Sync all (auto-discover)</span>
                      <p className="text-xs text-sercha-fog-grey">Index all content including future items</p>
                    </div>
                  </div>
                </button>

                {/* Select Specific Option */}
                <div
                  className={`rounded-lg border-2 p-3 transition-colors ${
                    selectedContainerIds.size > 0
                      ? "border-sercha-indigo bg-sercha-indigo/5"
                      : "border-sercha-silverline bg-white"
                  }`}
                >
                  <div className="flex items-center gap-3 mb-3">
                    <div className={`h-4 w-4 shrink-0 rounded-full border-2 flex items-center justify-center ${
                      selectedContainerIds.size > 0 ? "border-sercha-indigo" : "border-sercha-fog-grey"
                    }`}>
                      {selectedContainerIds.size > 0 && <div className="h-2 w-2 rounded-full bg-sercha-indigo" />}
                    </div>
                    <div>
                      <span className="font-medium text-sercha-ink-slate text-sm">Select specific containers</span>
                      <p className="text-xs text-sercha-fog-grey">Choose exactly which items to sync</p>
                    </div>
                  </div>

                  {/* Search */}
                  <div className="relative mb-3">
                    <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-sercha-fog-grey" />
                    <input
                      type="text"
                      value={containerSearchQuery}
                      onChange={(e) => setContainerSearchQuery(e.target.value)}
                      className="w-full rounded-lg border border-sercha-silverline bg-white py-2 pl-10 pr-4 text-sm placeholder:text-sercha-fog-grey focus:border-sercha-indigo focus:outline-none"
                      placeholder="Search containers..."
                    />
                  </div>

                  {/* Container List */}
                  <div className="max-h-48 overflow-y-auto space-y-1">
                    {loadingContainers ? (
                      <div className="flex items-center justify-center py-4">
                        <Loader2 className="h-5 w-5 animate-spin text-sercha-indigo" />
                      </div>
                    ) : filteredAvailableContainers.length === 0 ? (
                      <p className="text-center text-sm text-sercha-fog-grey py-4">
                        {containerSearchQuery ? "No matching containers" : "No containers available"}
                      </p>
                    ) : (
                      filteredAvailableContainers.map((container) => (
                        <label
                          key={container.id}
                          className="flex items-center gap-3 rounded-lg p-2 hover:bg-sercha-mist cursor-pointer"
                        >
                          <input
                            type="checkbox"
                            checked={selectedContainerIds.has(container.id)}
                            onChange={() => toggleContainerSelection(container.id)}
                            className="h-4 w-4 rounded border-sercha-silverline text-sercha-indigo focus:ring-sercha-indigo/20"
                          />
                          <span className="text-sm text-sercha-ink-slate truncate">{container.name}</span>
                        </label>
                      ))
                    )}
                  </div>

                  <p className="mt-2 text-xs text-sercha-fog-grey">
                    {selectedContainerIds.size} container{selectedContainerIds.size !== 1 ? "s" : ""} selected
                  </p>
                </div>
              </div>

              {/* Actions */}
              <div className="flex gap-3 mt-auto pt-4 border-t border-sercha-mist">
                <button
                  onClick={() => setShowContainerPicker(false)}
                  className="flex-1 rounded-lg border border-sercha-silverline bg-white px-4 py-2 text-sm font-medium text-sercha-fog-grey hover:bg-sercha-mist"
                >
                  Cancel
                </button>
                <button
                  onClick={handleSaveContainerSelection}
                  disabled={updatingContainers}
                  className="flex-1 rounded-lg bg-sercha-indigo px-4 py-2 text-sm font-medium text-white hover:bg-sercha-indigo/90 disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  {updatingContainers ? (
                    <Loader2 className="h-4 w-4 animate-spin mx-auto" />
                  ) : selectedContainerIds.size === 0 ? (
                    "Save (Sync All)"
                  ) : (
                    `Save (${selectedContainerIds.size} selected)`
                  )}
                </button>
              </div>
            </div>
          </>,
          document.body
        )}

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
                        {doc.mime_type || "unknown"}
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
