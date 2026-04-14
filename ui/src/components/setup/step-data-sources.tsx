"use client";

import { useState, useEffect } from "react";
import Image from "next/image";
import {
  Loader2,
  X,
  CheckCircle2,
  XCircle,
  Search,
  ArrowLeft,
  ExternalLink,
  ChevronRight,
  Folder,
  Home,
} from "lucide-react";
import {
  listProviders,
  listSources,
  listConnections,
  getCapabilities,
  startOAuth,
  getConnection,
  getConnectionContainers,
  createSource,
  deleteSource,
  ApiError,
  type ProviderListItem,
  type SourceSummary,
  type ConnectionSummary,
  type Container,
} from "@/lib/api";
import { PROVIDER_ICONS } from "@/lib/providers";

interface StepDataSourcesProps {
  connectionId?: string;
  provider?: string;
  onComplete: () => void;
  onSkip: () => void;
}

type SubFlowView =
  | "selection"
  | "connection_picker"
  | "connecting"
  | "select"
  | "creating"
  | "success"
  | "error";

export function StepDataSources({
  connectionId,
  provider: providerFromUrl,
  onComplete,
  onSkip,
}: StepDataSourcesProps) {
  // View state
  const [view, setView] = useState<SubFlowView>("selection");
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Data state
  const [providers, setProviders] = useState<ProviderListItem[]>([]);
  const [connectedSources, setConnectedSources] = useState<SourceSummary[]>([]);
  const [pendingProvider, setPendingProvider] = useState<ProviderListItem | null>(null);

  // Connection & container state
  const [selectedConnection, setSelectedConnection] = useState<ConnectionSummary | null>(null);
  const [existingConnections, setExistingConnections] = useState<ConnectionSummary[]>([]);
  const [containers, setContainers] = useState<Container[]>([]);
  const [selectedContainers, setSelectedContainers] = useState<Set<string>>(new Set());
  const [searchQuery, setSearchQuery] = useState("");
  const [createdSourceName, setCreatedSourceName] = useState("");

  // Folder navigation state
  const [folderPath, setFolderPath] = useState<{id: string; name: string}[]>([]);
  const [isLoadingContainers, setIsLoadingContainers] = useState(false);

  // Load data on mount
  useEffect(() => {
    const loadInitialData = async () => {
      try {
        const [caps, providerList, sourceList] = await Promise.all([
          getCapabilities(),
          listProviders(),
          listSources().catch(() => []), // Gracefully handle if no sources yet
        ]);

        // Filter providers to only show those configured in environment
        const configuredProviders = providerList.filter((p) =>
          caps.oauth_providers.includes(p.type)
        );
        setProviders(configuredProviders);
        setConnectedSources(sourceList);

        // Handle OAuth callback return (connection_id and provider come from URL via /oauth/complete)
        if (connectionId) {
          await handleOAuthReturn(connectionId, providerFromUrl, configuredProviders);
        }
      } catch {
        // Don't show error on initial load
        setProviders([]);
        setConnectedSources([]);
      } finally {
        setIsLoading(false);
      }
    };
    loadInitialData();
  }, [connectionId, providerFromUrl]);

  const loadConnectedSources = async () => {
    try {
      const sources = await listSources();
      setConnectedSources(sources);
    } catch {
      // Ignore - keep existing state
    }
  };

  const loadContainers = async (instId: string, parentId?: string) => {
    setIsLoadingContainers(true);
    try {
      const result = await getConnectionContainers(instId, undefined, parentId);
      setContainers(result.containers);
      // Only auto-select all on initial load (root level)
      if (!parentId && folderPath.length === 0) {
        setSelectedContainers(new Set(result.containers.map((c) => c.id)));
      }
    } catch {
      setError("Failed to load repositories/containers");
      setView("error");
    } finally {
      setIsLoadingContainers(false);
    }
  };

  // Navigate into a folder
  const handleDrillDown = async (container: Container) => {
    if (!selectedConnection || !container.has_children) return;
    setFolderPath([...folderPath, { id: container.id, name: container.name }]);
    await loadContainers(selectedConnection.id, container.id);
  };

  // Navigate via breadcrumb
  const handleBreadcrumbClick = async (index: number) => {
    if (!selectedConnection) return;
    if (index === -1) {
      // Go to root
      setFolderPath([]);
      await loadContainers(selectedConnection.id);
    } else {
      const newPath = folderPath.slice(0, index + 1);
      setFolderPath(newPath);
      await loadContainers(selectedConnection.id, newPath[newPath.length - 1].id);
    }
  };

  // Provider selection
  const handleSelectProvider = async (provider: ProviderListItem) => {
    setPendingProvider(provider);
    setError(null);

    // Fetch existing connections for this provider
    try {
      const allConnections = await listConnections();
      const providerConnections = allConnections.filter(
        (i) => i.provider_type === provider.type
      );

      if (providerConnections.length > 0) {
        // Show picker to choose existing or connect new
        setExistingConnections(providerConnections);
        setView("connection_picker");
      } else {
        // No existing connections, go straight to OAuth
        await startOAuthFlow(provider);
      }
    } catch {
      // If fetch fails, just go to OAuth
      await startOAuthFlow(provider);
    }
  };

  // Select an existing connection
  const handleSelectExistingConnection = async (connection: ConnectionSummary) => {
    setSelectedConnection(connection);
    await loadContainers(connection.id);
    setView("select");
  };

  // Removed: credentials are now managed via environment variables

  // Start OAuth redirect
  const startOAuthFlow = async (provider: ProviderListItem) => {
    setView("connecting");
    setError(null);
    setSelectedConnection(null);  // Clear any previous connection
    setContainers([]);
    setSelectedContainers(new Set());

    try {
      // Pass "setup" as return_context so OAuth callback returns to FTUE
      const result = await startOAuth(provider.type, undefined, "setup");
      // OAuth provider redirects to API callback, which redirects to /oauth/complete,
      // which then redirects back here with connection_id and provider in URL params
      window.location.href = result.authorization_url;
    } catch (err) {
      if (err instanceof ApiError) {
        setError(err.message || "Failed to start OAuth");
      } else {
        setError("Failed to connect. Please try again.");
      }
      setView("error");
    }
  };

  // Handle OAuth callback return
  const handleOAuthReturn = async (connId: string, providerType?: string, providerList?: ProviderListItem[]) => {
    try {
      const connection = await getConnection(connId);
      setSelectedConnection(connection);

      // Set pending provider from URL params (passed from /oauth/complete)
      // Use providerList arg when called from useEffect (state may not be set yet)
      if (providerType) {
        const available = providerList || providers;
        const provider = available.find((p) => p.type === providerType);
        if (provider) setPendingProvider(provider);
      }

      await loadContainers(connId);
      setView("select");
    } catch (err) {
      if (err instanceof ApiError) {
        setError(err.message || "Failed to load connection");
      } else {
        setError("Failed to complete connection. Please try again.");
      }
      setView("error");
    }
  };

  // Container selection
  const handleSelectAll = () => {
    if (selectedContainers.size === containers.length) {
      setSelectedContainers(new Set());
    } else {
      setSelectedContainers(new Set(containers.map((c) => c.id)));
    }
  };

  const handleToggleContainer = (id: string) => {
    const newSelection = new Set(selectedContainers);
    if (newSelection.has(id)) {
      newSelection.delete(id);
    } else {
      newSelection.add(id);
    }
    setSelectedContainers(newSelection);
  };

  // Create source
  const handleCreateSource = async () => {
    if (!selectedConnection || selectedContainers.size === 0) return;

    setView("creating");
    setError(null);

    try {
      // Refresh sources to get accurate count (prevents duplicate name errors)
      const latestSources = await listSources().catch(() => connectedSources);

      // Generate unique source name
      const baseName = selectedConnection.name;
      const existingNames = new Set(latestSources.map((s) => s.name));

      let sourceName: string;
      if (!existingNames.has(baseName)) {
        // Base name is available
        sourceName = baseName;
      } else {
        // Find next available number
        let num = 2;
        while (existingNames.has(`${baseName} (${num})`)) {
          num++;
        }
        sourceName = `${baseName} (${num})`;
      }

      await createSource({
        name: sourceName,
        provider_type: selectedConnection.provider_type,
        connection_id: selectedConnection.id,
        containers: containers.filter((c) => selectedContainers.has(c.id)),
      });
      setCreatedSourceName(sourceName);
      await loadConnectedSources();  // Refresh to include new source
      setView("success");
    } catch (err) {
      if (err instanceof ApiError) {
        setError(err.message || "Failed to create source");
      } else {
        setError("Failed to create source. Please try again.");
      }
      setView("error");
    }
  };

  // Add another source
  const handleAddAnother = () => {
    setPendingProvider(null);
    setSelectedConnection(null);
    setContainers([]);
    setSelectedContainers(new Set());
    setSearchQuery("");
    setFolderPath([]);
    setError(null);
    setView("selection");
  };

  // Remove connected source
  const handleRemoveSource = async (sourceId: string) => {
    try {
      await deleteSource(sourceId);
      setConnectedSources((prev) => prev.filter((s) => s.id !== sourceId));
    } catch {
      // Ignore errors for now
    }
  };

  // Back navigation
  const handleBack = () => {
    if (view === "connection_picker") {
      setPendingProvider(null);
      setExistingConnections([]);
      setView("selection");
    } else if (view === "select") {
      // Go back to connection picker if we have existing connections
      if (existingConnections.length > 0) {
        setSelectedConnection(null);
        setContainers([]);
        setSelectedContainers(new Set());
        setView("connection_picker");
      } else {
        handleAddAnother();
      }
    } else if (view === "error") {
      handleAddAnother();
    }
  };

  const filteredContainers = containers.filter(
    (c) =>
      c.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
      c.description?.toLowerCase().includes(searchQuery.toLowerCase())
  );

  // Loading state
  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-12">
        <Loader2 className="h-8 w-8 animate-spin text-sercha-indigo" />
      </div>
    );
  }

  // Success view
  if (view === "success") {
    return (
      <div className="mx-auto max-w-md text-center">
        <div className="mb-6 flex justify-center">
          <div className="flex h-16 w-16 items-center justify-center rounded-full bg-emerald-100">
            <CheckCircle2 className="h-10 w-10 text-emerald-600" />
          </div>
        </div>
        <h1 className="text-2xl font-semibold text-sercha-ink-slate">
          Source Connected!
        </h1>
        <p className="mt-2 text-sm text-sercha-fog-grey">
          {createdSourceName} has been added. {selectedContainers.size}{" "}
          {selectedContainers.size === 1 ? "item" : "items"} will be synced.
        </p>
        <div className="mt-8 flex gap-3">
          <button
            onClick={handleAddAnother}
            className="flex-1 rounded-lg border border-sercha-silverline bg-white px-4 py-2.5 text-sm font-medium text-sercha-fog-grey transition-colors hover:bg-sercha-mist"
          >
            Add Another Source
          </button>
          <button
            onClick={onComplete}
            className="flex-1 rounded-lg bg-sercha-indigo px-4 py-2.5 text-sm font-medium text-white transition-colors hover:bg-sercha-indigo/90"
          >
            Continue
          </button>
        </div>
      </div>
    );
  }

  // Error view
  if (view === "error") {
    return (
      <div className="mx-auto max-w-md text-center">
        <div className="mb-6 flex justify-center">
          <div className="flex h-16 w-16 items-center justify-center rounded-full bg-red-100">
            <XCircle className="h-10 w-10 text-red-600" />
          </div>
        </div>
        <h1 className="text-2xl font-semibold text-sercha-ink-slate">
          Something went wrong
        </h1>
        <p className="mt-2 text-sm text-sercha-fog-grey">{error}</p>
        <div className="mt-8 flex gap-3">
          <button
            onClick={handleAddAnother}
            className="flex-1 rounded-lg border border-sercha-silverline bg-white px-4 py-2.5 text-sm font-medium text-sercha-fog-grey transition-colors hover:bg-sercha-mist"
          >
            Try Again
          </button>
          <button
            onClick={onSkip}
            className="flex-1 rounded-lg bg-sercha-indigo px-4 py-2.5 text-sm font-medium text-white transition-colors hover:bg-sercha-indigo/90"
          >
            Skip for now
          </button>
        </div>
      </div>
    );
  }

  // Creating view
  if (view === "creating") {
    return (
      <div className="mx-auto max-w-md text-center">
        <div className="mb-6 flex justify-center">
          <Loader2 className="h-12 w-12 animate-spin text-sercha-indigo" />
        </div>
        <h1 className="text-2xl font-semibold text-sercha-ink-slate">
          Creating Source...
        </h1>
        <p className="mt-2 text-sm text-sercha-fog-grey">
          Setting up your data source. This will only take a moment.
        </p>
      </div>
    );
  }

  // Installation picker view
  if (view === "connection_picker") {
    return (
      <div className="mx-auto max-w-lg">
        <button
          onClick={handleBack}
          className="mb-4 flex items-center gap-1 text-sm text-sercha-fog-grey hover:text-sercha-ink-slate"
        >
          <ArrowLeft className="h-4 w-4" />
          Back
        </button>

        <div className="mb-8 flex items-center gap-4">
          {pendingProvider && (
            <Image
              src={PROVIDER_ICONS[pendingProvider.type] || "/logos/default.png"}
              alt={pendingProvider.name}
              width={48}
              height={48}
              className="h-12 w-12"
            />
          )}
          <div>
            <h1 className="text-2xl font-semibold text-sercha-ink-slate">
              Choose {pendingProvider?.name} Account
            </h1>
            <p className="mt-1 text-sm text-sercha-fog-grey">
              Select an existing connection or connect a new account.
            </p>
          </div>
        </div>

        {/* Existing connections */}
        <div className="mb-6 space-y-3">
          <h3 className="text-sm font-medium text-sercha-fog-grey">
            Existing Connections
          </h3>
          {existingConnections.map((connection) => (
            <button
              key={connection.id}
              onClick={() => handleSelectExistingConnection(connection)}
              className="flex w-full items-center gap-3 rounded-lg border border-sercha-silverline bg-white p-4 text-left transition-all hover:border-sercha-indigo hover:shadow-md"
            >
              <Image
                src={PROVIDER_ICONS[connection.provider_type] || "/logos/default.png"}
                alt={connection.provider_type}
                width={32}
                height={32}
                className="h-8 w-8"
              />
              <div className="flex-1">
                <p className="text-sm font-medium text-sercha-ink-slate">
                  {connection.name}
                </p>
                <p className="text-xs text-sercha-fog-grey">
                  {connection.account_id} · {connection.source_count} source{connection.source_count !== 1 ? "s" : ""}
                </p>
              </div>
              <span className="text-xs text-sercha-indigo">Select →</span>
            </button>
          ))}
        </div>

        {/* Connect new account */}
        <div className="border-t border-sercha-silverline pt-6">
          <button
            onClick={() => startOAuthFlow(pendingProvider!)}
            className="flex w-full items-center justify-center gap-2 rounded-lg border-2 border-dashed border-sercha-silverline bg-sercha-mist/50 p-4 text-sm font-medium text-sercha-fog-grey transition-colors hover:border-sercha-indigo hover:text-sercha-indigo"
          >
            <ExternalLink className="h-4 w-4" />
            Connect New {pendingProvider?.name} Account
          </button>
        </div>
      </div>
    );
  }

  // Connecting view
  if (view === "connecting") {
    return (
      <div className="mx-auto max-w-md text-center">
        <div className="mb-6 flex justify-center">
          <Loader2 className="h-12 w-12 animate-spin text-sercha-indigo" />
        </div>
        <h1 className="text-2xl font-semibold text-sercha-ink-slate">
          Connecting to {pendingProvider?.name}...
        </h1>
        <p className="mt-2 text-sm text-sercha-fog-grey">
          You&apos;ll be redirected to authorize access.
        </p>
      </div>
    );
  }

  // Container selection view
  if (view === "select") {
    // Check provider type from selectedConnection (more reliable than pendingProvider which may not be set after OAuth return)
    const providerType = selectedConnection?.provider_type || pendingProvider?.type;
    const showFolderNavigation = providerType === "google_drive" || providerType === "onedrive";

    return (
      <div className="mx-auto max-w-2xl">
        <button
          onClick={handleBack}
          className="mb-4 flex items-center gap-1 text-sm text-sercha-fog-grey hover:text-sercha-ink-slate"
        >
          <ArrowLeft className="h-4 w-4" />
          Back
        </button>

        <div className="mb-8 text-center">
          <h1 className="text-2xl font-semibold text-sercha-ink-slate">
            Select Content to Sync
          </h1>
          <p className="mt-2 text-sm text-sercha-fog-grey">
            Choose which {pendingProvider?.type === "github" ? "repositories" : "folders"} to
            index. {showFolderNavigation && "Click on folders to browse contents."}
          </p>
        </div>

        {/* Breadcrumb Navigation - Only for folder-based providers */}
        {showFolderNavigation && folderPath.length > 0 && (
          <div className="mb-4 flex items-center gap-1 text-sm overflow-x-auto">
            <button
              onClick={() => handleBreadcrumbClick(-1)}
              className="flex items-center gap-1 text-sercha-indigo hover:underline shrink-0"
              disabled={isLoadingContainers}
            >
              <Home className="h-4 w-4" />
              Root
            </button>
            {folderPath.map((folder, index) => (
              <span key={folder.id} className="flex items-center gap-1 shrink-0">
                <ChevronRight className="h-4 w-4 text-sercha-fog-grey" />
                <button
                  onClick={() => handleBreadcrumbClick(index)}
                  className={`hover:underline ${
                    index === folderPath.length - 1
                      ? "text-sercha-ink-slate font-medium"
                      : "text-sercha-indigo"
                  }`}
                  disabled={isLoadingContainers || index === folderPath.length - 1}
                >
                  {folder.name}
                </button>
              </span>
            ))}
          </div>
        )}

        {/* Search and Select All */}
        <div className="mb-4 flex items-center gap-4">
          <div className="relative flex-1">
            <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-sercha-fog-grey" />
            <input
              type="text"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="w-full rounded-lg border border-sercha-silverline bg-white py-2 pl-10 pr-4 text-sm text-sercha-ink-slate placeholder:text-sercha-fog-grey focus:border-sercha-indigo focus:outline-none focus:ring-2 focus:ring-sercha-indigo/20"
              placeholder="Search..."
            />
          </div>
          <button
            onClick={handleSelectAll}
            className="text-sm font-medium text-sercha-indigo hover:underline"
          >
            {selectedContainers.size === containers.length
              ? "Deselect All"
              : "Select All"}
          </button>
        </div>

        {/* Selection Count */}
        <p className="mb-4 text-sm text-sercha-fog-grey">
          {selectedContainers.size} selected
          {showFolderNavigation && folderPath.length > 0 && (
            <span className="ml-2 text-sercha-indigo">
              (browsing: {folderPath[folderPath.length - 1].name})
            </span>
          )}
        </p>

        {/* Container List */}
        <div className="mb-6 max-h-80 space-y-2 overflow-y-auto rounded-xl border border-sercha-silverline bg-white p-2">
          {isLoadingContainers ? (
            <div className="flex items-center justify-center py-8">
              <Loader2 className="h-6 w-6 animate-spin text-sercha-indigo" />
            </div>
          ) : filteredContainers.length === 0 ? (
            <div className="py-8 text-center text-sm text-sercha-fog-grey">
              {searchQuery ? "No matching items found" : "No folders found"}
            </div>
          ) : (
            filteredContainers.map((container) => (
              <div
                key={container.id}
                className="flex items-center gap-3 rounded-lg p-3 transition-colors hover:bg-sercha-mist"
              >
                <input
                  type="checkbox"
                  checked={selectedContainers.has(container.id)}
                  onChange={() => handleToggleContainer(container.id)}
                  className="h-4 w-4 rounded border-sercha-silverline text-sercha-indigo focus:ring-sercha-indigo/20"
                />
                {container.type === "folder" && (
                  <Folder className="h-5 w-5 text-sercha-fog-grey shrink-0" />
                )}
                <div className="min-w-0 flex-1">
                  <p className="truncate text-sm font-medium text-sercha-ink-slate">
                    {container.name}
                  </p>
                  {container.description && (
                    <p className="truncate text-xs text-sercha-fog-grey">
                      {container.description}
                    </p>
                  )}
                </div>
                {(container.metadata as { private?: boolean } | undefined)
                  ?.private && (
                  <span className="rounded-full bg-sercha-mist px-2 py-0.5 text-xs text-sercha-fog-grey shrink-0">
                    Private
                  </span>
                )}
                {container.has_children && (
                  <button
                    onClick={() => handleDrillDown(container)}
                    className="flex items-center gap-1 text-xs text-sercha-indigo hover:underline shrink-0"
                    disabled={isLoadingContainers}
                  >
                    Browse
                    <ChevronRight className="h-4 w-4" />
                  </button>
                )}
              </div>
            ))
          )}
        </div>

        {/* Action Buttons */}
        <div className="flex gap-3">
          <button
            onClick={handleBack}
            className="flex-1 rounded-lg border border-sercha-silverline bg-white px-4 py-2.5 text-sm font-medium text-sercha-fog-grey transition-colors hover:bg-sercha-mist"
          >
            Cancel
          </button>
          <button
            onClick={handleCreateSource}
            disabled={selectedContainers.size === 0}
            className="flex flex-1 items-center justify-center rounded-lg bg-sercha-indigo px-4 py-2.5 text-sm font-medium text-white transition-colors hover:bg-sercha-indigo/90 focus:outline-none focus:ring-2 focus:ring-sercha-indigo/50 disabled:cursor-not-allowed disabled:opacity-60"
          >
            Create Source
          </button>
        </div>
      </div>
    );
  }

  // Main selection view (default)
  return (
    <div className="mx-auto max-w-2xl">
      <div className="mb-8 text-center">
        <h1 className="text-2xl font-semibold text-sercha-ink-slate">
          Connect Data Sources
        </h1>
        <p className="mt-2 text-sm text-sercha-fog-grey">
          Connect your accounts to start indexing your data. You can add more
          sources later from the dashboard.
        </p>
      </div>

      {/* Connected Sources */}
      {connectedSources.length > 0 && (
        <div className="mb-8">
          <h3 className="mb-3 text-sm font-medium text-sercha-fog-grey">
            Connected Sources
          </h3>
          <div className="flex flex-wrap gap-2">
            {connectedSources.map((source) => (
              <div
                key={source.id}
                className="flex items-center gap-2 rounded-lg border border-emerald-200 bg-emerald-50 px-3 py-2"
              >
                <Image
                  src={PROVIDER_ICONS[source.provider_type] || "/logos/default.png"}
                  alt={source.provider_type}
                  width={20}
                  height={20}
                  className="h-5 w-5"
                />
                <span className="text-sm text-sercha-ink-slate">{source.name}</span>
                <button
                  onClick={() => handleRemoveSource(source.id)}
                  className="ml-1 text-sercha-fog-grey hover:text-red-600"
                >
                  <X className="h-4 w-4" />
                </button>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Provider Grid */}
      <div className="mb-8">
        <h3 className="mb-3 text-sm font-medium text-sercha-fog-grey">
          Add a Data Source
        </h3>
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {providers.map((provider) => (
            <div
              key={provider.type}
              className="flex flex-col items-center rounded-xl border border-sercha-silverline bg-white p-6 transition-all hover:border-sercha-indigo hover:shadow-md"
            >
              <button
                onClick={() => handleSelectProvider(provider)}
                className="flex flex-col items-center"
              >
                <Image
                  src={PROVIDER_ICONS[provider.type] || "/logos/default.png"}
                  alt={provider.name}
                  width={48}
                  height={48}
                  className="mb-3 h-12 w-12"
                />
                <p className="text-sm font-medium text-sercha-ink-slate">
                  {provider.name}
                </p>
                <p className="mt-1 text-xs text-sercha-fog-grey">
                  {provider.configured ? "Ready to connect" : "Not available"}
                </p>
              </button>
            </div>
          ))}
        </div>
      </div>

      {/* Action Buttons */}
      <div className="flex gap-3">
        <button
          onClick={onSkip}
          className="flex-1 rounded-lg border border-sercha-silverline bg-white px-4 py-2.5 text-sm font-medium text-sercha-fog-grey transition-colors hover:bg-sercha-mist"
        >
          Skip for now
        </button>
        <button
          onClick={onComplete}
          disabled={connectedSources.length === 0}
          className="flex flex-1 items-center justify-center rounded-lg bg-sercha-indigo px-4 py-2.5 text-sm font-medium text-white transition-colors hover:bg-sercha-indigo/90 focus:outline-none focus:ring-2 focus:ring-sercha-indigo/50 disabled:cursor-not-allowed disabled:opacity-60"
        >
          Continue
        </button>
      </div>
    </div>
  );
}
