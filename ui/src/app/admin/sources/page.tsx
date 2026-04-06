"use client";

import { useState, useEffect } from "react";
import Image from "next/image";
import Link from "next/link";
import { useRouter } from "next/navigation";
import {
  Plus,
  Loader2,
  RefreshCw,
  AlertCircle,
  ChevronRight,
  Lock,
  ExternalLink,
} from "lucide-react";
import { AdminLayout } from "@/components/layout";
import {
  listSources,
  listConnections,
  getCapabilities,
  listProviders,
  startOAuth,
  triggerSync,
  SourceSummary,
  ConnectionSummary,
  ProviderListItem,
  ApiError,
} from "@/lib/api";
import { getProviderIcon, PROVIDER_NAMES } from "@/lib/providers";

// Combined view: sources grouped by their connection + available/unavailable providers
interface SourceWithConnection extends SourceSummary {
  connection?: ConnectionSummary;
}

function formatLastSync(dateStr?: string): string {
  if (!dateStr) return "Never synced";
  const date = new Date(dateStr);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffMins = Math.floor(diffMs / 60000);
  const diffHours = Math.floor(diffMins / 60);
  const diffDays = Math.floor(diffHours / 24);

  if (diffMins < 1) return "Just now";
  if (diffMins < 60) return `${diffMins}m ago`;
  if (diffHours < 24) return `${diffHours}h ago`;
  return `${diffDays}d ago`;
}

function getStatusColor(status?: string): string {
  switch (status) {
    case "syncing":
      return "text-sercha-indigo";
    case "error":
      return "text-red-500";
    default:
      return "text-emerald-600";
  }
}

// Connected Source Card
function ConnectedSourceCard({
  source,
  onSync,
  syncing,
  onClick,
}: {
  source: SourceWithConnection;
  onSync: () => void;
  syncing: boolean;
  onClick: () => void;
}) {
  return (
    <div
      onClick={onClick}
      className="group flex cursor-pointer items-center gap-4 rounded-xl border border-sercha-silverline bg-white p-4 transition-all hover:border-sercha-indigo/30 hover:shadow-md"
    >
      {/* Provider Icon */}
      <div className="flex h-12 w-12 flex-shrink-0 items-center justify-center rounded-lg bg-sercha-snow">
        <Image
          src={getProviderIcon(source.provider_type)}
          alt={source.provider_type}
          width={28}
          height={28}
          className="h-7 w-7 object-contain"
        />
      </div>

      {/* Info */}
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <p className="text-sm font-semibold text-sercha-ink-slate group-hover:text-sercha-indigo">
            {source.name}
          </p>
          {source.status === "syncing" && (
            <RefreshCw size={14} className="animate-spin text-sercha-indigo" />
          )}
          {source.status === "error" && (
            <AlertCircle size={14} className="text-red-500" />
          )}
        </div>
        <p className="text-xs text-sercha-fog-grey">
          {source.document_count.toLocaleString()} documents •{" "}
          {source.connection?.account_id || PROVIDER_NAMES[source.provider_type] || source.provider_type}
        </p>
      </div>

      {/* Status & Sync Info */}
      <div className="flex items-center gap-3">
        <div className="text-right">
          <p className="text-xs text-sercha-fog-grey">{formatLastSync(source.last_synced)}</p>
          <span className={`text-xs font-medium ${getStatusColor(source.status)}`}>
            {source.status === "syncing" ? "Syncing" : source.status === "error" ? "Error" : "Idle"}
          </span>
        </div>

        {/* Sync Button */}
        <button
          onClick={(e) => {
            e.stopPropagation();
            onSync();
          }}
          disabled={syncing || source.status === "syncing"}
          className="rounded-lg p-2 text-sercha-fog-grey hover:bg-sercha-mist hover:text-sercha-indigo disabled:opacity-50"
          title="Sync now"
        >
          <RefreshCw size={16} className={syncing ? "animate-spin" : ""} />
        </button>

        {/* Arrow */}
        <ChevronRight size={18} className="text-sercha-silverline group-hover:text-sercha-indigo" />
      </div>
    </div>
  );
}

// Available Provider Card (configured, can connect)
function AvailableProviderCard({
  provider,
  onConnect,
  connecting,
}: {
  provider: ProviderListItem;
  onConnect: () => void;
  connecting: boolean;
}) {
  return (
    <div className="flex items-center gap-4 rounded-xl border border-dashed border-sercha-silverline bg-sercha-snow/50 p-4 transition-all hover:border-sercha-indigo/50 hover:bg-white">
      {/* Provider Icon */}
      <div className="flex h-12 w-12 flex-shrink-0 items-center justify-center rounded-lg bg-white border border-sercha-silverline">
        <Image
          src={getProviderIcon(provider.type)}
          alt={provider.name}
          width={28}
          height={28}
          className="h-7 w-7 object-contain"
        />
      </div>

      {/* Info */}
      <div className="min-w-0 flex-1">
        <p className="text-sm font-semibold text-sercha-ink-slate">{provider.name}</p>
        <p className="text-xs text-sercha-fog-grey">Ready to connect</p>
      </div>

      {/* Connect Button */}
      <button
        onClick={onConnect}
        disabled={connecting}
        className="inline-flex items-center gap-1.5 rounded-lg bg-sercha-indigo px-3 py-2 text-sm font-medium text-white transition-all hover:bg-sercha-indigo/90 disabled:opacity-50"
      >
        {connecting ? (
          <Loader2 size={14} className="animate-spin" />
        ) : (
          <ExternalLink size={14} />
        )}
        Connect
      </button>
    </div>
  );
}

// Unavailable Provider Card (not configured on server)
function UnavailableProviderCard({ provider }: { provider: ProviderListItem }) {
  return (
    <div className="flex items-center gap-4 rounded-xl border border-sercha-silverline bg-sercha-mist/50 p-4 opacity-60">
      {/* Provider Icon */}
      <div className="flex h-12 w-12 flex-shrink-0 items-center justify-center rounded-lg bg-sercha-snow border border-sercha-silverline">
        <Image
          src={getProviderIcon(provider.type)}
          alt={provider.name}
          width={28}
          height={28}
          className="h-7 w-7 object-contain grayscale"
        />
      </div>

      {/* Info */}
      <div className="min-w-0 flex-1">
        <p className="text-sm font-medium text-sercha-fog-grey">{provider.name}</p>
        <p className="text-xs text-sercha-fog-grey">Requires server configuration</p>
      </div>

      {/* Lock Icon */}
      <div className="flex items-center gap-1 text-sercha-fog-grey">
        <Lock size={14} />
        <span className="text-xs">Not configured</span>
      </div>
    </div>
  );
}

export default function SourcesPage() {
  const router = useRouter();

  // Data state
  const [sources, setSources] = useState<SourceWithConnection[]>([]);
  const [availableProviders, setAvailableProviders] = useState<ProviderListItem[]>([]);
  const [unavailableProviders, setUnavailableProviders] = useState<ProviderListItem[]>([]);

  // UI state
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [syncingIds, setSyncingIds] = useState<Set<string>>(new Set());
  const [connectingProvider, setConnectingProvider] = useState<string | null>(null);

  // Load all data
  const loadData = async () => {
    try {
      setLoading(true);
      setError(null);

      // Fetch all data in parallel
      const [sourcesData, connectionsData, capabilities, allProviders] = await Promise.all([
        listSources(),
        listConnections(),
        getCapabilities(),
        listProviders(),
      ]);

      // Map connections to sources (handle null/undefined)
      const connectionMap = new Map((connectionsData || []).map((c) => [c.id, c]));
      const sourcesWithConnections: SourceWithConnection[] = (sourcesData || []).map((s) => ({
        ...s,
        connection: s.connection_id ? connectionMap.get(s.connection_id) : undefined,
      }));
      setSources(sourcesWithConnections);

      // Determine which providers are available vs unavailable
      // A provider is "available" if it's in capabilities.oauth_providers AND no source exists for it
      const configuredProviders = new Set(capabilities.oauth_providers);
      const providersWithSources = new Set((sourcesData || []).map((s) => s.provider_type));

      const available: ProviderListItem[] = [];
      const unavailable: ProviderListItem[] = [];

      for (const provider of allProviders) {
        if (providersWithSources.has(provider.type)) {
          // Already has a source, skip (shown in connected section)
          continue;
        }
        if (configuredProviders.has(provider.type)) {
          available.push(provider);
        } else {
          unavailable.push(provider);
        }
      }

      setAvailableProviders(available);
      setUnavailableProviders(unavailable);
    } catch (err) {
      setError("Failed to load data");
      console.error("Failed to fetch data:", err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadData();
  }, []);

  // Handlers
  const handleSync = async (id: string) => {
    try {
      setSyncingIds((prev) => new Set(prev).add(id));
      await triggerSync(id);
      setTimeout(loadData, 1000);
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

  const handleConnect = async (providerType: string) => {
    try {
      setConnectingProvider(providerType);
      const result = await startOAuth(providerType, undefined, "admin-sources");
      window.location.href = result.authorization_url;
    } catch (err) {
      console.error("Failed to start OAuth:", err);
      if (err instanceof ApiError) {
        setError(err.message);
      }
      setConnectingProvider(null);
    }
  };

  // Loading state
  if (loading) {
    return (
      <AdminLayout title="Sources" description="Manage your data sources">
        <div className="flex items-center justify-center py-12">
          <Loader2 className="h-8 w-8 animate-spin text-sercha-indigo" />
        </div>
      </AdminLayout>
    );
  }

  // Error state
  if (error) {
    return (
      <AdminLayout title="Sources" description="Manage your data sources">
        <div className="flex flex-col items-center justify-center rounded-2xl border border-red-200 bg-red-50 py-12">
          <AlertCircle className="mb-4 h-8 w-8 text-red-500" />
          <p className="text-red-700">{error}</p>
          <button onClick={loadData} className="mt-4 text-sm text-sercha-indigo hover:underline">
            Try again
          </button>
        </div>
      </AdminLayout>
    );
  }

  const hasConnectedSources = sources.length > 0;
  const hasAvailableProviders = availableProviders.length > 0;
  const hasUnavailableProviders = unavailableProviders.length > 0;

  return (
    <AdminLayout title="Sources" description="Manage your data sources and connections">
      <div className="space-y-8">
        {/* Connected Sources Section */}
        {hasConnectedSources && (
          <section>
            <div className="mb-4 flex items-center justify-between">
              <div>
                <h2 className="text-lg font-semibold text-sercha-ink-slate">Connected Sources</h2>
                <p className="text-sm text-sercha-fog-grey">
                  {sources.length} source{sources.length !== 1 ? "s" : ""} syncing data
                </p>
              </div>
              {hasAvailableProviders && (
                <Link
                  href="/admin/sources/new"
                  className="inline-flex items-center gap-2 rounded-full bg-sercha-indigo px-4 py-2 text-sm font-semibold text-white transition-all hover:bg-sercha-indigo/90"
                >
                  <Plus size={16} />
                  Add Source
                </Link>
              )}
            </div>
            <div className="space-y-3">
              {sources.map((source) => (
                <ConnectedSourceCard
                  key={source.id}
                  source={source}
                  onSync={() => handleSync(source.id)}
                  syncing={syncingIds.has(source.id)}
                  onClick={() => router.push(`/admin/sources/view?id=${source.id}`)}
                />
              ))}
            </div>
          </section>
        )}

        {/* Available Providers Section */}
        {hasAvailableProviders && (
          <section>
            <div className="mb-4">
              <h2 className="text-lg font-semibold text-sercha-ink-slate">
                {hasConnectedSources ? "Add More Sources" : "Connect a Data Source"}
              </h2>
              <p className="text-sm text-sercha-fog-grey">
                {hasConnectedSources
                  ? "Connect additional providers to index more data"
                  : "Choose a provider to start indexing your data"}
              </p>
            </div>
            <div className="space-y-3">
              {availableProviders.map((provider) => (
                <AvailableProviderCard
                  key={provider.type}
                  provider={provider}
                  onConnect={() => handleConnect(provider.type)}
                  connecting={connectingProvider === provider.type}
                />
              ))}
            </div>
          </section>
        )}

        {/* Empty State - No sources and no available providers */}
        {!hasConnectedSources && !hasAvailableProviders && (
          <div className="flex flex-col items-center justify-center rounded-2xl border-2 border-dashed border-sercha-silverline bg-white py-16">
            <div className="mb-4 rounded-full bg-sercha-indigo-soft p-4">
              <Lock className="h-8 w-8 text-sercha-indigo" />
            </div>
            <h3 className="mb-2 text-lg font-semibold text-sercha-ink-slate">
              No providers configured
            </h3>
            <p className="mb-2 max-w-md text-center text-sercha-fog-grey">
              OAuth providers need to be configured on the server via environment variables.
            </p>
            <p className="text-sm text-sercha-fog-grey">
              Set <code className="rounded bg-sercha-mist px-1.5 py-0.5 font-mono text-xs">GITHUB_CLIENT_ID</code> and{" "}
              <code className="rounded bg-sercha-mist px-1.5 py-0.5 font-mono text-xs">GITHUB_CLIENT_SECRET</code> to enable GitHub.
            </p>
          </div>
        )}

        {/* Unavailable Providers Section */}
        {hasUnavailableProviders && (
          <section>
            <div className="mb-4">
              <h2 className="text-base font-medium text-sercha-fog-grey">Not Configured</h2>
              <p className="text-sm text-sercha-fog-grey">
                These providers require server-side configuration
              </p>
            </div>
            <div className="space-y-2">
              {unavailableProviders.map((provider) => (
                <UnavailableProviderCard key={provider.type} provider={provider} />
              ))}
            </div>
          </section>
        )}
      </div>
    </AdminLayout>
  );
}
