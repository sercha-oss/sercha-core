"use client";

import { useState, useEffect } from "react";
import Link from "next/link";
import { CheckCircle2, ArrowRight, Sparkles, Link2, FileText, Loader2, Zap } from "lucide-react";
import { getAISettings, listProviders, listSources, getCapabilityPreferences } from "@/lib/api";
import { useAuth } from "@/lib/auth";

interface StepCompleteProps {
  completedSteps: number[];
}

interface ActualStatus {
  aiConfigured: boolean;
  capabilitiesConfigured: boolean;
  providerConfigured: boolean;
  sourceConnected: boolean;
}

export function StepComplete({ completedSteps }: StepCompleteProps) {
  const { isAuthenticated, isLoading: authLoading } = useAuth();
  const [loading, setLoading] = useState(true);
  const [status, setStatus] = useState<ActualStatus>({
    aiConfigured: false,
    capabilitiesConfigured: false,
    providerConfigured: false,
    sourceConnected: false,
  });

  // Fetch actual status from API on mount
  useEffect(() => {
    const fetchStatus = async () => {
      try {
        const [aiSettings, capabilityPrefs, providers, sources] = await Promise.all([
          getAISettings().catch(() => null),
          getCapabilityPreferences().catch(() => null),
          listProviders().catch(() => []),
          listSources().catch(() => []),
        ]);

        // Check if any capabilities are enabled
        const capabilitiesEnabled = capabilityPrefs
          ? capabilityPrefs.text_indexing_enabled ||
            capabilityPrefs.embedding_indexing_enabled ||
            capabilityPrefs.bm25_search_enabled ||
            capabilityPrefs.vector_search_enabled
          : false;

        setStatus({
          aiConfigured: aiSettings?.embedding?.is_configured || aiSettings?.llm?.is_configured || false,
          capabilitiesConfigured: capabilitiesEnabled,
          providerConfigured: providers.some((p) => p.configured),
          sourceConnected: sources.length > 0,
        });
      } catch {
        // Fall back to completedSteps if API fails
        setStatus({
          aiConfigured: completedSteps.includes(2),
          capabilitiesConfigured: completedSteps.includes(3),
          providerConfigured: completedSteps.includes(4),
          sourceConnected: completedSteps.includes(4),
        });
      } finally {
        setLoading(false);
      }
    };
    fetchStatus();
  }, [completedSteps]);

  const features = [
    {
      icon: Sparkles,
      title: "AI Configured",
      description: "Semantic search enabled",
      completed: status.aiConfigured,
    },
    {
      icon: Zap,
      title: "Capabilities Configured",
      description: "Search capabilities enabled",
      completed: status.capabilitiesConfigured,
    },
    {
      icon: Link2,
      title: "Provider Configured",
      description: "OAuth credentials saved",
      completed: status.providerConfigured,
    },
    {
      icon: FileText,
      title: "Source Connected",
      description: "Data syncing started",
      completed: status.sourceConnected,
    },
  ];

  const configuredCount = features.filter((f) => f.completed).length;

  if (loading || authLoading) {
    return (
      <div className="flex items-center justify-center py-12">
        <Loader2 className="h-8 w-8 animate-spin text-sercha-indigo" />
      </div>
    );
  }

  // Determine the destination and button text based on auth state
  const ctaHref = isAuthenticated ? "/admin" : "/login";
  const ctaText = isAuthenticated ? "Go to Dashboard" : "Login to Continue";

  return (
    <div className="mx-auto max-w-lg text-center">
      {/* Success Icon */}
      <div className="mb-6 flex justify-center">
        <div className="relative">
          <div className="flex h-20 w-20 items-center justify-center rounded-full bg-emerald-100">
            <CheckCircle2 className="h-12 w-12 text-emerald-600" />
          </div>
          <div className="absolute -bottom-1 -right-1 flex h-8 w-8 items-center justify-center rounded-full bg-sercha-indigo text-sm font-bold text-white">
            {configuredCount}
          </div>
        </div>
      </div>

      {/* Title */}
      <h1 className="text-2xl font-semibold text-sercha-ink-slate">
        Your Sercha is Ready!
      </h1>
      <p className="mt-2 text-sm text-sercha-fog-grey">
        You&apos;ve successfully set up your enterprise search platform.
      </p>

      {/* Configuration Summary */}
      <div className="mt-8 rounded-xl border border-sercha-silverline bg-white p-4">
        <div className="grid gap-3">
          {features.map((feature) => (
            <div
              key={feature.title}
              className={`flex items-center gap-3 rounded-lg p-3 ${
                feature.completed ? "bg-emerald-50" : "bg-sercha-mist"
              }`}
            >
              <div
                className={`flex h-8 w-8 items-center justify-center rounded-full ${
                  feature.completed
                    ? "bg-emerald-100 text-emerald-600"
                    : "bg-sercha-silverline text-sercha-fog-grey"
                }`}
              >
                <feature.icon size={16} />
              </div>
              <div className="text-left">
                <p
                  className={`text-sm font-medium ${
                    feature.completed
                      ? "text-emerald-700"
                      : "text-sercha-fog-grey"
                  }`}
                >
                  {feature.title}
                </p>
                <p className="text-xs text-sercha-fog-grey">
                  {feature.completed ? feature.description : "Not configured"}
                </p>
              </div>
            </div>
          ))}
        </div>
      </div>

      {/* What's Next */}
      <div className="mt-8 rounded-xl border border-sercha-indigo/20 bg-sercha-indigo/5 p-4 text-left">
        <h2 className="mb-2 text-sm font-medium text-sercha-ink-slate">
          What&apos;s Next?
        </h2>
        <ul className="space-y-2 text-sm text-sercha-fog-grey">
          <li className="flex items-center gap-2">
            <span className="h-1.5 w-1.5 rounded-full bg-sercha-indigo" />
            Connect more data sources from the dashboard
          </li>
          <li className="flex items-center gap-2">
            <span className="h-1.5 w-1.5 rounded-full bg-sercha-indigo" />
            Invite team members to search your knowledge base
          </li>
          <li className="flex items-center gap-2">
            <span className="h-1.5 w-1.5 rounded-full bg-sercha-indigo" />
            Fine-tune AI settings for better search results
          </li>
        </ul>
      </div>

      {/* CTA Button */}
      <Link
        href={ctaHref}
        className="mt-8 inline-flex items-center justify-center gap-2 rounded-lg bg-sercha-indigo px-8 py-3 text-sm font-medium text-white transition-colors hover:bg-sercha-indigo/90"
      >
        {ctaText}
        <ArrowRight size={16} />
      </Link>
    </div>
  );
}
