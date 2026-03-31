"use client";

import { useState, useEffect, useCallback } from "react";
import { AdminLayout } from "@/components/layout";
import {
  Loader2,
  AlertCircle,
  Check,
  RefreshCw,
  Zap,
  Settings2,
  Trash2,
} from "lucide-react";
import {
  getAISettings,
  updateAISettings,
  testAIConnection,
  getAIProviders,
  getCapabilities,
  deleteEmbeddingConfig,
  deleteLLMConfig,
  type AISettingsResponse,
  type AIProviderConfig,
  type AIProvidersResponse,
} from "@/lib/api";
import { AIConfigWizard } from "@/components/settings/ai-config-wizard";

// AI Provider Display Card
function AIProviderDisplay({
  title,
  description,
  isConfigured,
  provider,
  model,
  onConfigure,
  onRemove,
  removing,
}: {
  title: string;
  description: string;
  isConfigured: boolean;
  provider?: string;
  model?: string;
  onConfigure: () => void;
  onRemove?: () => void;
  removing?: boolean;
}) {
  const [showConfirm, setShowConfirm] = useState(false);

  const handleRemove = () => {
    if (showConfirm) {
      onRemove?.();
      setShowConfirm(false);
    } else {
      setShowConfirm(true);
    }
  };

  return (
    <div className="rounded-xl border border-sercha-silverline bg-sercha-snow p-4">
      <div className="flex items-start justify-between">
        <div className="flex-1">
          <h4 className="text-sm font-semibold text-sercha-ink-slate">{title}</h4>
          <p className="mt-0.5 text-xs text-sercha-fog-grey">{description}</p>

          {isConfigured ? (
            <div className="mt-3 flex items-center gap-2">
              <span className="inline-flex items-center gap-1 rounded-full bg-emerald-100 px-2 py-0.5 text-xs font-medium text-emerald-700">
                <Check className="h-3 w-3" />
                Configured
              </span>
              <span className="text-sm text-sercha-ink-slate">
                {provider} / {model}
              </span>
            </div>
          ) : (
            <div className="mt-3">
              <span className="rounded-full bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-500">
                Not configured
              </span>
            </div>
          )}
        </div>

        <div className="flex gap-2">
          <button
            onClick={onConfigure}
            className="inline-flex items-center gap-1.5 rounded-lg border border-sercha-silverline bg-white px-3 py-1.5 text-xs font-medium text-sercha-ink-slate hover:bg-sercha-mist"
          >
            <Settings2 className="h-3.5 w-3.5" />
            Configure
          </button>
          {isConfigured && onRemove && (
            <button
              onClick={handleRemove}
              disabled={removing}
              className={`inline-flex items-center gap-1.5 rounded-lg border px-3 py-1.5 text-xs font-medium ${
                showConfirm
                  ? "border-red-300 bg-red-50 text-red-600 hover:bg-red-100"
                  : "border-sercha-silverline bg-white text-sercha-fog-grey hover:bg-sercha-mist hover:text-red-600"
              }`}
            >
              {removing ? (
                <Loader2 className="h-3.5 w-3.5 animate-spin" />
              ) : (
                <Trash2 className="h-3.5 w-3.5" />
              )}
              {showConfirm ? "Confirm" : "Remove"}
            </button>
          )}
        </div>
      </div>
      {showConfirm && (
        <div className="mt-3 flex items-center justify-between rounded-lg bg-red-50 p-2">
          <span className="text-xs text-red-600">
            This will disable {title.toLowerCase()} features. Are you sure?
          </span>
          <button
            onClick={() => setShowConfirm(false)}
            className="text-xs text-red-600 underline hover:no-underline"
          >
            Cancel
          </button>
        </div>
      )}
    </div>
  );
}

// AI Configuration Section
function AIConfigurationSection({
  aiSettings,
  aiProviders,
  onSave,
  onTest,
  onRefresh,
  onRemoveEmbedding,
  onRemoveLLM,
}: {
  aiSettings: AISettingsResponse | null;
  aiProviders: AIProvidersResponse | null;
  onSave: (embedding: AIProviderConfig | null, llm: AIProviderConfig | null) => Promise<void>;
  onTest: () => Promise<boolean>;
  onRefresh: () => void;
  onRemoveEmbedding: () => Promise<void>;
  onRemoveLLM: () => Promise<void>;
}) {
  const [embeddingWizardOpen, setEmbeddingWizardOpen] = useState(false);
  const [llmWizardOpen, setLlmWizardOpen] = useState(false);
  const [testing, setTesting] = useState(false);
  const [testResult, setTestResult] = useState<"success" | "error" | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [removingEmbedding, setRemovingEmbedding] = useState(false);
  const [removingLLM, setRemovingLLM] = useState(false);

  const handleTest = async () => {
    setTesting(true);
    setTestResult(null);
    setError(null);

    try {
      const success = await onTest();
      setTestResult(success ? "success" : "error");
    } catch (err) {
      setTestResult("error");
      setError(err instanceof Error ? err.message : "Connection test failed");
    } finally {
      setTesting(false);
    }
  };

  const handleSaveEmbedding = async (config: AIProviderConfig) => {
    await onSave(config, null);
    onRefresh();
  };

  const handleSaveLLM = async (config: AIProviderConfig) => {
    await onSave(null, config);
    onRefresh();
  };

  const handleRemoveEmbedding = async () => {
    setRemovingEmbedding(true);
    setError(null);
    try {
      await onRemoveEmbedding();
      onRefresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to remove embedding provider");
    } finally {
      setRemovingEmbedding(false);
    }
  };

  const handleRemoveLLM = async () => {
    setRemovingLLM(true);
    setError(null);
    try {
      await onRemoveLLM();
      onRefresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to remove LLM provider");
    } finally {
      setRemovingLLM(false);
    }
  };

  const isAnyConfigured = aiSettings?.embedding?.is_configured || aiSettings?.llm?.is_configured;

  return (
    <section className="rounded-2xl border-2 border-sercha-silverline bg-white p-6">
      <div className="mb-6 flex items-center justify-between">
        <div>
          <h2 className="text-lg font-semibold text-sercha-ink-slate">
            AI Configuration
          </h2>
          <p className="text-sm text-sercha-fog-grey">
            Configure embedding and LLM providers for semantic search
          </p>
        </div>
      </div>

      {error && (
        <div className="mb-4 flex items-center gap-2 rounded-lg bg-red-50 p-3 text-sm text-red-600">
          <AlertCircle className="h-4 w-4" />
          {error}
        </div>
      )}

      {testResult === "success" && (
        <div className="mb-4 flex items-center gap-2 rounded-lg bg-emerald-50 p-3 text-sm text-emerald-600">
          <Check className="h-4 w-4" />
          Connection test successful!
        </div>
      )}

      <div className="grid gap-4 md:grid-cols-2">
        <AIProviderDisplay
          title="Embedding Provider"
          description="Converts text to vectors for semantic search"
          isConfigured={aiSettings?.embedding?.is_configured ?? false}
          provider={aiSettings?.embedding?.provider}
          model={aiSettings?.embedding?.model}
          onConfigure={() => setEmbeddingWizardOpen(true)}
          onRemove={handleRemoveEmbedding}
          removing={removingEmbedding}
        />
        <AIProviderDisplay
          title="LLM Provider"
          description="Powers AI features like query expansion"
          isConfigured={aiSettings?.llm?.is_configured ?? false}
          provider={aiSettings?.llm?.provider}
          model={aiSettings?.llm?.model}
          onConfigure={() => setLlmWizardOpen(true)}
          onRemove={handleRemoveLLM}
          removing={removingLLM}
        />
      </div>

      {/* Test Button */}
      {isAnyConfigured && (
        <div className="mt-6 flex items-center justify-end border-t border-sercha-mist pt-4">
          <button
            onClick={handleTest}
            disabled={testing}
            className="inline-flex items-center gap-2 rounded-lg border border-sercha-silverline bg-white px-4 py-2 text-sm font-medium text-sercha-ink-slate hover:bg-sercha-mist disabled:opacity-50"
          >
            {testing ? (
              <Loader2 className="h-4 w-4 animate-spin" />
            ) : (
              <Zap className="h-4 w-4" />
            )}
            Test Connection
          </button>
        </div>
      )}

      {/* Wizards */}
      {aiProviders && (
        <>
          <AIConfigWizard
            type="embedding"
            isOpen={embeddingWizardOpen}
            onClose={() => setEmbeddingWizardOpen(false)}
            onSave={handleSaveEmbedding}
            providers={aiProviders.embedding}
            currentConfig={{
              provider: aiSettings?.embedding?.provider,
              model: aiSettings?.embedding?.model,
              hasApiKey: aiSettings?.embedding?.has_api_key,
            }}
          />
          <AIConfigWizard
            type="llm"
            isOpen={llmWizardOpen}
            onClose={() => setLlmWizardOpen(false)}
            onSave={handleSaveLLM}
            providers={aiProviders.llm}
            currentConfig={{
              provider: aiSettings?.llm?.provider,
              model: aiSettings?.llm?.model,
              hasApiKey: aiSettings?.llm?.has_api_key,
            }}
          />
        </>
      )}
    </section>
  );
}

// Main AI Settings Page
export default function AISettingsPage() {
  const [aiSettings, setAISettings] = useState<AISettingsResponse | null>(null);
  const [aiProviders, setAIProviders] = useState<AIProvidersResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchData = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);
      const [aiData, providersData, caps] = await Promise.all([
        getAISettings(),
        getAIProviders(),
        getCapabilities(),
      ]);
      setAISettings(aiData);

      // Filter providers to only show those configured in environment
      const filteredProviders: AIProvidersResponse = {
        embedding: providersData.embedding.filter((p) =>
          caps.ai_providers.embedding.includes(p.id)
        ),
        llm: providersData.llm.filter((p) =>
          caps.ai_providers.llm.includes(p.id)
        ),
      };
      setAIProviders(filteredProviders);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load AI settings");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  const handleSaveAISettings = async (
    embedding: AIProviderConfig | null,
    llm: AIProviderConfig | null
  ) => {
    const updated = await updateAISettings({
      ...(embedding && { embedding }),
      ...(llm && { llm }),
    });
    setAISettings(updated);
  };

  const handleTestAI = async () => {
    await testAIConnection();
    return true;
  };

  const handleRemoveEmbedding = async () => {
    await deleteEmbeddingConfig();
  };

  const handleRemoveLLM = async () => {
    await deleteLLMConfig();
  };

  if (loading) {
    return (
      <AdminLayout title="AI Settings" description="Configure AI providers">
        <div className="flex items-center justify-center py-12">
          <Loader2 className="h-8 w-8 animate-spin text-sercha-indigo" />
        </div>
      </AdminLayout>
    );
  }

  return (
    <AdminLayout title="AI Settings" description="Configure AI providers">
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

        <AIConfigurationSection
          aiSettings={aiSettings}
          aiProviders={aiProviders}
          onSave={handleSaveAISettings}
          onTest={handleTestAI}
          onRefresh={fetchData}
          onRemoveEmbedding={handleRemoveEmbedding}
          onRemoveLLM={handleRemoveLLM}
        />
      </div>
    </AdminLayout>
  );
}
