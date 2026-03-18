"use client";

import { useState, useEffect } from "react";
import Image from "next/image";
import { AdminLayout } from "@/components/layout";
import {
  Loader2,
  AlertCircle,
  Trash2,
  ChevronDown,
  ChevronUp,
  Pencil,
  ExternalLink,
  X,
  Plus,
} from "lucide-react";
import {
  listInstallations,
  deleteInstallation,
  getInstallationSources,
  listProviders,
  saveProviderConfig,
  deleteProviderConfig,
  InstallationSummary,
  InstallationSourceSummary,
  ProviderListItem,
} from "@/lib/api";
import { getProviderIcon, getProviderHelpUrl } from "@/lib/providers";

// Provider Credentials Section
function ProviderCredentialsSection() {
  const [providers, setProviders] = useState<ProviderListItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [editingProvider, setEditingProvider] = useState<ProviderListItem | null>(null);
  const [clientId, setClientId] = useState("");
  const [clientSecret, setClientSecret] = useState("");
  const [saving, setSaving] = useState(false);
  const [confirmDeleteType, setConfirmDeleteType] = useState<string | null>(null);
  const [deleting, setDeleting] = useState<string | null>(null);

  useEffect(() => {
    loadProviders();
  }, []);

  const loadProviders = async () => {
    try {
      setLoading(true);
      setError(null);
      const data = await listProviders();
      setProviders(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load providers");
    } finally {
      setLoading(false);
    }
  };

  const handleEdit = (provider: ProviderListItem) => {
    setEditingProvider(provider);
    setClientId("");
    setClientSecret("");
  };

  const handleSave = async () => {
    if (!editingProvider || !clientId || !clientSecret) return;

    setSaving(true);
    try {
      await saveProviderConfig(editingProvider.type, {
        client_id: clientId,
        client_secret: clientSecret,
      });
      await loadProviders();
      setEditingProvider(null);
      setClientId("");
      setClientSecret("");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to save credentials");
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async (type: string) => {
    if (confirmDeleteType !== type) {
      setConfirmDeleteType(type);
      return;
    }

    setDeleting(type);
    try {
      await deleteProviderConfig(type);
      await loadProviders();
      setConfirmDeleteType(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to delete credentials");
    } finally {
      setDeleting(null);
    }
  };

  if (loading) {
    return (
      <section className="rounded-2xl border-2 border-sercha-silverline bg-white p-6">
        <h2 className="mb-4 text-lg font-semibold text-sercha-ink-slate">Provider Credentials</h2>
        <div className="flex items-center justify-center py-8">
          <Loader2 className="h-6 w-6 animate-spin text-sercha-indigo" />
        </div>
      </section>
    );
  }

  return (
    <section className="rounded-2xl border-2 border-sercha-silverline bg-white p-6">
      <h2 className="mb-1 text-lg font-semibold text-sercha-ink-slate">Provider Credentials</h2>
      <p className="mb-4 text-sm text-sercha-fog-grey">
        Configure OAuth app credentials (Client ID & Secret) for each provider.
      </p>

      {error && (
        <div className="mb-4 flex items-center gap-2 rounded-lg bg-red-50 p-3 text-sm text-red-600">
          <AlertCircle className="h-4 w-4" />
          {error}
          <button onClick={() => setError(null)} className="ml-auto">
            <X className="h-4 w-4" />
          </button>
        </div>
      )}

      {providers.filter(p => p.configured).length === 0 ? (
        <div className="rounded-lg border border-dashed border-sercha-silverline bg-sercha-snow p-6 text-center">
          <p className="text-sm text-sercha-fog-grey">No provider credentials configured.</p>
          <p className="mt-1 text-xs text-sercha-fog-grey">
            Configure providers when adding a new data source.
          </p>
        </div>
      ) : (
      <div className="space-y-3">
        {providers.filter(p => p.configured).map((provider) => (
          <div
            key={provider.type}
            className="rounded-lg border border-sercha-silverline bg-sercha-snow"
          >
            <div className="flex items-center justify-between p-4">
              <div className="flex items-center gap-3">
                <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-white border border-sercha-silverline overflow-hidden">
                  <Image
                    src={getProviderIcon(provider.type)}
                    alt={provider.name}
                    width={24}
                    height={24}
                    className="object-contain"
                  />
                </div>
                <div>
                  <div className="font-medium text-sercha-ink-slate">{provider.name}</div>
                  <div className="text-sm text-sercha-fog-grey">{provider.description}</div>
                </div>
              </div>
              <div className="flex items-center gap-2">
                {provider.configured ? (
                  <>
                    <span className="rounded-full bg-green-100 px-2 py-1 text-xs font-medium text-green-700">
                      Configured
                    </span>
                    <button
                      onClick={() => handleEdit(provider)}
                      className="rounded-lg p-2 text-sercha-fog-grey hover:bg-sercha-mist hover:text-sercha-ink-slate"
                      title="Edit credentials"
                    >
                      <Pencil className="h-4 w-4" />
                    </button>
                    <button
                      onClick={() => handleDelete(provider.type)}
                      disabled={deleting === provider.type}
                      className={`rounded-lg p-2 ${
                        confirmDeleteType === provider.type
                          ? "bg-red-100 text-red-600"
                          : "text-sercha-fog-grey hover:bg-red-50 hover:text-red-600"
                      }`}
                      title="Delete credentials"
                    >
                      {deleting === provider.type ? (
                        <Loader2 className="h-4 w-4 animate-spin" />
                      ) : (
                        <Trash2 className="h-4 w-4" />
                      )}
                    </button>
                  </>
                ) : (
                  <>
                    <span className="rounded-full bg-sercha-mist px-2 py-1 text-xs text-sercha-fog-grey">
                      Not configured
                    </span>
                    <button
                      onClick={() => handleEdit(provider)}
                      className="flex items-center gap-1 rounded-lg bg-sercha-indigo px-3 py-1.5 text-sm font-medium text-white hover:bg-sercha-indigo/90"
                    >
                      <Plus className="h-3 w-3" />
                      Configure
                    </button>
                  </>
                )}
              </div>
            </div>

            {/* Delete Confirmation */}
            {confirmDeleteType === provider.type && (
              <div className="border-t border-red-200 bg-red-50 px-4 py-3">
                <p className="text-sm text-red-700">
                  Delete {provider.name} credentials? Existing installations will stop working.
                </p>
                <div className="mt-2 flex gap-2">
                  <button
                    onClick={() => setConfirmDeleteType(null)}
                    className="rounded-lg border border-sercha-silverline bg-white px-3 py-1.5 text-sm text-sercha-ink-slate hover:bg-sercha-mist"
                  >
                    Cancel
                  </button>
                  <button
                    onClick={() => handleDelete(provider.type)}
                    disabled={deleting === provider.type}
                    className="rounded-lg bg-red-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-red-700 disabled:opacity-50"
                  >
                    {deleting === provider.type ? "Deleting..." : "Delete Credentials"}
                  </button>
                </div>
              </div>
            )}
          </div>
        ))}
      </div>
      )}

      {/* Edit/Configure Modal */}
      {editingProvider && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
          <div className="mx-4 w-full max-w-md rounded-2xl bg-white p-6 shadow-xl">
            <div className="mb-4 flex items-center justify-between">
              <div className="flex items-center gap-3">
                <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-sercha-snow border border-sercha-silverline overflow-hidden">
                  <Image
                    src={getProviderIcon(editingProvider.type)}
                    alt={editingProvider.name}
                    width={24}
                    height={24}
                    className="object-contain"
                  />
                </div>
                <div>
                  <h3 className="font-semibold text-sercha-ink-slate">
                    {editingProvider.configured ? "Edit" : "Configure"} {editingProvider.name}
                  </h3>
                  <p className="text-sm text-sercha-fog-grey">Enter your OAuth app credentials</p>
                </div>
              </div>
              <button
                onClick={() => setEditingProvider(null)}
                className="rounded-lg p-2 text-sercha-fog-grey hover:bg-sercha-mist"
              >
                <X className="h-5 w-5" />
              </button>
            </div>

            <div className="space-y-4">
              <div>
                <label className="mb-1 block text-sm font-medium text-sercha-ink-slate">
                  Client ID
                </label>
                <input
                  type="text"
                  value={clientId}
                  onChange={(e) => setClientId(e.target.value)}
                  placeholder="Enter client ID"
                  className="w-full rounded-lg border border-sercha-silverline px-3 py-2 text-sm focus:border-sercha-indigo focus:outline-none focus:ring-1 focus:ring-sercha-indigo"
                />
              </div>
              <div>
                <label className="mb-1 block text-sm font-medium text-sercha-ink-slate">
                  Client Secret
                </label>
                <input
                  type="password"
                  value={clientSecret}
                  onChange={(e) => setClientSecret(e.target.value)}
                  placeholder="Enter client secret"
                  className="w-full rounded-lg border border-sercha-silverline px-3 py-2 text-sm focus:border-sercha-indigo focus:outline-none focus:ring-1 focus:ring-sercha-indigo"
                />
              </div>

              {getProviderHelpUrl(editingProvider.type) && (
                <a
                  href={getProviderHelpUrl(editingProvider.type)}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="flex items-center gap-1 text-sm text-sercha-indigo hover:underline"
                >
                  <ExternalLink className="h-3 w-3" />
                  Get credentials from {editingProvider.name}
                </a>
              )}
            </div>

            <div className="mt-6 flex justify-end gap-2">
              <button
                onClick={() => setEditingProvider(null)}
                className="rounded-lg border border-sercha-silverline px-4 py-2 text-sm font-medium text-sercha-ink-slate hover:bg-sercha-mist"
              >
                Cancel
              </button>
              <button
                onClick={handleSave}
                disabled={saving || !clientId || !clientSecret}
                className="rounded-lg bg-sercha-indigo px-4 py-2 text-sm font-medium text-white hover:bg-sercha-indigo/90 disabled:opacity-50"
              >
                {saving ? (
                  <span className="flex items-center gap-2">
                    <Loader2 className="h-4 w-4 animate-spin" />
                    Saving...
                  </span>
                ) : (
                  "Save Credentials"
                )}
              </button>
            </div>
          </div>
        </div>
      )}
    </section>
  );
}

// OAuth Installations Section
function OAuthInstallationsSection() {
  const [installations, setInstallations] = useState<InstallationSummary[]>([]);
  const [expandedId, setExpandedId] = useState<string | null>(null);
  const [sources, setSources] = useState<Record<string, InstallationSourceSummary[]>>({});
  const [confirmDeleteId, setConfirmDeleteId] = useState<string | null>(null);
  const [deleting, setDeleting] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    loadInstallations();
  }, []);

  const loadInstallations = async () => {
    try {
      setLoading(true);
      setError(null);
      const data = await listInstallations();
      setInstallations(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load installations");
    } finally {
      setLoading(false);
    }
  };

  const toggleExpand = async (id: string) => {
    if (expandedId === id) {
      setExpandedId(null);
    } else {
      setExpandedId(id);
      // Fetch sources if not already loaded
      if (!sources[id]) {
        try {
          const data = await getInstallationSources(id);
          setSources((prev) => ({ ...prev, [id]: data }));
        } catch (err) {
          console.error("Failed to load sources:", err);
        }
      }
    }
  };

  const handleDelete = async (id: string) => {
    if (confirmDeleteId !== id) {
      setConfirmDeleteId(id);
      return;
    }

    setDeleting(id);
    try {
      await deleteInstallation(id);
      setInstallations((prev) => prev.filter((i) => i.id !== id));
      setConfirmDeleteId(null);
    } catch (err) {
      console.error("Failed to delete installation:", err);
    } finally {
      setDeleting(null);
    }
  };

  const getProviderIcon = (providerType: string) => {
    // Simple provider icon based on type
    const icons: Record<string, string> = {
      google_drive: "G",
      onedrive: "O",
      github: "GH",
      gitlab: "GL",
      notion: "N",
      dropbox: "D",
    };
    return icons[providerType] || providerType.charAt(0).toUpperCase();
  };

  const getProviderName = (providerType: string) => {
    const names: Record<string, string> = {
      google_drive: "Google Drive",
      onedrive: "OneDrive",
      github: "GitHub",
      gitlab: "GitLab",
      notion: "Notion",
      dropbox: "Dropbox",
    };
    return names[providerType] || providerType;
  };

  if (loading) {
    return (
      <section className="rounded-2xl border-2 border-sercha-silverline bg-white p-6">
        <h2 className="mb-4 text-lg font-semibold text-sercha-ink-slate">OAuth Installations</h2>
        <div className="flex items-center justify-center py-8">
          <Loader2 className="h-6 w-6 animate-spin text-sercha-indigo" />
        </div>
      </section>
    );
  }

  return (
    <section className="rounded-2xl border-2 border-sercha-silverline bg-white p-6">
      <h2 className="mb-1 text-lg font-semibold text-sercha-ink-slate">OAuth Installations</h2>
      <p className="mb-4 text-sm text-sercha-fog-grey">
        Manage connected accounts. Deleting an installation will also delete all associated sources and documents.
      </p>

      {error && (
        <div className="mb-4 flex items-center gap-2 rounded-lg bg-red-50 p-3 text-sm text-red-600">
          <AlertCircle className="h-4 w-4" />
          {error}
        </div>
      )}

      {installations.length === 0 ? (
        <div className="rounded-lg border border-dashed border-sercha-silverline bg-sercha-snow p-6 text-center">
          <p className="text-sm text-sercha-fog-grey">No OAuth installations configured.</p>
          <p className="mt-1 text-xs text-sercha-fog-grey">
            Connect a data source to see installations here.
          </p>
        </div>
      ) : (
        <div className="space-y-3">
          {installations.map((inst) => (
            <div
              key={inst.id}
              className="rounded-lg border border-sercha-silverline bg-sercha-snow"
            >
              <div className="flex items-center justify-between p-4">
                <div className="flex items-center gap-3">
                  <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-sercha-indigo text-sm font-bold text-white">
                    {getProviderIcon(inst.provider_type)}
                  </div>
                  <div>
                    <div className="font-medium text-sercha-ink-slate">
                      {getProviderName(inst.provider_type)}
                    </div>
                    <div className="text-sm text-sercha-fog-grey">
                      {inst.account_id || inst.name}
                    </div>
                  </div>
                </div>
                <div className="flex items-center gap-2">
                  <span className="rounded-full bg-sercha-mist px-2 py-1 text-xs text-sercha-fog-grey">
                    {inst.source_count} source{inst.source_count !== 1 ? "s" : ""}
                  </span>
                  <button
                    onClick={() => toggleExpand(inst.id)}
                    className="rounded-lg p-2 text-sercha-fog-grey hover:bg-sercha-mist hover:text-sercha-ink-slate"
                  >
                    {expandedId === inst.id ? (
                      <ChevronUp className="h-4 w-4" />
                    ) : (
                      <ChevronDown className="h-4 w-4" />
                    )}
                  </button>
                  <button
                    onClick={() => handleDelete(inst.id)}
                    disabled={deleting === inst.id}
                    className={`rounded-lg p-2 ${
                      confirmDeleteId === inst.id
                        ? "bg-red-100 text-red-600"
                        : "text-sercha-fog-grey hover:bg-red-50 hover:text-red-600"
                    }`}
                  >
                    {deleting === inst.id ? (
                      <Loader2 className="h-4 w-4 animate-spin" />
                    ) : (
                      <Trash2 className="h-4 w-4" />
                    )}
                  </button>
                </div>
              </div>

              {/* Expanded Sources List */}
              {expandedId === inst.id && (
                <div className="border-t border-sercha-silverline bg-white px-4 py-3">
                  <p className="mb-2 text-xs font-medium text-sercha-fog-grey">Connected Sources:</p>
                  {sources[inst.id]?.length ? (
                    <div className="space-y-2">
                      {sources[inst.id].map((src) => (
                        <div
                          key={src.source.id}
                          className="flex items-center justify-between rounded-lg bg-sercha-snow px-3 py-2 text-sm"
                        >
                          <span className="text-sercha-ink-slate">{src.source.name}</span>
                          <span className="text-xs text-sercha-fog-grey">
                            {src.document_count} doc{src.document_count !== 1 ? "s" : ""}
                          </span>
                        </div>
                      ))}
                    </div>
                  ) : (
                    <p className="text-sm text-sercha-fog-grey">No sources connected.</p>
                  )}
                </div>
              )}

              {/* Delete Confirmation */}
              {confirmDeleteId === inst.id && (
                <div className="border-t border-red-200 bg-red-50 px-4 py-3">
                  <p className="text-sm text-red-700">
                    This will delete {inst.source_count} source{inst.source_count !== 1 ? "s" : ""} and all associated documents.
                  </p>
                  <div className="mt-2 flex gap-2">
                    <button
                      onClick={() => setConfirmDeleteId(null)}
                      className="rounded-lg border border-sercha-silverline bg-white px-3 py-1.5 text-sm text-sercha-ink-slate hover:bg-sercha-mist"
                    >
                      Cancel
                    </button>
                    <button
                      onClick={() => handleDelete(inst.id)}
                      disabled={deleting === inst.id}
                      className="rounded-lg bg-red-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-red-700 disabled:opacity-50"
                    >
                      {deleting === inst.id ? "Deleting..." : "Delete Installation"}
                    </button>
                  </div>
                </div>
              )}
            </div>
          ))}
        </div>
      )}
    </section>
  );
}

// Main OAuth Settings Page
export default function OAuthSettingsPage() {
  return (
    <AdminLayout title="OAuth Settings" description="Manage provider credentials and installations">
      <div className="space-y-6">
        <ProviderCredentialsSection />
        <OAuthInstallationsSection />
      </div>
    </AdminLayout>
  );
}
