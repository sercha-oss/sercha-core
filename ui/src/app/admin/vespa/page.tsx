"use client";

import { useState, useEffect, useCallback } from "react";
import { AdminLayout } from "@/components/layout";
import {
  VespaOverviewCards,
  VespaNodesTable,
  VespaQueryStats,
  VespaFeedStats,
} from "@/components/vespa";
import {
  getVespaStatus,
  getVespaMetrics,
  disconnectVespa,
  connectVespa,
  VespaStatus,
  VespaMetrics,
} from "@/lib/api";
import { RefreshCw, AlertCircle, Loader2, Unplug, ArrowUpCircle, AlertTriangle } from "lucide-react";

export default function VespaPage() {
  const [status, setStatus] = useState<VespaStatus | null>(null);
  const [metrics, setMetrics] = useState<VespaMetrics | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [disconnecting, setDisconnecting] = useState(false);
  const [showDisconnectConfirm, setShowDisconnectConfirm] = useState(false);
  const [upgrading, setUpgrading] = useState(false);

  const fetchData = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);
      const [statusData, metricsData] = await Promise.all([
        getVespaStatus(),
        getVespaMetrics(),
      ]);
      setStatus(statusData);
      setMetrics(metricsData);
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Failed to load Vespa data"
      );
    } finally {
      setLoading(false);
    }
  }, []);

  const handleDisconnect = async () => {
    if (!showDisconnectConfirm) {
      setShowDisconnectConfirm(true);
      return;
    }

    setDisconnecting(true);
    setError(null);
    try {
      await disconnectVespa();
      setShowDisconnectConfirm(false);
      // Redirect to setup or clear the state
      setStatus(null);
      setMetrics(null);
      // Refresh to show disconnected state
      await fetchData();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to disconnect Vespa");
    } finally {
      setDisconnecting(false);
    }
  };

  const handleUpgradeSchema = async () => {
    if (!status?.endpoint) {
      setError("No Vespa endpoint configured");
      return;
    }
    setUpgrading(true);
    setError(null);
    try {
      // Re-connect with current settings - this will upgrade the schema if embedding is now available
      const updatedStatus = await connectVespa({
        endpoint: status.endpoint,
        dev_mode: status.dev_mode ?? true,
      });
      setStatus(updatedStatus);
      // Refresh metrics too
      await fetchData();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to upgrade schema");
    } finally {
      setUpgrading(false);
    }
  };

  useEffect(() => {
    fetchData();
    // Auto-refresh every 30 seconds
    const interval = setInterval(fetchData, 30000);
    return () => clearInterval(interval);
  }, [fetchData]);

  return (
    <AdminLayout
      title="Vespa Cluster"
      description="Search engine metrics and status"
    >
      {/* Header with refresh and disconnect */}
      <div className="mb-6 flex items-center justify-between">
        <div>
          {metrics && (
            <p className="text-sm text-sercha-fog-grey">
              Last updated:{" "}
              {new Date(metrics.timestamp * 1000).toLocaleTimeString()}
            </p>
          )}
        </div>
        <div className="flex gap-2">
          <button
            onClick={fetchData}
            disabled={loading}
            className="flex items-center gap-2 rounded-lg border border-sercha-silverline px-3 py-2 text-sm text-sercha-fog-grey hover:bg-sercha-mist disabled:opacity-50"
          >
            <RefreshCw className={`h-4 w-4 ${loading ? "animate-spin" : ""}`} />
            Refresh
          </button>
          {status?.connected && (
            <button
              onClick={handleDisconnect}
              disabled={disconnecting}
              className={`flex items-center gap-2 rounded-lg border px-3 py-2 text-sm ${
                showDisconnectConfirm
                  ? "border-red-300 bg-red-50 text-red-600 hover:bg-red-100"
                  : "border-sercha-silverline text-sercha-fog-grey hover:bg-sercha-mist hover:text-red-600"
              } disabled:opacity-50`}
            >
              {disconnecting ? (
                <Loader2 className="h-4 w-4 animate-spin" />
              ) : (
                <Unplug className="h-4 w-4" />
              )}
              {showDisconnectConfirm ? "Confirm Disconnect" : "Disconnect"}
            </button>
          )}
        </div>
      </div>

      {/* Disconnect Confirmation Banner */}
      {showDisconnectConfirm && (
        <div className="mb-6 flex items-center justify-between rounded-lg bg-red-50 p-4">
          <div className="text-sm text-red-700">
            <strong>Warning:</strong> Disconnecting will remove the Vespa configuration.
            Documents in Vespa will become orphaned. This action cannot be undone.
          </div>
          <button
            onClick={() => setShowDisconnectConfirm(false)}
            className="text-sm text-red-600 underline hover:no-underline"
          >
            Cancel
          </button>
        </div>
      )}

      {/* Schema Upgrade Available Banner */}
      {status?.connected && status?.can_upgrade && (
        <div className="mb-6 rounded-xl border-2 border-amber-300 bg-amber-50 p-4">
          <div className="flex items-start gap-3">
            <AlertTriangle className="mt-0.5 h-5 w-5 flex-shrink-0 text-amber-600" />
            <div className="flex-1">
              <h3 className="font-semibold text-amber-800">Schema Upgrade Available</h3>
              <p className="mt-1 text-sm text-amber-700">
                An embedding provider has been configured. Upgrade the Vespa schema to enable semantic search.
              </p>
              <p className="mt-2 text-sm text-amber-600">
                After upgrading, existing documents will need to be reindexed to generate embeddings.
              </p>
              <button
                onClick={handleUpgradeSchema}
                disabled={upgrading}
                className="mt-3 inline-flex items-center gap-2 rounded-lg bg-amber-600 px-4 py-2 text-sm font-medium text-white hover:bg-amber-700 disabled:opacity-50"
              >
                {upgrading ? (
                  <Loader2 className="h-4 w-4 animate-spin" />
                ) : (
                  <ArrowUpCircle className="h-4 w-4" />
                )}
                {upgrading ? "Upgrading Schema..." : "Upgrade Schema"}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Error state */}
      {error && (
        <div className="mb-6 flex items-center gap-2 rounded-lg bg-red-50 p-4 text-red-600">
          <AlertCircle className="h-5 w-5" />
          {error}
        </div>
      )}

      {/* Loading state */}
      {loading && !status && !metrics && (
        <div className="flex items-center justify-center py-12">
          <Loader2 className="h-8 w-8 animate-spin text-sercha-indigo" />
        </div>
      )}

      {/* Content */}
      {status && metrics && (
        <div className="space-y-6">
          {/* Overview Cards */}
          <VespaOverviewCards status={status} metrics={metrics} />

          {/* Query & Feed Stats */}
          <div className="grid gap-6 lg:grid-cols-2">
            <VespaQueryStats metrics={metrics} />
            <VespaFeedStats metrics={metrics} />
          </div>

          {/* Nodes Table */}
          <VespaNodesTable nodes={metrics.nodes} />
        </div>
      )}
    </AdminLayout>
  );
}
