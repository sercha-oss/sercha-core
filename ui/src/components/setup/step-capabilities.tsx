"use client";

import { useState, useEffect } from "react";
import { Loader2, Zap, AlertCircle } from "lucide-react";
import {
  getCapabilities,
  getCapabilityPreferences,
  updateCapabilityPreferences,
  type CapabilitiesResponse,
  type CapabilityPreferencesResponse,
  type UpdateCapabilityPreferencesRequest,
} from "@/lib/api";
import { cn } from "@/lib/utils";

interface StepCapabilitiesProps {
  onComplete: () => void;
  onSkip: () => void;
}

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
      className={cn(
        "relative h-6 w-11 rounded-full transition-colors",
        disabled
          ? "cursor-not-allowed bg-sercha-mist"
          : enabled
            ? "bg-sercha-indigo"
            : "bg-sercha-silverline hover:bg-sercha-fog-grey"
      )}
    >
      <span
        className={cn(
          "absolute top-0.5 h-5 w-5 rounded-full bg-white shadow transition-transform",
          enabled ? "left-[22px]" : "left-0.5"
        )}
      />
    </button>
  );
}

// Capability toggle card for setup wizard
function CapabilityToggle({
  title,
  description,
  backend,
  enabled,
  available,
  onChange,
  disabled,
}: {
  title: string;
  description: string;
  backend: string;
  enabled: boolean;
  available: boolean;
  onChange: (enabled: boolean) => void;
  disabled?: boolean;
}) {
  const isDisabled = disabled || !available;

  return (
    <div
      className={cn(
        "flex items-center justify-between rounded-xl border p-4 transition-all",
        isDisabled
          ? "border-sercha-mist bg-sercha-snow opacity-75"
          : "border-sercha-silverline bg-white hover:border-sercha-indigo"
      )}
    >
      <div className="flex-1">
        <div className="flex items-center gap-2">
          <p className="text-sm font-medium text-sercha-ink-slate">{title}</p>
          <span
            className={cn(
              "h-2 w-2 rounded-full",
              available ? "bg-emerald-500" : "bg-red-500"
            )}
            title={available ? "Available" : "Unavailable"}
          />
        </div>
        <p className="mt-0.5 text-xs text-sercha-fog-grey">{description}</p>
        <p className="mt-1 text-xs text-sercha-silverline">Backend: {backend}</p>
      </div>
      <Toggle enabled={enabled} onChange={onChange} disabled={isDisabled} />
    </div>
  );
}

export function StepCapabilities({ onComplete, onSkip }: StepCapabilitiesProps) {
  const [_preferences, setPreferences] = useState<CapabilityPreferencesResponse | null>(null);
  const [capabilities, setCapabilities] = useState<CapabilitiesResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Local state for toggles
  const [textIndexing, setTextIndexing] = useState(true);
  const [embeddingIndexing, setEmbeddingIndexing] = useState(true);

  // Load initial data
  useEffect(() => {
    const loadData = async () => {
      try {
        const [prefsData, capsData] = await Promise.all([
          getCapabilityPreferences().catch(() => null),
          getCapabilities().catch(() => null),
        ]);

        setPreferences(prefsData); // Store for potential future use
        setCapabilities(capsData);

        // Initialize local state from preferences if available
        if (prefsData) {
          setTextIndexing(prefsData.text_indexing_enabled);
          setEmbeddingIndexing(prefsData.embedding_indexing_enabled);
        } else if (capsData) {
          // No saved preferences — default based on actual backend availability
          setTextIndexing(capsData.features?.text_indexing?.available ?? false);
          setEmbeddingIndexing(capsData.features?.embedding_indexing?.available ?? false);
        }
      } catch (err) {
        console.error("Failed to load capability preferences:", err);
      } finally {
        setLoading(false);
      }
    };
    loadData();
  }, []);

  // Check backend availability from capabilities features response
  const isTextIndexingAvailable = (): boolean => {
    return capabilities?.features?.text_indexing?.available ?? false;
  };

  const isEmbeddingIndexingAvailable = (): boolean => {
    return capabilities?.features?.embedding_indexing?.available ?? false;
  };

  const handleSubmit = async () => {
    setIsSubmitting(true);
    setError(null);

    try {
      const updateReq: UpdateCapabilityPreferencesRequest = {
        text_indexing_enabled: textIndexing,
        embedding_indexing_enabled: embeddingIndexing,
        // Search capabilities auto-enabled based on indexing
        bm25_search_enabled: textIndexing,
        vector_search_enabled: embeddingIndexing,
      };

      await updateCapabilityPreferences(updateReq);
      onComplete();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to save capabilities");
    } finally {
      setIsSubmitting(false);
    }
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center py-12">
        <Loader2 className="h-8 w-8 animate-spin text-sercha-indigo" />
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-2xl">
      <div className="mb-8 text-center">
        <h1 className="text-2xl font-semibold text-sercha-ink-slate">
          Configure Capabilities
        </h1>
        <p className="mt-2 text-sm text-sercha-fog-grey">
          Choose which search and indexing capabilities to enable.
          These can be changed later in the admin dashboard.
        </p>
      </div>

      <div className="space-y-6">
        {/* Indexing Section */}
        <div className="rounded-xl border border-sercha-silverline bg-white p-6">
          <div className="mb-4 flex items-center gap-2">
            <Zap className="h-5 w-5 text-sercha-indigo" />
            <h2 className="text-lg font-medium text-sercha-ink-slate">
              Indexing Capabilities
            </h2>
          </div>
          <p className="mb-4 text-sm text-sercha-fog-grey">
            Indexing capabilities determine how your documents are processed and stored.
            Enabling both provides the most comprehensive search experience.
          </p>

          <div className="space-y-3">
            <CapabilityToggle
              title="Text Indexing (BM25)"
              description="Full-text indexing for keyword search. Recommended for all deployments."
              backend="OpenSearch"
              enabled={textIndexing}
              available={isTextIndexingAvailable()}
              onChange={setTextIndexing}
            />

            <CapabilityToggle
              title="Embedding Indexing"
              description={
                isEmbeddingIndexingAvailable()
                  ? "Vector embeddings for semantic search."
                  : "Requires an AI embedding provider (step 2) and pgvector backend."
              }
              backend="pgvector"
              enabled={embeddingIndexing}
              available={isEmbeddingIndexingAvailable()}
              onChange={setEmbeddingIndexing}
            />
          </div>
        </div>

        {/* Search Preview */}
        <div className="rounded-xl border border-sercha-silverline bg-sercha-snow p-4">
          <h3 className="mb-3 text-sm font-medium text-sercha-ink-slate">
            Search Capabilities (Auto-configured)
          </h3>
          <div className="space-y-2 text-sm">
            <div className="flex items-center gap-2">
              <span
                className={cn(
                  "h-2 w-2 rounded-full",
                  textIndexing ? "bg-emerald-500" : "bg-gray-300"
                )}
              />
              <span className={textIndexing ? "text-sercha-ink-slate" : "text-sercha-fog-grey"}>
                BM25 Search {textIndexing ? "(enabled)" : "(disabled)"}
              </span>
            </div>
            <div className="flex items-center gap-2">
              <span
                className={cn(
                  "h-2 w-2 rounded-full",
                  embeddingIndexing ? "bg-emerald-500" : "bg-gray-300"
                )}
              />
              <span className={embeddingIndexing ? "text-sercha-ink-slate" : "text-sercha-fog-grey"}>
                Vector Search {embeddingIndexing ? "(enabled)" : "(disabled)"}
              </span>
            </div>
            <div className="flex items-center gap-2">
              <span
                className={cn(
                  "h-2 w-2 rounded-full",
                  textIndexing && embeddingIndexing ? "bg-emerald-500" : "bg-gray-300"
                )}
              />
              <span
                className={
                  textIndexing && embeddingIndexing
                    ? "text-sercha-ink-slate"
                    : "text-sercha-fog-grey"
                }
              >
                Hybrid Search {textIndexing && embeddingIndexing ? "(enabled)" : "(disabled)"}
              </span>
            </div>
          </div>
          <p className="mt-3 text-xs text-sercha-fog-grey">
            Search capabilities are automatically enabled when their corresponding indexing is enabled.
          </p>
        </div>

        {/* Error Message */}
        {error && (
          <div className="flex items-center gap-2 rounded-lg bg-red-50 p-3 text-sm text-red-600">
            <AlertCircle className="h-4 w-4" />
            {error}
          </div>
        )}

        {/* Action Buttons */}
        <div className="flex gap-3">
          <button
            onClick={onSkip}
            className="flex-1 rounded-lg border border-sercha-silverline bg-white px-4 py-2.5 text-sm font-medium text-sercha-fog-grey transition-colors hover:bg-sercha-mist"
          >
            Skip for now
          </button>
          <button
            onClick={handleSubmit}
            disabled={isSubmitting}
            className="flex flex-1 items-center justify-center rounded-lg bg-sercha-indigo px-4 py-2.5 text-sm font-medium text-white transition-colors hover:bg-sercha-indigo/90 focus:outline-none focus:ring-2 focus:ring-sercha-indigo/50 disabled:cursor-not-allowed disabled:opacity-60"
          >
            {isSubmitting ? (
              <>
                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                Saving...
              </>
            ) : (
              "Save & Continue"
            )}
          </button>
        </div>
      </div>
    </div>
  );
}
