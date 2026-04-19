"use client";

import { useState, useEffect, useCallback } from "react";
import { AdminLayout } from "@/components/layout";
import { CapabilityCard } from "@/components/capabilities/capability-card";
import {
  Loader2,
  AlertCircle,
  RefreshCw,
  Check,
} from "lucide-react";
import {
  getCapabilities,
  getCapabilityPreferences,
  updateCapabilityPreferences,
  type CapabilitiesResponse,
  type CapabilityPreferencesResponse,
  type UpdateCapabilityPreferencesRequest,
} from "@/lib/api";

// Capability configuration with static metadata
interface CapabilityConfig {
  id: string;
  name: string;
  description: string;
  phase: "indexing" | "search";
  backend: string;
  prefKey: keyof CapabilityPreferencesResponse;
  dependsOn?: keyof CapabilityPreferencesResponse;
  dependsOnLabel?: string;
}

const CAPABILITY_CONFIGS: CapabilityConfig[] = [
  // Indexing capabilities
  {
    id: "text_indexing",
    name: "Text Indexing (BM25)",
    description: "Full-text indexing for keyword search",
    phase: "indexing",
    backend: "OpenSearch",
    prefKey: "text_indexing_enabled",
  },
  {
    id: "embedding_indexing",
    name: "Embedding Indexing",
    description: "Vector embeddings for semantic search",
    phase: "indexing",
    backend: "pgvector",
    prefKey: "embedding_indexing_enabled",
  },
  // Search capabilities
  {
    id: "bm25_search",
    name: "BM25 Search",
    description: "Keyword-based text search",
    phase: "search",
    backend: "OpenSearch",
    prefKey: "bm25_search_enabled",
    dependsOn: "text_indexing_enabled",
    dependsOnLabel: "Text Indexing",
  },
  {
    id: "vector_search",
    name: "Vector Search",
    description: "Semantic similarity search",
    phase: "search",
    backend: "pgvector",
    prefKey: "vector_search_enabled",
    dependsOn: "embedding_indexing_enabled",
    dependsOnLabel: "Embedding Indexing",
  },
  {
    id: "query_expansion",
    name: "Query Expansion",
    description: "Expands queries with related terms using LLM",
    phase: "search",
    backend: "LLM",
    prefKey: "query_expansion_enabled",
  },
];

export default function CapabilitiesPage() {
  const [capabilities, setCapabilities] = useState<CapabilitiesResponse | null>(null);
  const [preferences, setPreferences] = useState<CapabilityPreferencesResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState<string | null>(null);
  const [savedKey, setSavedKey] = useState<string | null>(null);

  const fetchData = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);

      const [capsData, prefsData] = await Promise.all([
        getCapabilities().catch(() => null),
        getCapabilityPreferences().catch(() => null),
      ]);

      setCapabilities(capsData);
      setPreferences(prefsData);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load capabilities");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  // Check capability availability from backend features response
  const isCapabilityAvailable = (capabilityId: string): boolean => {
    if (!capabilities?.features) return false;
    const features = capabilities.features as Record<string, { available: boolean; enabled: boolean; active: boolean }>;
    return features[capabilityId]?.available ?? false;
  };

  const handleToggle = async (prefKey: keyof CapabilityPreferencesResponse, enabled: boolean) => {
    if (!preferences) return;

    setSaving(prefKey);
    setError(null);

    try {
      const updateReq: UpdateCapabilityPreferencesRequest = {
        [prefKey]: enabled,
      };

      const updated = await updateCapabilityPreferences(updateReq);
      setPreferences(updated);
      setSavedKey(prefKey);
      setTimeout(() => setSavedKey(null), 1500);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to update capability");
    } finally {
      setSaving(null);
    }
  };

  // Separate capabilities by phase and backend
  const indexingCapabilities = CAPABILITY_CONFIGS.filter((c) => c.phase === "indexing");
  const searchCapabilities = CAPABILITY_CONFIGS.filter((c) => c.phase === "search" && c.backend !== "LLM");
  const llmCapabilities = CAPABILITY_CONFIGS.filter((c) => c.backend === "LLM");

  if (loading) {
    return (
      <AdminLayout title="Capabilities" description="Manage search and indexing capabilities">
        <div className="flex items-center justify-center py-12">
          <Loader2 className="h-8 w-8 animate-spin text-sercha-indigo" />
        </div>
      </AdminLayout>
    );
  }

  return (
    <AdminLayout title="Capabilities" description="Manage search and indexing capabilities">
      <div className="space-y-8">
        {/* Header with refresh */}
        <div className="flex items-center justify-between">
          <div>
            <p className="text-sm text-sercha-fog-grey">
              Configure which search and indexing capabilities are enabled for your team.
              Disabling indexing will also disable dependent search capabilities.
            </p>
          </div>
          <button
            onClick={fetchData}
            disabled={loading}
            className="flex items-center gap-2 rounded-lg border border-sercha-silverline px-3 py-2 text-sm text-sercha-fog-grey hover:bg-sercha-mist disabled:opacity-50"
          >
            <RefreshCw className={`h-4 w-4 ${loading ? "animate-spin" : ""}`} />
            Refresh
          </button>
        </div>

        {/* Error display */}
        {error && (
          <div className="flex items-center gap-2 rounded-lg bg-red-50 p-4 text-red-600">
            <AlertCircle className="h-5 w-5" />
            {error}
          </div>
        )}

        {/* Feature flags from capabilities response */}
        {capabilities?.features && (
          <div className="rounded-xl border border-sercha-silverline bg-white p-4">
            <h3 className="mb-3 text-sm font-medium text-sercha-ink-slate">Backend Availability</h3>
            <div className="flex flex-wrap gap-3">
              {[
                { key: "text_indexing", label: "Text Indexing" },
                { key: "embedding_indexing", label: "Embedding Indexing" },
                { key: "bm25_search", label: "BM25 Search" },
                { key: "vector_search", label: "Vector Search" },
                { key: "query_expansion", label: "Query Expansion" },
              ].map(({ key, label }) => {
                const feature = (capabilities.features as Record<string, { available: boolean; enabled: boolean; active: boolean }>)[key];
                const available = feature?.available ?? false;
                return (
                  <span
                    key={key}
                    className={`inline-flex items-center gap-1.5 rounded-full px-3 py-1 text-xs font-medium ${
                      available
                        ? "bg-emerald-100 text-emerald-700"
                        : "bg-gray-100 text-gray-500"
                    }`}
                  >
                    <span
                      className={`h-1.5 w-1.5 rounded-full ${
                        available ? "bg-emerald-500" : "bg-gray-400"
                      }`}
                    />
                    {label}
                  </span>
                );
              })}
            </div>
          </div>
        )}

        {/* Indexing Capabilities */}
        <section>
          <h2 className="mb-4 text-lg font-semibold text-sercha-ink-slate">
            Indexing Capabilities
          </h2>
          <div className="grid gap-4 sm:grid-cols-2">
            {indexingCapabilities.map((config) => {
              const isEnabled = preferences?.[config.prefKey] as boolean ?? false;
              const available = isCapabilityAvailable(config.id);
              const isSaving = saving === config.prefKey;
              const justSaved = savedKey === config.prefKey;

              return (
                <div key={config.id} className="relative">
                  <CapabilityCard
                    name={config.name}
                    description={config.description}
                    backend={config.backend}
                    phase={config.phase}
                    available={available}
                    enabled={isEnabled}
                    onToggle={(enabled) => handleToggle(config.prefKey, enabled)}
                    disabled={isSaving}
                  />
                  {/* Saving/saved indicator */}
                  {(isSaving || justSaved) && (
                    <div className="absolute right-2 top-2">
                      {isSaving ? (
                        <Loader2 className="h-4 w-4 animate-spin text-sercha-indigo" />
                      ) : (
                        <Check className="h-4 w-4 text-emerald-500" />
                      )}
                    </div>
                  )}
                </div>
              );
            })}
          </div>
        </section>

        {/* Search Capabilities */}
        <section>
          <h2 className="mb-4 text-lg font-semibold text-sercha-ink-slate">
            Search Capabilities
          </h2>
          <div className="grid gap-4 sm:grid-cols-2">
            {searchCapabilities.map((config) => {
              const isEnabled = preferences?.[config.prefKey] as boolean ?? false;
              const available = isCapabilityAvailable(config.id);
              const dependencyMet = config.dependsOn
                ? (preferences?.[config.dependsOn] as boolean ?? false)
                : true;
              const isSaving = saving === config.prefKey;
              const justSaved = savedKey === config.prefKey;

              return (
                <div key={config.id} className="relative">
                  <CapabilityCard
                    name={config.name}
                    description={config.description}
                    backend={config.backend}
                    phase={config.phase}
                    available={available}
                    enabled={isEnabled}
                    dependsOn={config.dependsOnLabel}
                    dependencyMet={dependencyMet}
                    onToggle={(enabled) => handleToggle(config.prefKey, enabled)}
                    disabled={isSaving}
                  />
                  {/* Saving/saved indicator */}
                  {(isSaving || justSaved) && (
                    <div className="absolute right-2 top-2">
                      {isSaving ? (
                        <Loader2 className="h-4 w-4 animate-spin text-sercha-indigo" />
                      ) : (
                        <Check className="h-4 w-4 text-emerald-500" />
                      )}
                    </div>
                  )}
                </div>
              );
            })}
          </div>
        </section>

        {/* LLM Enhancements */}
        <section>
          <h2 className="mb-4 text-lg font-semibold text-sercha-ink-slate">
            LLM Enhancements
          </h2>
          <p className="mb-4 text-sm text-sercha-fog-grey">
            AI-powered features that enhance search quality. Requires an LLM provider to be configured.
          </p>
          <div className="grid gap-4 sm:grid-cols-2">
            {llmCapabilities.map((config) => {
              const isEnabled = preferences?.[config.prefKey] as boolean ?? false;
              const available = isCapabilityAvailable(config.id);
              const isSaving = saving === config.prefKey;
              const justSaved = savedKey === config.prefKey;

              return (
                <div key={config.id} className="relative">
                  <CapabilityCard
                    name={config.name}
                    description={config.description}
                    backend={config.backend}
                    phase={config.phase}
                    available={available}
                    enabled={isEnabled}
                    onToggle={(enabled) => handleToggle(config.prefKey, enabled)}
                    disabled={isSaving}
                  />
                  {/* Saving/saved indicator */}
                  {(isSaving || justSaved) && (
                    <div className="absolute right-2 top-2">
                      {isSaving ? (
                        <Loader2 className="h-4 w-4 animate-spin text-sercha-indigo" />
                      ) : (
                        <Check className="h-4 w-4 text-emerald-500" />
                      )}
                    </div>
                  )}
                </div>
              );
            })}
          </div>
        </section>

        {/* Info note */}
        <div className="rounded-xl border border-sercha-indigo/20 bg-sercha-indigo/5 p-4">
          <h3 className="mb-2 text-sm font-medium text-sercha-ink-slate">
            About Capabilities
          </h3>
          <ul className="space-y-1 text-sm text-sercha-fog-grey">
            <li>
              <strong>Indexing capabilities</strong> control how documents are processed and stored.
            </li>
            <li>
              <strong>Search capabilities</strong> control which search methods are available.
            </li>
            <li>
              <strong>LLM enhancements</strong> use AI to improve search quality (requires LLM provider).
            </li>
            <li>
              Disabling an indexing capability will automatically disable its dependent search capability.
            </li>
            <li>
              Changes take effect immediately for new searches. Existing indexes are preserved.
            </li>
          </ul>
        </div>
      </div>
    </AdminLayout>
  );
}
