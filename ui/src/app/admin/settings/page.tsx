"use client";

import { useState, useEffect, useCallback } from "react";
import { AdminLayout } from "@/components/layout";
import {
  Loader2,
  AlertCircle,
  AlertTriangle,
  Check,
  Save,
  RefreshCw,
  RotateCcw,
} from "lucide-react";
import {
  getSettings,
  updateSettings,
  getAISettings,
  getAIStatus,
  triggerReindex,
  Settings,
  UpdateSettingsRequest,
  AISettingsResponse,
  AISettingsStatus,
} from "@/lib/api";

// Toggle switch component
function Toggle({
  enabled,
  onChange,
  disabled,
}: {
  enabled: boolean;
  onChange: (enabled: boolean) => void;
  disabled?: boolean;
}) {
  return (
    <button
      type="button"
      onClick={() => !disabled && onChange(!enabled)}
      disabled={disabled}
      className={`relative h-6 w-11 rounded-full transition-colors ${
        disabled
          ? "cursor-not-allowed bg-sercha-mist"
          : enabled
            ? "bg-sercha-indigo"
            : "bg-sercha-silverline hover:bg-sercha-fog-grey"
      }`}
    >
      <span
        className={`absolute top-0.5 h-5 w-5 rounded-full bg-white shadow transition-transform ${
          enabled ? "left-[22px]" : "left-0.5"
        }`}
      />
    </button>
  );
}

// General Settings Section
function GeneralSettingsSection({
  settings,
  aiSettings,
  aiStatus,
  onSave,
}: {
  settings: Settings | null;
  aiSettings: AISettingsResponse | null;
  aiStatus: AISettingsStatus | null;
  onSave: (data: UpdateSettingsRequest) => Promise<void>;
}) {
  // Check if embedding is configured for semantic search features
  const isEmbeddingConfigured = aiSettings?.embedding?.is_configured ?? false;
  // Schema upgrade required means embeddings are configured but Vespa schema isn't ready
  const schemaUpgradeRequired = aiStatus?.schema_upgrade_required ?? false;
  // Can only use vector search if embedding is configured AND schema is ready
  const canUseVectorSearch = isEmbeddingConfigured && !schemaUpgradeRequired;
  const [searchMode, setSearchMode] = useState<"hybrid" | "text" | "semantic">("hybrid");
  const [resultsPerPage, setResultsPerPage] = useState(20);
  const [semanticEnabled, setSemanticEnabled] = useState(true);
  const [autoSuggestEnabled, setAutoSuggestEnabled] = useState(true);
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Load initial values from settings
  useEffect(() => {
    if (settings) {
      setSearchMode(settings.default_search_mode);
      setResultsPerPage(settings.results_per_page);
      setSemanticEnabled(settings.semantic_search_enabled);
      setAutoSuggestEnabled(settings.auto_suggest_enabled);
    }
  }, [settings]);

  // Reset search mode and semantic toggle if embedding is not configured or schema not ready
  useEffect(() => {
    if (!canUseVectorSearch) {
      if (searchMode === "hybrid" || searchMode === "semantic") {
        setSearchMode("text");
      }
      if (semanticEnabled) {
        setSemanticEnabled(false);
      }
    }
  }, [canUseVectorSearch, searchMode, semanticEnabled]);

  const handleSave = async () => {
    setSaving(true);
    setError(null);
    setSaved(false);

    try {
      await onSave({
        default_search_mode: searchMode,
        results_per_page: resultsPerPage,
        semantic_search_enabled: semanticEnabled,
        auto_suggest_enabled: autoSuggestEnabled,
      });
      setSaved(true);
      setTimeout(() => setSaved(false), 2000);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to save settings");
    } finally {
      setSaving(false);
    }
  };

  return (
    <section className="rounded-2xl border-2 border-sercha-silverline bg-white p-6">
      <h2 className="mb-6 text-lg font-semibold text-sercha-ink-slate">
        Search Settings
      </h2>

      {error && (
        <div className="mb-4 flex items-center gap-2 rounded-lg bg-red-50 p-3 text-sm text-red-600">
          <AlertCircle className="h-4 w-4" />
          {error}
        </div>
      )}

      <div className="space-y-6">
        {/* Search Mode */}
        <div className="grid gap-4 md:grid-cols-2">
          <div>
            <label className="mb-1.5 block text-sm font-medium text-sercha-ink-slate">
              Default Search Mode
            </label>
            <select
              value={searchMode}
              onChange={(e) => setSearchMode(e.target.value as typeof searchMode)}
              className="w-full rounded-lg border border-sercha-silverline bg-white px-4 py-2.5 text-sm text-sercha-ink-slate focus:border-sercha-indigo focus:outline-none focus:ring-2 focus:ring-sercha-indigo/20"
            >
              <option value="hybrid" disabled={!canUseVectorSearch}>
                Hybrid (BM25 + Vector){!canUseVectorSearch ? (schemaUpgradeRequired ? " - Schema upgrade required" : " - Requires Embedding") : ""}
              </option>
              <option value="text">Text Only (BM25)</option>
              <option value="semantic" disabled={!canUseVectorSearch}>
                Semantic Only (Vector){!canUseVectorSearch ? (schemaUpgradeRequired ? " - Schema upgrade required" : " - Requires Embedding") : ""}
              </option>
            </select>
            <p className="mt-1 text-xs text-sercha-fog-grey">
              How search queries are processed by default
            </p>
            {schemaUpgradeRequired && (searchMode === "hybrid" || searchMode === "semantic") && (
              <p className="mt-1 flex items-center gap-1 text-xs text-amber-600">
                <AlertCircle className="h-3 w-3" />
                Upgrade the Vespa schema to use this mode (go to Vespa settings)
              </p>
            )}
            {!isEmbeddingConfigured && !schemaUpgradeRequired && (searchMode === "hybrid" || searchMode === "semantic") && (
              <p className="mt-1 flex items-center gap-1 text-xs text-amber-600">
                <AlertCircle className="h-3 w-3" />
                Configure an embedding provider in AI Settings to use this mode
              </p>
            )}
          </div>

          <div>
            <label className="mb-1.5 block text-sm font-medium text-sercha-ink-slate">
              Results Per Page
            </label>
            <input
              type="number"
              value={resultsPerPage}
              onChange={(e) => setResultsPerPage(parseInt(e.target.value) || 20)}
              min={5}
              max={100}
              className="w-full rounded-lg border border-sercha-silverline bg-white px-4 py-2.5 text-sm text-sercha-ink-slate focus:border-sercha-indigo focus:outline-none focus:ring-2 focus:ring-sercha-indigo/20"
            />
            <p className="mt-1 text-xs text-sercha-fog-grey">
              Number of results shown per page (5-100)
            </p>
          </div>
        </div>

        {/* Feature Toggles */}
        <div className="border-t border-sercha-mist pt-6">
          <h3 className="mb-4 text-sm font-medium text-sercha-ink-slate">Features</h3>
          <div className="space-y-4">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm font-medium text-sercha-ink-slate">Semantic Search</p>
                <p className="text-xs text-sercha-fog-grey">
                  Enable AI-powered semantic search capabilities
                </p>
                {schemaUpgradeRequired && semanticEnabled && (
                  <p className="mt-1 flex items-center gap-1 text-xs text-amber-600">
                    <AlertCircle className="h-3 w-3" />
                    Upgrade Vespa schema to enable this feature
                  </p>
                )}
                {!isEmbeddingConfigured && !schemaUpgradeRequired && semanticEnabled && (
                  <p className="mt-1 flex items-center gap-1 text-xs text-amber-600">
                    <AlertCircle className="h-3 w-3" />
                    Requires embedding provider in AI Settings
                  </p>
                )}
              </div>
              <Toggle
                enabled={semanticEnabled}
                onChange={setSemanticEnabled}
                disabled={!canUseVectorSearch && !semanticEnabled}
              />
            </div>

            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm font-medium text-sercha-ink-slate">Auto-Suggest</p>
                <p className="text-xs text-sercha-fog-grey">
                  Show search suggestions as you type
                </p>
              </div>
              <Toggle enabled={autoSuggestEnabled} onChange={setAutoSuggestEnabled} />
            </div>
          </div>
        </div>

        {/* Save Button */}
        <div className="flex justify-end border-t border-sercha-mist pt-4">
          <button
            onClick={handleSave}
            disabled={saving}
            className="inline-flex items-center gap-2 rounded-lg bg-sercha-indigo px-4 py-2 text-sm font-medium text-white hover:bg-sercha-indigo/90 disabled:opacity-50"
          >
            {saving ? (
              <Loader2 className="h-4 w-4 animate-spin" />
            ) : saved ? (
              <Check className="h-4 w-4" />
            ) : (
              <Save className="h-4 w-4" />
            )}
            {saving ? "Saving..." : saved ? "Saved!" : "Save Changes"}
          </button>
        </div>
      </div>
    </section>
  );
}

// Main Settings Page
export default function SettingsPage() {
  const [settings, setSettings] = useState<Settings | null>(null);
  const [aiSettings, setAISettings] = useState<AISettingsResponse | null>(null);
  const [aiStatus, setAIStatus] = useState<AISettingsStatus | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [reindexing, setReindexing] = useState(false);
  const [reindexError, setReindexError] = useState<string | null>(null);

  const fetchData = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);
      const [settingsData, aiData, statusData] = await Promise.all([
        getSettings(),
        getAISettings(),
        getAIStatus(),
      ]);
      setSettings(settingsData);
      setAISettings(aiData);
      setAIStatus(statusData);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load settings");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  const handleReindex = async () => {
    setReindexing(true);
    setReindexError(null);
    try {
      await triggerReindex();
      // Refresh status after triggering reindex
      const statusData = await getAIStatus();
      setAIStatus(statusData);
    } catch (err) {
      setReindexError(err instanceof Error ? err.message : "Failed to trigger reindex");
    } finally {
      setReindexing(false);
    }
  };

  const handleSaveSettings = async (data: UpdateSettingsRequest) => {
    const updated = await updateSettings(data);
    setSettings(updated);
  };

  if (loading) {
    return (
      <AdminLayout title="Other Settings" description="Configure other settings">
        <div className="flex items-center justify-center py-12">
          <Loader2 className="h-8 w-8 animate-spin text-sercha-indigo" />
        </div>
      </AdminLayout>
    );
  }

  return (
    <AdminLayout title="Other Settings" description="Configure other settings">
      <div className="space-y-8">
        {error && (
          <div className="flex items-center gap-2 rounded-lg bg-red-50 p-4 text-red-600">
            <AlertCircle className="h-5 w-5" />
            {error}
            <button
              onClick={fetchData}
              className="ml-auto inline-flex items-center gap-1 text-sm font-medium hover:underline"
            >
              <RefreshCw className="h-4 w-4" />
              Retry
            </button>
          </div>
        )}

        {/* Schema Upgrade Required Banner */}
        {aiStatus?.schema_upgrade_required && (
          <div className="rounded-xl border-2 border-amber-300 bg-amber-50 p-4">
            <div className="flex items-start gap-3">
              <AlertTriangle className="mt-0.5 h-5 w-5 flex-shrink-0 text-amber-600" />
              <div className="flex-1">
                <h3 className="font-semibold text-amber-800">Schema Upgrade Required</h3>
                <p className="mt-1 text-sm text-amber-700">
                  {aiStatus.schema_upgrade_reason || "Vespa schema needs upgrade to support embeddings."}
                </p>
                <p className="mt-2 text-sm text-amber-600">
                  Go to the Vespa page and click &quot;Reconnect&quot; to upgrade the schema. After upgrading, you&apos;ll need to reindex existing documents.
                </p>
                <a
                  href="/admin/vespa"
                  className="mt-3 inline-flex items-center gap-2 rounded-lg bg-amber-600 px-4 py-2 text-sm font-medium text-white hover:bg-amber-700"
                >
                  Go to Vespa Settings
                </a>
              </div>
            </div>
          </div>
        )}

        {/* Reindex Required Banner */}
        {aiStatus?.reindex_required && !(aiStatus?.schema_upgrade_required) && (
          <div className="rounded-xl border-2 border-amber-300 bg-amber-50 p-4">
            <div className="flex items-start gap-3">
              <AlertTriangle className="mt-0.5 h-5 w-5 flex-shrink-0 text-amber-600" />
              <div className="flex-1">
                <h3 className="font-semibold text-amber-800">Reindex Required</h3>
                <p className="mt-1 text-sm text-amber-700">
                  {aiStatus.reindex_reason || "Documents need to be reindexed to enable semantic search."}
                </p>
                {reindexError && (
                  <p className="mt-2 text-sm text-red-600">{reindexError}</p>
                )}
                <button
                  onClick={handleReindex}
                  disabled={reindexing}
                  className="mt-3 inline-flex items-center gap-2 rounded-lg bg-amber-600 px-4 py-2 text-sm font-medium text-white hover:bg-amber-700 disabled:opacity-50"
                >
                  {reindexing ? (
                    <Loader2 className="h-4 w-4 animate-spin" />
                  ) : (
                    <RotateCcw className="h-4 w-4" />
                  )}
                  {reindexing ? "Starting Reindex..." : "Start Reindex"}
                </button>
              </div>
            </div>
          </div>
        )}

        {/* General Settings */}
        <GeneralSettingsSection settings={settings} aiSettings={aiSettings} aiStatus={aiStatus} onSave={handleSaveSettings} />
      </div>
    </AdminLayout>
  );
}
