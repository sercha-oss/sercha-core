"use client";

import { useState, useEffect, useCallback, useMemo } from "react";
import { AdminLayout } from "@/components/layout";
import { CapabilityCard } from "@/components/capabilities/capability-card";
import { Loader2, AlertCircle, RefreshCw, Check } from "lucide-react";
import {
  getCapabilities,
  getCapabilityPreferences,
  updateCapabilityPreferences,
  type CapabilitiesResponse,
  type CapabilityDescriptor,
  type CapabilityPreferencesResponse,
} from "@/lib/api";

// CapabilitiesPage renders one card per registered descriptor returned by
// the backend. The backend's CapabilityRegistry is the source of truth —
// no hardcoded list lives here; new capabilities (Core or add-on) appear
// automatically once registered server-side.
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

  // Resolve toggle + availability for a capability. Capabilities absent
  // from the persisted toggles fall back to the descriptor's
  // default_enabled (matching the backend's resolution logic).
  const isEnabled = (d: CapabilityDescriptor): boolean => {
    if (!preferences) return d.default_enabled;
    const v = preferences.toggles[d.type];
    return v !== undefined ? v : d.default_enabled;
  };
  const isAvailable = (type: string): boolean =>
    capabilities?.features?.[type]?.available ?? false;

  const handleToggle = async (type: string, enabled: boolean) => {
    setSaving(type);
    setError(null);
    try {
      const updated = await updateCapabilityPreferences({ toggles: { [type]: enabled } });
      setPreferences(updated);
      setSavedKey(type);
      setTimeout(() => setSavedKey(null), 1500);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to update capability");
    } finally {
      setSaving(null);
    }
  };

  // Build the descriptor lookup once and group descriptors by phase +
  // backend for the UI sections. Falling back to backend_id="" puts
  // backend-less capabilities in the Search bucket — vanishingly rare in
  // practice but the UI shouldn't drop them silently.
  const grouped = useMemo(() => {
    const descriptors = capabilities?.descriptors ?? [];
    const indexing = descriptors.filter((d) => d.phase === "indexing");
    const llm = descriptors.filter((d) => d.phase === "search" && d.backend_id === "llm");
    const search = descriptors.filter((d) => d.phase === "search" && d.backend_id !== "llm");
    return { indexing, search, llm };
  }, [capabilities]);

  const descriptorsByType = useMemo(() => {
    const m = new Map<string, CapabilityDescriptor>();
    for (const d of capabilities?.descriptors ?? []) m.set(d.type, d);
    return m;
  }, [capabilities]);

  const renderCard = (d: CapabilityDescriptor) => {
    const enabled = isEnabled(d);
    const available = isAvailable(d.type);
    const isSavingThis = saving === d.type;
    const justSaved = savedKey === d.type;

    // Surface the first dependency in the UI as the "depends on" label,
    // mirroring the previous behaviour. Multi-dep descriptors still
    // cascade-disable correctly server-side.
    const firstDep = d.depends_on?.[0];
    const depDescriptor = firstDep ? descriptorsByType.get(firstDep) : undefined;
    const dependencyMet = firstDep ? isEnabled(descriptorsByType.get(firstDep) ?? d) : true;

    return (
      <div key={d.type} className="relative">
        <CapabilityCard
          name={d.display_name}
          description={d.description}
          backend={d.backend_id ?? ""}
          phase={d.phase}
          available={available}
          enabled={enabled}
          dependsOn={depDescriptor?.display_name}
          dependencyMet={dependencyMet}
          onToggle={(en) => handleToggle(d.type, en)}
          disabled={isSavingThis}
        />
        {(isSavingThis || justSaved) && (
          <div className="absolute right-2 top-2">
            {isSavingThis ? (
              <Loader2 className="h-4 w-4 animate-spin text-sercha-indigo" />
            ) : (
              <Check className="h-4 w-4 text-emerald-500" />
            )}
          </div>
        )}
      </div>
    );
  };

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
        <div className="flex items-center justify-between">
          <p className="text-sm text-sercha-fog-grey">
            Configure which capabilities are enabled for your team. Disabling
            a capability whose dependents rely on it will cascade-disable them.
          </p>
          <button
            onClick={fetchData}
            disabled={loading}
            className="flex items-center gap-2 rounded-lg border border-sercha-silverline px-3 py-2 text-sm text-sercha-fog-grey hover:bg-sercha-mist disabled:opacity-50"
          >
            <RefreshCw className={`h-4 w-4 ${loading ? "animate-spin" : ""}`} />
            Refresh
          </button>
        </div>

        {error && (
          <div className="flex items-center gap-2 rounded-lg bg-red-50 p-4 text-red-600">
            <AlertCircle className="h-5 w-5" />
            {error}
          </div>
        )}

        {/* Backend availability badges */}
        {capabilities?.descriptors && capabilities.descriptors.length > 0 && (
          <div className="rounded-xl border border-sercha-silverline bg-white p-4">
            <h3 className="mb-3 text-sm font-medium text-sercha-ink-slate">Backend Availability</h3>
            <div className="flex flex-wrap gap-3">
              {capabilities.descriptors.map((d) => {
                const available = isAvailable(d.type);
                return (
                  <span
                    key={d.type}
                    className={`inline-flex items-center gap-1.5 rounded-full px-3 py-1 text-xs font-medium ${
                      available ? "bg-emerald-100 text-emerald-700" : "bg-gray-100 text-gray-500"
                    }`}
                  >
                    <span
                      className={`h-1.5 w-1.5 rounded-full ${
                        available ? "bg-emerald-500" : "bg-gray-400"
                      }`}
                    />
                    {d.display_name}
                  </span>
                );
              })}
            </div>
          </div>
        )}

        {grouped.indexing.length > 0 && (
          <section>
            <h2 className="mb-4 text-lg font-semibold text-sercha-ink-slate">
              Indexing Capabilities
            </h2>
            <div className="grid gap-4 sm:grid-cols-2">{grouped.indexing.map(renderCard)}</div>
          </section>
        )}

        {grouped.search.length > 0 && (
          <section>
            <h2 className="mb-4 text-lg font-semibold text-sercha-ink-slate">
              Search Capabilities
            </h2>
            <div className="grid gap-4 sm:grid-cols-2">{grouped.search.map(renderCard)}</div>
          </section>
        )}

        {grouped.llm.length > 0 && (
          <section>
            <h2 className="mb-4 text-lg font-semibold text-sercha-ink-slate">
              LLM Enhancements
            </h2>
            <p className="mb-4 text-sm text-sercha-fog-grey">
              AI-powered features that enhance search quality. Requires an LLM provider.
            </p>
            <div className="grid gap-4 sm:grid-cols-2">{grouped.llm.map(renderCard)}</div>
          </section>
        )}

        <div className="rounded-xl border border-sercha-indigo/20 bg-sercha-indigo/5 p-4">
          <h3 className="mb-2 text-sm font-medium text-sercha-ink-slate">About Capabilities</h3>
          <ul className="space-y-1 text-sm text-sercha-fog-grey">
            <li>
              <strong>Indexing capabilities</strong> control how documents are processed and stored.
            </li>
            <li>
              <strong>Search capabilities</strong> control which retrieval methods are available.
            </li>
            <li>
              <strong>LLM enhancements</strong> use AI to improve search quality.
            </li>
            <li>
              Capabilities with unmet dependencies are disabled automatically server-side.
            </li>
            <li>
              Changes take effect immediately. Existing indexes are preserved.
            </li>
          </ul>
        </div>
      </div>
    </AdminLayout>
  );
}
