"use client";

import { useState, useEffect } from "react";
import {
  Loader2,
  CheckCircle2,
  XCircle,
  Sparkles,
  ExternalLink,
} from "lucide-react";
import {
  getAISettings,
  getAIProviders,
  getCapabilities,
  updateAISettings,
  testAIConnection,
  ApiError,
  type AISettingsRequest,
  type AIProvider,
  type AIProviderMeta,
  type AIModelMeta,
} from "@/lib/api";

interface StepAIProps {
  onComplete: () => void;
  onSkip: () => void;
}

type TestStatus = "idle" | "testing" | "success" | "error";

export function StepAI({ onComplete, onSkip }: StepAIProps) {
  // Provider metadata from API
  const [embeddingProviders, setEmbeddingProviders] = useState<AIProviderMeta[]>([]);
  const [llmProviders, setLlmProviders] = useState<AIProviderMeta[]>([]);
  const [loadingProviders, setLoadingProviders] = useState(true);

  // Embedding state
  const [embeddingProvider, setEmbeddingProvider] = useState<AIProvider | "">("");
  const [embeddingModel, setEmbeddingModel] = useState("");

  // LLM state
  const [llmProvider, setLlmProvider] = useState<AIProvider | "">("");
  const [llmModel, setLlmModel] = useState("");

  // Status
  const [testStatus, setTestStatus] = useState<TestStatus>("idle");
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Load providers and existing settings
  useEffect(() => {
    const loadData = async () => {
      try {
        const [caps, providersData, existingSettings] = await Promise.all([
          getCapabilities(),
          getAIProviders(),
          getAISettings().catch(() => null), // Ignore if no settings yet
        ]);

        // Filter providers to only show those configured in environment
        const filteredEmbedding = providersData.embedding.filter((p) =>
          caps.ai_providers.embedding.includes(p.id)
        );
        const filteredLLM = providersData.llm.filter((p) =>
          caps.ai_providers.llm.includes(p.id)
        );

        setEmbeddingProviders(filteredEmbedding);
        setLlmProviders(filteredLLM);

        // Pre-populate with existing settings if available
        if (existingSettings?.embedding?.provider) {
          setEmbeddingProvider(existingSettings.embedding.provider);
          setEmbeddingModel(existingSettings.embedding.model || "");
        } else if (filteredEmbedding.length > 0) {
          // Default to first provider and model
          setEmbeddingProvider(filteredEmbedding[0].id);
          if (filteredEmbedding[0].models.length > 0) {
            setEmbeddingModel(filteredEmbedding[0].models[0].id);
          }
        }

        if (existingSettings?.llm?.provider) {
          setLlmProvider(existingSettings.llm.provider);
          setLlmModel(existingSettings.llm.model || "");
        }
      } catch (err) {
        console.error("Failed to load AI providers:", err);
        setError("Failed to load AI providers");
      } finally {
        setLoadingProviders(false);
      }
    };
    loadData();
  }, []);

  // Get selected provider metadata
  const selectedEmbeddingProvider = embeddingProviders.find(
    (p) => p.id === embeddingProvider
  );
  const selectedLlmProvider = llmProviders.find((p) => p.id === llmProvider);

  // Get selected model metadata
  const selectedEmbeddingModel = selectedEmbeddingProvider?.models.find(
    (m) => m.id === embeddingModel
  );

  // Handle provider change
  const handleEmbeddingProviderChange = (providerId: AIProvider | "") => {
    setEmbeddingProvider(providerId);
    if (providerId) {
      const provider = embeddingProviders.find((p) => p.id === providerId);
      if (provider && provider.models.length > 0) {
        setEmbeddingModel(provider.models[0].id);
      }
    } else {
      setEmbeddingModel("");
    }
    setTestStatus("idle");
  };

  const handleLlmProviderChange = (providerId: AIProvider | "") => {
    setLlmProvider(providerId);
    if (providerId) {
      const provider = llmProviders.find((p) => p.id === providerId);
      if (provider && provider.models.length > 0) {
        setLlmModel(provider.models[0].id);
      }
    } else {
      setLlmModel("");
    }
  };

  const handleTest = async () => {
    setTestStatus("testing");
    setError(null);

    try {
      // First save the settings, then test
      const settings: AISettingsRequest = {};
      if (embeddingProvider) {
        settings.embedding = {
          provider: embeddingProvider as AIProvider,
          model: embeddingModel,
        };
      }
      if (llmProvider) {
        settings.llm = {
          provider: llmProvider as AIProvider,
          model: llmModel,
        };
      }

      // Save first so test has something to test
      await updateAISettings(settings);
      await testAIConnection();
      setTestStatus("success");
    } catch (err) {
      setTestStatus("error");
      if (err instanceof ApiError) {
        setError(err.message || "Connection test failed");
      } else {
        setError("Failed to test connection");
      }
    }
  };

  const handleSubmit = async () => {
    setIsSubmitting(true);
    setError(null);

    try {
      const settings: AISettingsRequest = {};

      // Add embedding config if provider selected
      if (embeddingProvider) {
        settings.embedding = {
          provider: embeddingProvider as AIProvider,
          model: embeddingModel,
        };
      }

      // Add LLM config if provider selected
      if (llmProvider) {
        settings.llm = {
          provider: llmProvider as AIProvider,
          model: llmModel,
        };
      }

      await updateAISettings(settings);
      onComplete();
    } catch (err) {
      if (err instanceof ApiError) {
        setError(err.message || "Failed to save settings");
      } else {
        setError("Failed to save settings");
      }
    } finally {
      setIsSubmitting(false);
    }
  };

  // Check if we can test (need at least embedding or LLM configured)
  const canTest =
    (embeddingProvider && embeddingModel) ||
    (llmProvider && llmModel);

  if (loadingProviders) {
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
          Configure AI
        </h1>
        <p className="mt-2 text-sm text-sercha-fog-grey">
          Enable semantic search with embeddings and AI-powered features with an
          LLM. You can skip this step and configure later.
        </p>
      </div>

      <div className="space-y-8">
        {/* Embedding Configuration */}
        <div className="rounded-xl border border-sercha-silverline bg-white p-6">
          <div className="mb-4 flex items-center gap-2">
            <Sparkles className="h-5 w-5 text-sercha-indigo" />
            <h2 className="text-lg font-medium text-sercha-ink-slate">
              Embedding Model
            </h2>
            <span className="rounded-full bg-sercha-indigo/10 px-2 py-0.5 text-xs text-sercha-indigo">
              Recommended
            </span>
          </div>
          <p className="mb-4 text-sm text-sercha-fog-grey">
            Embeddings enable semantic search, finding results based on meaning
            rather than just keywords.
          </p>

          <div className="grid gap-4 md:grid-cols-2">
            {/* Provider */}
            <div>
              <label className="mb-1.5 block text-sm font-medium text-sercha-ink-slate">
                Provider
              </label>
              <select
                value={embeddingProvider}
                onChange={(e) =>
                  handleEmbeddingProviderChange(e.target.value as AIProvider | "")
                }
                className="w-full rounded-lg border border-sercha-silverline bg-white px-4 py-2.5 text-sm text-sercha-ink-slate focus:border-sercha-indigo focus:outline-none focus:ring-2 focus:ring-sercha-indigo/20"
              >
                <option value="">None</option>
                {embeddingProviders.map((p) => (
                  <option key={p.id} value={p.id}>
                    {p.name}
                  </option>
                ))}
              </select>
              {selectedEmbeddingProvider?.api_key_url && (
                <a
                  href={selectedEmbeddingProvider.api_key_url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="mt-1.5 inline-flex items-center gap-1 text-xs text-sercha-indigo hover:underline"
                >
                  Get your API key
                  <ExternalLink className="h-3 w-3" />
                </a>
              )}
            </div>

            {/* Model */}
            {embeddingProvider && (
              <div>
                <label className="mb-1.5 block text-sm font-medium text-sercha-ink-slate">
                  Model
                </label>
                <select
                  value={embeddingModel}
                  onChange={(e) => setEmbeddingModel(e.target.value)}
                  className="w-full rounded-lg border border-sercha-silverline bg-white px-4 py-2.5 text-sm text-sercha-ink-slate focus:border-sercha-indigo focus:outline-none focus:ring-2 focus:ring-sercha-indigo/20"
                >
                  {selectedEmbeddingProvider?.models.map((m: AIModelMeta) => (
                    <option key={m.id} value={m.id}>
                      {m.name}
                      {m.dimensions ? ` (${m.dimensions}d)` : ""}
                    </option>
                  ))}
                </select>
                {selectedEmbeddingModel?.dimensions && (
                  <p className="mt-1 text-xs text-sercha-fog-grey">
                    Vector dimensions: {selectedEmbeddingModel.dimensions}
                  </p>
                )}
              </div>
            )}
          </div>

          {/* Info Note */}
          {embeddingProvider && (
            <p className="mt-3 text-xs text-sercha-fog-grey">
              API keys are configured via environment variables on the server.
            </p>
          )}
        </div>

        {/* LLM Configuration */}
        <div className="rounded-xl border border-sercha-silverline bg-white p-6">
          <div className="mb-4 flex items-center gap-2">
            <Sparkles className="h-5 w-5 text-sercha-fog-grey" />
            <h2 className="text-lg font-medium text-sercha-ink-slate">
              LLM Model
            </h2>
            <span className="rounded-full bg-sercha-mist px-2 py-0.5 text-xs text-sercha-fog-grey">
              Optional
            </span>
          </div>
          <p className="mb-4 text-sm text-sercha-fog-grey">
            An LLM enables AI-powered features like answer generation and
            document summarization.
          </p>

          <div className="grid gap-4 md:grid-cols-2">
            {/* Provider */}
            <div>
              <label className="mb-1.5 block text-sm font-medium text-sercha-ink-slate">
                Provider
              </label>
              <select
                value={llmProvider}
                onChange={(e) =>
                  handleLlmProviderChange(e.target.value as AIProvider | "")
                }
                className="w-full rounded-lg border border-sercha-silverline bg-white px-4 py-2.5 text-sm text-sercha-ink-slate focus:border-sercha-indigo focus:outline-none focus:ring-2 focus:ring-sercha-indigo/20"
              >
                <option value="">None</option>
                {llmProviders.map((p) => (
                  <option key={p.id} value={p.id}>
                    {p.name}
                  </option>
                ))}
              </select>
              {selectedLlmProvider?.api_key_url && (
                <a
                  href={selectedLlmProvider.api_key_url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="mt-1.5 inline-flex items-center gap-1 text-xs text-sercha-indigo hover:underline"
                >
                  Get your API key
                  <ExternalLink className="h-3 w-3" />
                </a>
              )}
            </div>

            {/* Model */}
            {llmProvider && (
              <div>
                <label className="mb-1.5 block text-sm font-medium text-sercha-ink-slate">
                  Model
                </label>
                <select
                  value={llmModel}
                  onChange={(e) => setLlmModel(e.target.value)}
                  className="w-full rounded-lg border border-sercha-silverline bg-white px-4 py-2.5 text-sm text-sercha-ink-slate focus:border-sercha-indigo focus:outline-none focus:ring-2 focus:ring-sercha-indigo/20"
                >
                  {selectedLlmProvider?.models.map((m: AIModelMeta) => (
                    <option key={m.id} value={m.id}>
                      {m.name}
                    </option>
                  ))}
                </select>
              </div>
            )}
          </div>

          {/* Info Note */}
          {llmProvider && (
            <p className="mt-3 text-xs text-sercha-fog-grey">
              API keys are configured via environment variables on the server.
            </p>
          )}
        </div>

        {/* Test Status */}
        {testStatus !== "idle" && (
          <div
            className={`flex items-center gap-3 rounded-lg p-4 ${
              testStatus === "testing"
                ? "border border-sercha-silverline bg-sercha-mist"
                : testStatus === "success"
                  ? "border border-emerald-200 bg-emerald-50"
                  : "border border-red-200 bg-red-50"
            }`}
          >
            {testStatus === "testing" && (
              <>
                <Loader2 className="h-5 w-5 animate-spin text-sercha-indigo" />
                <span className="text-sm text-sercha-ink-slate">
                  Testing connection...
                </span>
              </>
            )}
            {testStatus === "success" && (
              <>
                <CheckCircle2 className="h-5 w-5 text-emerald-600" />
                <span className="text-sm font-medium text-emerald-700">
                  Connection successful
                </span>
              </>
            )}
            {testStatus === "error" && (
              <>
                <XCircle className="h-5 w-5 text-red-600" />
                <span className="text-sm text-red-700">{error}</span>
              </>
            )}
          </div>
        )}

        {/* Error Message */}
        {error && testStatus !== "error" && (
          <div className="rounded-lg bg-red-50 px-4 py-3 text-sm text-red-600">
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
            onClick={handleTest}
            disabled={testStatus === "testing" || !canTest}
            className="rounded-lg border border-sercha-silverline bg-white px-4 py-2.5 text-sm font-medium text-sercha-ink-slate transition-colors hover:bg-sercha-mist disabled:cursor-not-allowed disabled:opacity-50"
          >
            Test Connection
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
