"use client";

import { useState, useEffect } from "react";
import {
  X,
  ChevronRight,
  ChevronLeft,
  Loader2,
  Check,
  AlertTriangle,
  RefreshCw,
} from "lucide-react";
import {
  type AIProvider,
  type AIProviderMeta,
  type AIModelMeta,
  type AIProviderConfig,
  getVespaMetrics,
  triggerReindex,
} from "@/lib/api";

type WizardStep = "provider" | "model" | "confirm";

interface AIConfigWizardProps {
  type: "embedding" | "llm";
  isOpen: boolean;
  onClose: () => void;
  onSave: (config: AIProviderConfig) => Promise<void>;
  providers: AIProviderMeta[];
  currentConfig?: {
    provider?: AIProvider;
    model?: string;
    hasApiKey?: boolean;
  };
}

export function AIConfigWizard({
  type,
  isOpen,
  onClose,
  onSave,
  providers,
  currentConfig,
}: AIConfigWizardProps) {
  const [step, setStep] = useState<WizardStep>("provider");
  const [selectedProvider, setSelectedProvider] = useState<AIProviderMeta | null>(null);
  const [selectedModel, setSelectedModel] = useState<AIModelMeta | null>(null);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // For embedding change confirmation
  const [documentCount, setDocumentCount] = useState<number>(0);
  const [_loadingDocCount, setLoadingDocCount] = useState(false);
  const [reindexAfterSave, setReindexAfterSave] = useState(true);
  const [_reindexTriggered, setReindexTriggered] = useState(false);

  // Check if embedding model is changing (requires reindex)
  const isModelChange = type === "embedding" &&
    currentConfig?.model &&
    selectedModel &&
    currentConfig.model !== selectedModel.id;

  // Fetch document count when wizard opens (for embedding type)
  useEffect(() => {
    if (isOpen && type === "embedding") {
      setLoadingDocCount(true);
      getVespaMetrics()
        .then((metrics) => {
          setDocumentCount(metrics.documents?.total ?? 0);
        })
        .catch(() => {
          setDocumentCount(0);
        })
        .finally(() => {
          setLoadingDocCount(false);
        });
    }
  }, [isOpen, type]);

  // Reset state when wizard opens/closes
  useEffect(() => {
    if (isOpen) {
      setStep("provider");
      setSelectedProvider(null);
      setSelectedModel(null);
      setError(null);
      setReindexAfterSave(true);
      setReindexTriggered(false);

      // Pre-select current provider if exists
      if (currentConfig?.provider) {
        const current = providers.find(p => p.id === currentConfig.provider);
        if (current) {
          setSelectedProvider(current);
          const currentModel = current.models.find(m => m.id === currentConfig.model);
          if (currentModel) {
            setSelectedModel(currentModel);
          }
        }
      }
    }
  }, [isOpen, currentConfig, providers]);

  if (!isOpen) return null;

  const title = type === "embedding" ? "Configure Embedding Provider" : "Configure LLM Provider";

  // Determine if we need a confirmation step (embedding with documents and model change)
  const needsConfirmation = type === "embedding" && documentCount > 0 && isModelChange;

  const getStepNumber = (): number => {
    const steps: WizardStep[] = ["provider", "model"];
    if (needsConfirmation) steps.push("confirm");
    return steps.indexOf(step) + 1;
  };

  const getTotalSteps = (): number => {
    let count = 2; // provider + model
    if (needsConfirmation) count++;
    return count;
  };

  const getNextStep = (): WizardStep | null => {
    if (step === "provider") return "model";
    if (step === "model") {
      if (needsConfirmation) return "confirm";
      return null;
    }
    return null;
  };

  const getPrevStep = (): WizardStep | null => {
    if (step === "model") return "provider";
    if (step === "confirm") return "model";
    return null;
  };

  const canProceed = (): boolean => {
    if (step === "provider") return selectedProvider !== null;
    if (step === "model") return selectedModel !== null;
    if (step === "confirm") return true; // User has reviewed the confirmation
    return false;
  };

  const handleNext = () => {
    const next = getNextStep();
    if (next) {
      setStep(next);
    } else {
      handleSave();
    }
  };

  const handleBack = () => {
    const prev = getPrevStep();
    if (prev) setStep(prev);
  };

  const handleSave = async () => {
    if (!selectedProvider || !selectedModel) return;

    setSaving(true);
    setError(null);

    try {
      const config: AIProviderConfig = {
        provider: selectedProvider.id,
        model: selectedModel.id,
      };

      await onSave(config);

      // Trigger reindex if needed and user opted in
      if (needsConfirmation && reindexAfterSave) {
        try {
          await triggerReindex();
          setReindexTriggered(true);
        } catch (reindexErr) {
          // Don't fail the save, just show a warning
          console.warn("Reindex trigger failed:", reindexErr);
        }
      }

      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to save configuration");
    } finally {
      setSaving(false);
    }
  };

  const isLastStep = getNextStep() === null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      {/* Backdrop */}
      <div
        className="absolute inset-0 bg-black/50"
        onClick={onClose}
      />

      {/* Modal */}
      <div className="relative w-full max-w-lg rounded-2xl bg-white shadow-xl">
        {/* Header */}
        <div className="flex items-center justify-between border-b border-sercha-silverline px-6 py-4">
          <h2 className="text-lg font-semibold text-sercha-ink-slate">{title}</h2>
          <button
            onClick={onClose}
            className="rounded-lg p-1 text-sercha-fog-grey hover:bg-sercha-mist hover:text-sercha-ink-slate"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        {/* Progress */}
        <div className="border-b border-sercha-silverline px-6 py-3">
          <div className="flex items-center gap-2 text-sm text-sercha-fog-grey">
            <span className="font-medium text-sercha-indigo">Step {getStepNumber()}</span>
            <span>of {getTotalSteps()}</span>
          </div>
          <div className="mt-2 h-1 w-full overflow-hidden rounded-full bg-sercha-mist">
            <div
              className="h-full bg-sercha-indigo transition-all duration-300"
              style={{ width: `${(getStepNumber() / getTotalSteps()) * 100}%` }}
            />
          </div>
        </div>

        {/* Content */}
        <div className="p-6">
          {error && (
            <div className="mb-4 rounded-lg bg-red-50 px-4 py-3 text-sm text-red-600">
              {error}
            </div>
          )}

          {/* Step: Provider Selection */}
          {step === "provider" && (
            <div>
              <h3 className="mb-4 text-sm font-medium text-sercha-ink-slate">
                Select Provider
              </h3>
              <div className="space-y-2">
                {providers.map((provider) => (
                  <button
                    key={provider.id}
                    onClick={() => {
                      setSelectedProvider(provider);
                      setSelectedModel(provider.models[0] || null);
                    }}
                    className={`w-full rounded-lg border-2 p-4 text-left transition-colors ${
                      selectedProvider?.id === provider.id
                        ? "border-sercha-indigo bg-sercha-indigo/5"
                        : "border-sercha-silverline hover:border-sercha-fog-grey"
                    }`}
                  >
                    <div className="flex items-center justify-between">
                      <div>
                        <p className="font-medium text-sercha-ink-slate">{provider.name}</p>
                        <p className="mt-0.5 text-xs text-sercha-fog-grey">
                          {provider.models.length} model{provider.models.length !== 1 ? "s" : ""} available
                        </p>
                      </div>
                      {selectedProvider?.id === provider.id && (
                        <Check className="h-5 w-5 text-sercha-indigo" />
                      )}
                    </div>
                  </button>
                ))}
              </div>
            </div>
          )}

          {/* Step: Model Selection */}
          {step === "model" && selectedProvider && (
            <div>
              <h3 className="mb-1 text-sm font-medium text-sercha-ink-slate">
                Select Model
              </h3>
              <p className="mb-4 text-xs text-sercha-fog-grey">
                Provider: {selectedProvider.name}
              </p>

              {/* Info about env config */}
              <div className="mb-4 rounded-lg border border-sercha-indigo/20 bg-sercha-indigo/5 p-3">
                <p className="text-xs text-sercha-fog-grey">
                  API keys are configured via environment variables on the server.
                </p>
              </div>

              <div className="space-y-2">
                {selectedProvider.models.map((model) => (
                  <button
                    key={model.id}
                    onClick={() => setSelectedModel(model)}
                    className={`w-full rounded-lg border-2 p-4 text-left transition-colors ${
                      selectedModel?.id === model.id
                        ? "border-sercha-indigo bg-sercha-indigo/5"
                        : "border-sercha-silverline hover:border-sercha-fog-grey"
                    }`}
                  >
                    <div className="flex items-center justify-between">
                      <div>
                        <p className="font-medium text-sercha-ink-slate">{model.name}</p>
                        {model.dimensions && (
                          <p className="mt-0.5 text-xs text-sercha-fog-grey">
                            {model.dimensions} dimensions
                          </p>
                        )}
                      </div>
                      {selectedModel?.id === model.id && (
                        <Check className="h-5 w-5 text-sercha-indigo" />
                      )}
                    </div>
                  </button>
                ))}
              </div>
            </div>
          )}

          {/* Step: Confirmation (for embedding model changes with documents) */}
          {step === "confirm" && (
            <div>
              <div className="mb-4 flex items-start gap-3 rounded-lg border border-amber-200 bg-amber-50 p-4">
                <AlertTriangle className="h-5 w-5 flex-shrink-0 text-amber-600" />
                <div>
                  <h4 className="font-medium text-amber-800">Reindex Required</h4>
                  <p className="mt-1 text-sm text-amber-700">
                    Changing the embedding model requires reindexing all documents.
                    You have <strong>{documentCount.toLocaleString()}</strong> documents indexed.
                  </p>
                </div>
              </div>

              <div className="rounded-lg border border-sercha-silverline p-4">
                <h4 className="mb-2 text-sm font-medium text-sercha-ink-slate">Configuration Change</h4>
                <div className="space-y-1 text-sm text-sercha-fog-grey">
                  <p>
                    <span className="font-medium">From:</span>{" "}
                    {currentConfig?.provider} / {currentConfig?.model}
                  </p>
                  <p>
                    <span className="font-medium">To:</span>{" "}
                    {selectedProvider?.name} / {selectedModel?.name}
                  </p>
                </div>
              </div>

              <div className="mt-4">
                <label className="flex items-center gap-3 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={reindexAfterSave}
                    onChange={(e) => setReindexAfterSave(e.target.checked)}
                    className="h-4 w-4 rounded border-sercha-silverline text-sercha-indigo focus:ring-sercha-indigo"
                  />
                  <span className="text-sm text-sercha-ink-slate">
                    <RefreshCw className="mr-1 inline h-4 w-4" />
                    Trigger reindex after saving
                  </span>
                </label>
                <p className="ml-7 mt-1 text-xs text-sercha-fog-grey">
                  This will regenerate embeddings for all documents using the new model.
                  The process runs in the background.
                </p>
              </div>
            </div>
          )}
        </div>

        {/* Footer */}
        <div className="flex items-center justify-between border-t border-sercha-silverline px-6 py-4">
          <button
            onClick={handleBack}
            disabled={!getPrevStep()}
            className="inline-flex items-center gap-1 rounded-lg px-4 py-2 text-sm font-medium text-sercha-fog-grey hover:text-sercha-ink-slate disabled:invisible"
          >
            <ChevronLeft className="h-4 w-4" />
            Back
          </button>

          <div className="flex gap-3">
            <button
              onClick={onClose}
              className="rounded-lg border border-sercha-silverline bg-white px-4 py-2 text-sm font-medium text-sercha-fog-grey hover:bg-sercha-mist"
            >
              Cancel
            </button>
            <button
              onClick={handleNext}
              disabled={!canProceed() || saving}
              className="inline-flex items-center gap-1 rounded-lg bg-sercha-indigo px-4 py-2 text-sm font-medium text-white hover:bg-sercha-indigo/90 disabled:opacity-50"
            >
              {saving ? (
                <>
                  <Loader2 className="h-4 w-4 animate-spin" />
                  Saving...
                </>
              ) : isLastStep ? (
                step === "confirm" ? (
                  reindexAfterSave ? "Save & Reindex" : "Save Configuration"
                ) : (
                  "Save & Test"
                )
              ) : (
                <>
                  Next
                  <ChevronRight className="h-4 w-4" />
                </>
              )}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
