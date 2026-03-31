"use client";

import { useState, useEffect, Suspense } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import Image from "next/image";
import Link from "next/link";
import {
  Loader2,
  CheckCircle2,
  XCircle,
  Search,
  ArrowLeft,
  ExternalLink,
  ChevronRight,
  Folder,
  Home,
} from "lucide-react";
import { AdminLayout } from "@/components/layout";
import {
  listProviders,
  listSources,
  listInstallations,
  getCapabilities,
  startOAuth,
  getInstallation,
  getInstallationContainers,
  createSource,
  ApiError,
  type ProviderListItem,
  type SourceSummary,
  type InstallationSummary,
  type Container,
} from "@/lib/api";
import { PROVIDER_ICONS } from "@/lib/providers";

type ViewState =
  | "selection"
  | "installation_picker"
  | "connecting"
  | "select"
  | "creating"
  | "success"
  | "error";

function AddSourceWizardContent() {
  const router = useRouter();
  const searchParams = useSearchParams();

  // View state
  const [view, setView] = useState<ViewState>("selection");
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Data state
  const [providers, setProviders] = useState<ProviderListItem[]>([]);
  const [pendingProvider, setPendingProvider] = useState<ProviderListItem | null>(null);

  // Installation & container state
  const [selectedInstallation, setSelectedInstallation] = useState<InstallationSummary | null>(null);
  const [existingInstallations, setExistingInstallations] = useState<InstallationSummary[]>([]);
  const [containers, setContainers] = useState<Container[]>([]);
  const [selectedContainers, setSelectedContainers] = useState<Set<string>>(new Set());
  const [searchQuery, setSearchQuery] = useState("");

  // Success tracking
  const [createdSourceName, setCreatedSourceName] = useState("");

  // Folder navigation state
  const [folderPath, setFolderPath] = useState<{id: string; name: string}[]>([]);
  const [isLoadingContainers, setIsLoadingContainers] = useState(false);

  // Load data on mount
  useEffect(() => {
    const loadInitialData = async () => {
      try {
        const [caps, providerList] = await Promise.all([
          getCapabilities(),
          listProviders(),
        ]);

        // Filter providers to only show those configured in environment
        const configuredProviders = providerList.filter((p) =>
          caps.oauth_providers.includes(p.type)
        );
        setProviders(configuredProviders);

        // Handle OAuth callback return (installation_id and provider come from URL)
        const installationId = searchParams.get("installation_id");
        const providerFromUrl = searchParams.get("provider");

        if (installationId) {
          await handleOAuthReturn(installationId, providerFromUrl || undefined, configuredProviders);
        }
      } catch {
        setProviders([]);
      } finally {
        setIsLoading(false);
      }
    };
    loadInitialData();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const loadContainers = async (instId: string, parentId?: string) => {
    setIsLoadingContainers(true);
    try {
      const result = await getInstallationContainers(instId, undefined, parentId);
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
    if (!selectedInstallation || !container.has_children) return;
    setFolderPath([...folderPath, { id: container.id, name: container.name }]);
    await loadContainers(selectedInstallation.id, container.id);
  };

  // Navigate via breadcrumb
  const handleBreadcrumbClick = async (index: number) => {
    if (!selectedInstallation) return;
    if (index === -1) {
      // Go to root
      setFolderPath([]);
      await loadContainers(selectedInstallation.id);
    } else {
      const newPath = folderPath.slice(0, index + 1);
      setFolderPath(newPath);
      await loadContainers(selectedInstallation.id, newPath[newPath.length - 1].id);
    }
  };

  // Provider selection
  const handleSelectProvider = async (provider: ProviderListItem) => {
    setPendingProvider(provider);
    setError(null);

    // Fetch existing installations for this provider
    try {
      const allInstallations = await listInstallations();
      const providerInstallations = allInstallations.filter(
        (i) => i.provider_type === provider.type
      );

      if (providerInstallations.length > 0) {
        // Show picker to choose existing or connect new
        setExistingInstallations(providerInstallations);
        setView("installation_picker");
      } else {
        // No existing installations, go straight to OAuth
        await startOAuthFlow(provider);
      }
    } catch {
      // If fetch fails, just go to OAuth
      await startOAuthFlow(provider);
    }
  };

  // Select an existing installation
  const handleSelectExistingInstallation = async (installation: InstallationSummary) => {
    setSelectedInstallation(installation);
    await loadContainers(installation.id);
    setView("select");
  };

  // Removed: credentials are now managed via environment variables

  // Start OAuth redirect
  const startOAuthFlow = async (provider: ProviderListItem) => {
    setView("connecting");
    setError(null);
    setSelectedInstallation(null);
    setContainers([]);
    setSelectedContainers(new Set());

    try {
      // Pass "admin-sources" as return_context so OAuth callback returns here
      const result = await startOAuth(provider.type, undefined, "admin-sources");
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
  const handleOAuthReturn = async (instId: string, providerType?: string, providerList?: ProviderListItem[]) => {
    try {
      const installation = await getInstallation(instId);
      setSelectedInstallation(installation);

      // Set pending provider from URL params
      if (providerType) {
        const list = providerList || providers;
        const provider = list.find((p) => p.type === providerType);
        if (provider) setPendingProvider(provider);
      }

      await loadContainers(instId);
      setView("select");

      // Clean up URL params
      router.replace("/admin/sources/new", { scroll: false });
    } catch (err) {
      if (err instanceof ApiError) {
        setError(err.message || "Failed to load installation");
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
    if (!selectedInstallation || selectedContainers.size === 0) return;

    setView("creating");
    setError(null);

    try {
      // Fetch existing sources to generate unique name
      const existingSources = await listSources().catch(() => [] as SourceSummary[]);

      // Generate unique source name
      const baseName = selectedInstallation.name;
      const existingNames = new Set(existingSources.map((s) => s.name));

      let sourceName: string;
      if (!existingNames.has(baseName)) {
        sourceName = baseName;
      } else {
        let num = 2;
        while (existingNames.has(`${baseName} (${num})`)) {
          num++;
        }
        sourceName = `${baseName} (${num})`;
      }

      await createSource({
        name: sourceName,
        provider_type: selectedInstallation.provider_type,
        installation_id: selectedInstallation.id,
        selected_containers: Array.from(selectedContainers),
      });
      setCreatedSourceName(sourceName);
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

  // Reset to add another source
  const handleAddAnother = () => {
    setPendingProvider(null);
    setSelectedInstallation(null);
    setExistingInstallations([]);
    setContainers([]);
    setSelectedContainers(new Set());
    setSearchQuery("");
    setFolderPath([]);
    setError(null);
    setView("selection");
  };

  // Back navigation
  const handleBack = () => {
    if (view === "installation_picker") {
      setPendingProvider(null);
      setExistingInstallations([]);
      setView("selection");
    } else if (view === "select") {
      if (existingInstallations.length > 0) {
        setSelectedInstallation(null);
        setContainers([]);
        setSelectedContainers(new Set());
        setView("installation_picker");
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
      <AdminLayout title="Add Source" description="Connect a new data source">
        <div className="flex items-center justify-center py-12">
          <Loader2 className="h-8 w-8 animate-spin text-sercha-indigo" />
        </div>
      </AdminLayout>
    );
  }

  // Success view
  if (view === "success") {
    return (
      <AdminLayout title="Source Created" description="Your new data source is ready">
        <div className="mx-auto max-w-md py-12 text-center">
          <div className="mb-6 flex justify-center">
            <div className="flex h-16 w-16 items-center justify-center rounded-full bg-emerald-100">
              <CheckCircle2 className="h-10 w-10 text-emerald-600" />
            </div>
          </div>
          <h1 className="text-2xl font-semibold text-sercha-ink-slate">
            Source Created!
          </h1>
          <p className="mt-2 text-sm text-sercha-fog-grey">
            {createdSourceName} has been connected. {selectedContainers.size}{" "}
            {selectedContainers.size === 1 ? "item" : "items"} will be synced.
          </p>
          <div className="mt-8 flex justify-center gap-4">
            <button
              onClick={handleAddAnother}
              className="rounded-lg border border-sercha-silverline bg-white px-6 py-2.5 text-sm font-medium text-sercha-ink-slate transition-colors hover:bg-sercha-mist"
            >
              Add Another
            </button>
            <Link
              href="/admin/sources"
              className="rounded-lg bg-sercha-indigo px-6 py-2.5 text-sm font-medium text-white transition-colors hover:bg-sercha-indigo/90"
            >
              View Sources
            </Link>
          </div>
        </div>
      </AdminLayout>
    );
  }

  // Error view
  if (view === "error") {
    return (
      <AdminLayout title="Error" description="Something went wrong">
        <div className="mx-auto max-w-md py-12 text-center">
          <div className="mb-6 flex justify-center">
            <div className="flex h-16 w-16 items-center justify-center rounded-full bg-red-100">
              <XCircle className="h-10 w-10 text-red-600" />
            </div>
          </div>
          <h1 className="text-2xl font-semibold text-sercha-ink-slate">
            Something went wrong
          </h1>
          <p className="mt-2 text-sm text-sercha-fog-grey">{error}</p>
          <div className="mt-8 flex justify-center gap-4">
            <button
              onClick={handleAddAnother}
              className="rounded-lg border border-sercha-silverline bg-white px-6 py-2.5 text-sm font-medium text-sercha-ink-slate transition-colors hover:bg-sercha-mist"
            >
              Try Again
            </button>
            <Link
              href="/admin/sources"
              className="rounded-lg bg-sercha-indigo px-6 py-2.5 text-sm font-medium text-white transition-colors hover:bg-sercha-indigo/90"
            >
              Back to Sources
            </Link>
          </div>
        </div>
      </AdminLayout>
    );
  }

  // Creating view
  if (view === "creating") {
    return (
      <AdminLayout title="Creating Source" description="Setting up your data source">
        <div className="mx-auto max-w-md py-12 text-center">
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
      </AdminLayout>
    );
  }

  // Connecting view
  if (view === "connecting") {
    return (
      <AdminLayout title="Connecting" description="Redirecting to authorize">
        <div className="mx-auto max-w-md py-12 text-center">
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
      </AdminLayout>
    );
  }

  // Installation picker view
  if (view === "installation_picker") {
    return (
      <AdminLayout title="Choose Account" description={`Select ${pendingProvider?.name} account`}>
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
                src={PROVIDER_ICONS[pendingProvider.type] || "/icon.svg"}
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

          {/* Existing installations */}
          <div className="mb-6 space-y-3">
            <h3 className="text-sm font-medium text-sercha-fog-grey">
              Existing Connections
            </h3>
            {existingInstallations.map((installation) => (
              <button
                key={installation.id}
                onClick={() => handleSelectExistingInstallation(installation)}
                className="flex w-full items-center gap-3 rounded-lg border border-sercha-silverline bg-white p-4 text-left transition-all hover:border-sercha-indigo hover:shadow-md"
              >
                <Image
                  src={PROVIDER_ICONS[installation.provider_type] || "/icon.svg"}
                  alt={installation.provider_type}
                  width={32}
                  height={32}
                  className="h-8 w-8"
                />
                <div className="flex-1">
                  <p className="text-sm font-medium text-sercha-ink-slate">
                    {installation.name}
                  </p>
                  <p className="text-xs text-sercha-fog-grey">
                    {installation.account_id} · {installation.source_count} source{installation.source_count !== 1 ? "s" : ""}
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
      </AdminLayout>
    );
  }

  // Container selection view
  if (view === "select") {
    // Check provider type from selectedInstallation (more reliable than pendingProvider)
    const providerType = selectedInstallation?.provider_type || pendingProvider?.type;
    const showFolderNavigation = providerType === "google_drive" || providerType === "onedrive";

    return (
      <AdminLayout title="Select Content" description="Choose what to sync">
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
                  {(container.metadata as { private?: boolean } | undefined)?.private && (
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
      </AdminLayout>
    );
  }

  // Main selection view (default)
  return (
    <AdminLayout title="Add Source" description="Connect a new data source">
      <div className="mx-auto max-w-2xl">
        <div className="mb-8 text-center">
          <h1 className="text-2xl font-semibold text-sercha-ink-slate">
            Connect a Data Source
          </h1>
          <p className="mt-2 text-sm text-sercha-fog-grey">
            Choose a provider to connect. You&apos;ll be guided through the
            authorization process.
          </p>
        </div>

        {/* Provider Grid */}
        <div className="mb-8">
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {providers.map((provider) => (
              <button
                key={provider.type}
                onClick={() => handleSelectProvider(provider)}
                className="flex flex-col items-center rounded-xl border border-sercha-silverline bg-white p-6 transition-all hover:border-sercha-indigo hover:shadow-md"
              >
                <Image
                  src={PROVIDER_ICONS[provider.type] || "/icon.svg"}
                  alt={provider.name}
                  width={48}
                  height={48}
                  className="mb-3 h-12 w-12"
                />
                <p className="text-sm font-medium text-sercha-ink-slate">
                  {provider.name}
                </p>
                <p className="mt-1 text-xs text-sercha-fog-grey">
                  {provider.configured ? "Ready to connect" : "Requires setup"}
                </p>
              </button>
            ))}
          </div>
        </div>

        {/* Back Link */}
        <div className="text-center">
          <Link
            href="/admin/sources"
            className="text-sm text-sercha-fog-grey hover:text-sercha-ink-slate"
          >
            ← Back to Sources
          </Link>
        </div>
      </div>
    </AdminLayout>
  );
}

export default function AddSourcePage() {
  return (
    <Suspense
      fallback={
        <AdminLayout title="Add Source" description="Connect a new data source">
          <div className="flex items-center justify-center py-12">
            <Loader2 className="h-8 w-8 animate-spin text-sercha-indigo" />
          </div>
        </AdminLayout>
      }
    >
      <AddSourceWizardContent />
    </Suspense>
  );
}
