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
  FolderX,
  FileX,
  Plus,
  X,
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
  SyncExclusionSettings,
  DEFAULT_SYNC_EXCLUSION_PATTERNS,
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

// Search Settings Section
function SearchSettingsSection({
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
  // Schema upgrade required means embeddings are configured but search backend schema isn't ready
  const schemaUpgradeRequired = aiStatus?.schema_upgrade_required ?? false;
  // Can only use vector search if embedding is configured AND schema is ready
  const canUseVectorSearch = isEmbeddingConfigured && !schemaUpgradeRequired;
  const [searchMode, setSearchMode] = useState<"hybrid" | "text" | "semantic">("hybrid");
  const [resultsPerPage, setResultsPerPage] = useState(20);
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Load initial values from settings
  useEffect(() => {
    if (settings) {
      setSearchMode(settings.default_search_mode);
      setResultsPerPage(settings.results_per_page);
    }
  }, [settings]);

  // Reset search mode if embedding is not configured or schema not ready
  useEffect(() => {
    if (!canUseVectorSearch) {
      if (searchMode === "hybrid" || searchMode === "semantic") {
        setSearchMode("text");
      }
    }
  }, [canUseVectorSearch, searchMode]);

  const handleSave = async () => {
    setSaving(true);
    setError(null);
    setSaved(false);

    try {
      await onSave({
        default_search_mode: searchMode,
        results_per_page: resultsPerPage,
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
                Schema upgrade required to use this mode
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

// Sync Configuration Section
function SyncConfigurationSection({
  settings,
  onSave,
}: {
  settings: Settings | null;
  onSave: (data: UpdateSettingsRequest) => Promise<void>;
}) {
  const [syncInterval, setSyncInterval] = useState(60);
  const [syncEnabled, setSyncEnabled] = useState(true);
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Load initial values from settings
  useEffect(() => {
    if (settings) {
      setSyncInterval(settings.sync_interval_minutes);
      setSyncEnabled(settings.sync_enabled);
    }
  }, [settings]);

  const handleSave = async () => {
    setSaving(true);
    setError(null);
    setSaved(false);

    try {
      await onSave({
        sync_interval_minutes: syncInterval,
        sync_enabled: syncEnabled,
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
      <div className="mb-6">
        <h2 className="text-lg font-semibold text-sercha-ink-slate">
          Sync Configuration
        </h2>
        <p className="text-sm text-sercha-fog-grey">
          Configure automatic synchronization from data sources
        </p>
      </div>

      {error && (
        <div className="mb-4 flex items-center gap-2 rounded-lg bg-red-50 p-3 text-sm text-red-600">
          <AlertCircle className="h-4 w-4" />
          {error}
        </div>
      )}

      <div className="space-y-6">
        {/* Sync Enabled Toggle */}
        <div className="flex items-center justify-between">
          <div>
            <p className="text-sm font-medium text-sercha-ink-slate">Sync Enabled</p>
            <p className="text-xs text-sercha-fog-grey">
              Automatically sync data from connected sources
            </p>
          </div>
          <Toggle enabled={syncEnabled} onChange={setSyncEnabled} />
        </div>

        {/* Sync Interval */}
        <div>
          <label className="mb-1.5 block text-sm font-medium text-sercha-ink-slate">
            Sync Interval (minutes)
          </label>
          <input
            type="number"
            value={syncInterval}
            onChange={(e) => setSyncInterval(parseInt(e.target.value) || 60)}
            min={5}
            max={1440}
            className="w-full max-w-xs rounded-lg border border-sercha-silverline bg-white px-4 py-2.5 text-sm text-sercha-ink-slate focus:border-sercha-indigo focus:outline-none focus:ring-2 focus:ring-sercha-indigo/20"
          />
          <p className="mt-1 text-xs text-sercha-fog-grey">
            How often to sync data sources (5-1440 minutes)
          </p>
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
            {saving ? "Saving..." : saved ? "Saved!" : "Save Configuration"}
          </button>
        </div>
      </div>
    </section>
  );
}

// Sync Exclusions Section
function SyncExclusionsSection({
  settings,
  onSave,
}: {
  settings: Settings | null;
  onSave: (data: UpdateSettingsRequest) => Promise<void>;
}) {
  const [enabledPatterns, setEnabledPatterns] = useState<string[]>([]);
  const [disabledPatterns, setDisabledPatterns] = useState<string[]>([]);
  const [customPatterns, setCustomPatterns] = useState<string[]>([]);
  const [newPattern, setNewPattern] = useState("");
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Initialize from settings or defaults
  useEffect(() => {
    if (settings?.sync_exclusions) {
      setEnabledPatterns(settings.sync_exclusions.enabled_patterns || []);
      setDisabledPatterns(settings.sync_exclusions.disabled_patterns || []);
      setCustomPatterns(settings.sync_exclusions.custom_patterns || []);
    } else {
      // Default: all patterns enabled
      setEnabledPatterns(DEFAULT_SYNC_EXCLUSION_PATTERNS.map(p => p.pattern));
      setDisabledPatterns([]);
      setCustomPatterns([]);
    }
  }, [settings]);

  const isPatternEnabled = (pattern: string) => {
    // If no settings configured, all defaults are enabled
    if (!settings?.sync_exclusions) {
      return DEFAULT_SYNC_EXCLUSION_PATTERNS.some(p => p.pattern === pattern);
    }
    // Pattern is enabled if it's in enabled list and not in disabled list
    return enabledPatterns.includes(pattern) && !disabledPatterns.includes(pattern);
  };

  const togglePattern = (pattern: string) => {
    if (isPatternEnabled(pattern)) {
      // Disable: add to disabled, remove from enabled
      setDisabledPatterns(prev => [...prev, pattern]);
      setEnabledPatterns(prev => prev.filter(p => p !== pattern));
    } else {
      // Enable: remove from disabled, add to enabled
      setDisabledPatterns(prev => prev.filter(p => p !== pattern));
      if (!enabledPatterns.includes(pattern)) {
        setEnabledPatterns(prev => [...prev, pattern]);
      }
    }
  };

  const addCustomPattern = () => {
    const pattern = newPattern.trim();
    if (!pattern) return;
    if (customPatterns.includes(pattern) || enabledPatterns.includes(pattern)) {
      setError("Pattern already exists");
      return;
    }
    setCustomPatterns(prev => [...prev, pattern]);
    setEnabledPatterns(prev => [...prev, pattern]);
    setNewPattern("");
    setError(null);
  };

  const removeCustomPattern = (pattern: string) => {
    setCustomPatterns(prev => prev.filter(p => p !== pattern));
    setEnabledPatterns(prev => prev.filter(p => p !== pattern));
  };

  const handleSave = async () => {
    setSaving(true);
    setError(null);
    setSaved(false);

    try {
      const syncExclusions: SyncExclusionSettings = {
        enabled_patterns: enabledPatterns,
        disabled_patterns: disabledPatterns,
        custom_patterns: customPatterns,
      };
      await onSave({ sync_exclusions: syncExclusions });
      setSaved(true);
      setTimeout(() => setSaved(false), 2000);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to save exclusions");
    } finally {
      setSaving(false);
    }
  };

  // Count active patterns
  const activeCount = enabledPatterns.filter(p => !disabledPatterns.includes(p)).length +
    customPatterns.filter(p => !disabledPatterns.includes(p)).length;

  // Separate folder and file patterns for display
  const folderPatterns = DEFAULT_SYNC_EXCLUSION_PATTERNS.filter(p => p.category === "folder");
  const filePatterns = DEFAULT_SYNC_EXCLUSION_PATTERNS.filter(p => p.category === "file");

  return (
    <section className="rounded-2xl border-2 border-sercha-silverline bg-white p-6">
      <div className="mb-6">
        <h2 className="text-lg font-semibold text-sercha-ink-slate">
          Sync Exclusions
        </h2>
        <p className="text-sm text-sercha-fog-grey">
          Skip syncing files and folders matching these patterns ({activeCount} active)
        </p>
      </div>

      <div className="space-y-6">
        {error && (
          <div className="flex items-center gap-2 rounded-lg bg-red-50 p-3 text-sm text-red-600">
            <AlertCircle className="h-4 w-4" />
            {error}
          </div>
        )}

        {/* Folder Patterns */}
        <div>
          <h3 className="mb-3 flex items-center gap-2 text-sm font-medium text-sercha-ink-slate">
            <FolderX className="h-4 w-4" />
            Folder Patterns
          </h3>
          <div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
            {folderPatterns.map(({ pattern, description }) => (
              <label
                key={pattern}
                className="flex cursor-pointer items-center gap-3 rounded-lg border border-sercha-silverline bg-sercha-snow p-3 hover:bg-sercha-mist"
              >
                <input
                  type="checkbox"
                  checked={isPatternEnabled(pattern)}
                  onChange={() => togglePattern(pattern)}
                  className="h-4 w-4 rounded border-sercha-silverline text-sercha-indigo focus:ring-sercha-indigo"
                />
                <div className="min-w-0 flex-1">
                  <p className="truncate text-sm font-medium text-sercha-ink-slate">
                    {pattern}
                  </p>
                  <p className="truncate text-xs text-sercha-fog-grey">
                    {description}
                  </p>
                </div>
              </label>
            ))}
          </div>
        </div>

        {/* File Patterns */}
        <div>
          <h3 className="mb-3 flex items-center gap-2 text-sm font-medium text-sercha-ink-slate">
            <FileX className="h-4 w-4" />
            File Patterns
          </h3>
          <div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
            {filePatterns.map(({ pattern, description }) => (
              <label
                key={pattern}
                className="flex cursor-pointer items-center gap-3 rounded-lg border border-sercha-silverline bg-sercha-snow p-3 hover:bg-sercha-mist"
              >
                <input
                  type="checkbox"
                  checked={isPatternEnabled(pattern)}
                  onChange={() => togglePattern(pattern)}
                  className="h-4 w-4 rounded border-sercha-silverline text-sercha-indigo focus:ring-sercha-indigo"
                />
                <div className="min-w-0 flex-1">
                  <p className="truncate text-sm font-medium text-sercha-ink-slate">
                    {pattern}
                  </p>
                  <p className="truncate text-xs text-sercha-fog-grey">
                    {description}
                  </p>
                </div>
              </label>
            ))}
          </div>
        </div>

        {/* Custom Patterns */}
        <div>
          <h3 className="mb-3 flex items-center gap-2 text-sm font-medium text-sercha-ink-slate">
            <Plus className="h-4 w-4" />
            Custom Patterns
          </h3>
          <div className="flex gap-2">
            <input
              type="text"
              value={newPattern}
              onChange={(e) => setNewPattern(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && addCustomPattern()}
              placeholder="e.g., my-folder/ or *.bak"
              className="flex-1 rounded-lg border border-sercha-silverline bg-white px-4 py-2 text-sm text-sercha-ink-slate placeholder:text-sercha-fog-grey focus:border-sercha-indigo focus:outline-none focus:ring-2 focus:ring-sercha-indigo/20"
            />
            <button
              onClick={addCustomPattern}
              className="inline-flex items-center gap-1.5 rounded-lg bg-sercha-indigo px-4 py-2 text-sm font-medium text-white hover:bg-sercha-indigo/90"
            >
              <Plus className="h-4 w-4" />
              Add
            </button>
          </div>
          <p className="mt-1.5 text-xs text-sercha-fog-grey">
            Use &quot;folder/&quot; for folders, &quot;*.ext&quot; for file extensions
          </p>

          {customPatterns.length > 0 && (
            <div className="mt-3 flex flex-wrap gap-2">
              {customPatterns.map((pattern) => (
                <span
                  key={pattern}
                  className="inline-flex items-center gap-1.5 rounded-full bg-sercha-indigo/10 px-3 py-1 text-sm text-sercha-indigo"
                >
                  {pattern}
                  <button
                    onClick={() => removeCustomPattern(pattern)}
                    className="rounded-full p-0.5 hover:bg-sercha-indigo/20"
                  >
                    <X className="h-3.5 w-3.5" />
                  </button>
                </span>
              ))}
            </div>
          )}
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
            {saving ? "Saving..." : saved ? "Saved!" : "Save Exclusions"}
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
      <AdminLayout title="Other Settings" description="Configure search and sync settings">
        <div className="flex items-center justify-center py-12">
          <Loader2 className="h-8 w-8 animate-spin text-sercha-indigo" />
        </div>
      </AdminLayout>
    );
  }

  return (
    <AdminLayout title="Other Settings" description="Configure search and sync settings">
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
                  {aiStatus.schema_upgrade_reason || "Search schema needs upgrade to support embeddings."}
                </p>
                <p className="mt-2 text-sm text-amber-600">
                  After upgrading the search backend, you&apos;ll need to reindex existing documents.
                </p>
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

        {/* Search Settings */}
        <SearchSettingsSection settings={settings} aiSettings={aiSettings} aiStatus={aiStatus} onSave={handleSaveSettings} />

        {/* Sync Configuration */}
        <SyncConfigurationSection settings={settings} onSave={handleSaveSettings} />

        {/* Sync Exclusions */}
        <SyncExclusionsSection settings={settings} onSave={handleSaveSettings} />
      </div>
    </AdminLayout>
  );
}
