"use client";

import { useState, useRef, useEffect } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import Image from "next/image";
import {
  Search,
  Settings,
  Info,
  User,
  LayoutDashboard,
  LogOut,
  Loader2,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { useAuth } from "@/lib/auth";
import { getAIStatus, AISettingsStatus, getCapabilityPreferences } from "@/lib/api";

interface ToggleProps {
  enabled: boolean;
  onChange: (enabled: boolean) => void;
  disabled?: boolean;
  label: string;
  tooltip: string;
}

function Toggle({ enabled, onChange, disabled, label, tooltip }: ToggleProps) {
  return (
    <div className="group relative flex items-center gap-2">
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
      <span
        className={cn(
          "text-sm font-medium",
          disabled ? "text-sercha-silverline" : "text-sercha-ink-slate"
        )}
      >
        {label}
      </span>
      <div className="relative">
        <Info
          size={14}
          className={cn(
            "cursor-help",
            disabled ? "text-sercha-silverline" : "text-sercha-fog-grey"
          )}
        />
        {/* Tooltip */}
        <div className="pointer-events-none absolute bottom-full left-1/2 z-10 mb-2 w-48 -translate-x-1/2 rounded-lg bg-sercha-ink-slate px-3 py-2 text-xs text-white opacity-0 shadow-lg transition-opacity group-hover:opacity-100">
          {tooltip}
          {disabled && (
            <span className="mt-1 block text-sercha-silverline">
              Configure in Admin → Settings
            </span>
          )}
          <div className="absolute left-1/2 top-full -translate-x-1/2 border-4 border-transparent border-t-sercha-ink-slate" />
        </div>
      </div>
    </div>
  );
}

export default function SearchHomePage() {
  const router = useRouter();
  const { user, logout, isAdmin, isLoading: authLoading } = useAuth();
  const [query, setQuery] = useState("");
  const [aiStatus, setAiStatus] = useState<AISettingsStatus | null>(null);
  const [loadingStatus, setLoadingStatus] = useState(true);

  // Derived AI configuration from status AND capability preferences
  // Vector search available only if embedding provider is configured AND preference is enabled
  const embeddingConfigured = aiStatus?.embedding?.available ?? false;

  // Vector search toggle state
  const [vectorEnabled, setVectorEnabled] = useState(false);
  const [vectorSearchEnabled, setVectorSearchEnabled] = useState(true);

  // Dropdown states
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [userOpen, setUserOpen] = useState(false);
  const settingsRef = useRef<HTMLDivElement>(null);
  const userRef = useRef<HTMLDivElement>(null);

  // Fetch AI status and capability preferences only after auth is ready
  useEffect(() => {
    // Don't fetch until auth check is complete to avoid 401 race condition
    if (authLoading) return;

    Promise.all([
      getAIStatus().catch((err) => {
        console.error("Failed to fetch AI status:", err);
        return null;
      }),
      getCapabilityPreferences().catch((err) => {
        console.error("Failed to fetch capability preferences:", err);
        return null;
      }),
    ])
      .then(([status, prefs]) => {
        setAiStatus(status);
        const vectorPrefEnabled = prefs?.vector_search_enabled ?? true;
        setVectorSearchEnabled(vectorPrefEnabled);

        // Auto-enable vector search if both provider is available AND preference is enabled
        if (status?.embedding?.available && vectorPrefEnabled) {
          setVectorEnabled(true);
        }
      })
      .finally(() => {
        setLoadingStatus(false);
      });
  }, [authLoading]);

  // Close dropdowns when clicking outside
  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (settingsRef.current && !settingsRef.current.contains(event.target as Node)) {
        setSettingsOpen(false);
      }
      if (userRef.current && !userRef.current.contains(event.target as Node)) {
        setUserOpen(false);
      }
    }
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, []);

  // Derive mode from toggles for URL params
  // Vector ON = hybrid (BM25 + vector via RRF), Vector OFF = text (BM25 only)
  // Query expansion is an additive feature, not a mode selector
  const getMode = () => {
    if (vectorEnabled) return "hybrid";
    return "text";
  };

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault();
    if (query.trim()) {
      router.push(`/search?q=${encodeURIComponent(query)}&mode=${getMode()}`);
    }
  };

  return (
    <div className="flex min-h-screen flex-col bg-sercha-snow">
      {/* Header */}
      <header className="flex h-16 items-center justify-between border-b border-sercha-silverline bg-white px-6">
        <Image
          src="/logo-icon-only.svg"
          alt="Sercha"
          width={32}
          height={32}
          className="h-8 w-8"
        />
        <div className="flex items-center gap-1">
          {/* Admin Link - only for admins */}
          {isAdmin && (
            <Link
              href="/admin"
              className="flex items-center gap-2 rounded-lg px-3 py-2 text-sm font-medium text-sercha-fog-grey transition-colors hover:bg-sercha-mist hover:text-sercha-indigo"
            >
              <LayoutDashboard size={18} />
              Admin
            </Link>
          )}

          {/* Settings Dropdown */}
          <div ref={settingsRef} className="relative">
            <button
              onClick={() => {
                setSettingsOpen(!settingsOpen);
                setUserOpen(false);
              }}
              className="flex items-center gap-2 rounded-lg px-3 py-2 text-sm font-medium text-sercha-fog-grey transition-colors hover:bg-sercha-mist hover:text-sercha-indigo"
            >
              <Settings size={18} />
            </button>
            {settingsOpen && (
              <div className="absolute right-0 top-full mt-2 w-64 rounded-xl border border-sercha-silverline bg-white p-4 shadow-lg">
                <h3 className="mb-3 text-sm font-semibold text-sercha-ink-slate">
                  Settings
                </h3>
                <p className="text-sm text-sercha-fog-grey">
                  Settings panel under development.
                </p>
                <p className="mt-2 text-xs text-sercha-silverline">
                  Preferences, display options, and more coming soon.
                </p>
              </div>
            )}
          </div>

          {/* User Dropdown */}
          <div ref={userRef} className="relative">
            <button
              onClick={() => {
                setUserOpen(!userOpen);
                setSettingsOpen(false);
              }}
              className="flex items-center gap-2 rounded-lg px-3 py-2 text-sm font-medium text-sercha-fog-grey transition-colors hover:bg-sercha-mist hover:text-sercha-indigo"
            >
              <User size={18} />
            </button>
            {userOpen && (
              <div className="absolute right-0 top-full mt-2 w-64 rounded-xl border border-sercha-silverline bg-white p-4 shadow-lg">
                <div className="mb-3 border-b border-sercha-mist pb-3">
                  <p className="text-sm font-semibold text-sercha-ink-slate">
                    {user?.name}
                  </p>
                  <p className="text-xs text-sercha-fog-grey">{user?.email}</p>
                </div>
                <button
                  onClick={() => {
                    logout();
                    setUserOpen(false);
                  }}
                  className="flex w-full items-center gap-2 rounded-lg px-3 py-2 text-left text-sm text-red-600 transition-colors hover:bg-red-50"
                >
                  <LogOut size={16} />
                  Logout
                </button>
              </div>
            )}
          </div>
        </div>
      </header>

      {/* Main Content */}
      <main className="flex flex-1 flex-col items-center justify-center px-4">
        <div className="w-full max-w-2xl">
          {/* Logo */}
          <div className="mb-8 text-center">
            <Image
              src="/logo-wordmark.png"
              alt="Sercha"
              width={200}
              height={56}
              className="mx-auto h-14 w-auto"
            />
            <p className="mt-4 text-sercha-fog-grey">
              Search across all your connected data sources
            </p>
          </div>

          {/* Search Form */}
          <form onSubmit={handleSearch} className="space-y-4">
            {/* Search Input with embedded button */}
            <div className="relative flex items-end rounded-3xl border-2 border-sercha-silverline bg-white transition-all focus-within:border-sercha-indigo focus-within:ring-4 focus-within:ring-sercha-indigo-soft">
              <textarea
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter" && !e.shiftKey) {
                    e.preventDefault();
                    if (query.trim()) {
                      handleSearch(e);
                    }
                  }
                }}
                placeholder="Search documents, code, messages..."
                rows={1}
                className="max-h-32 min-h-[56px] w-full resize-none rounded-3xl bg-transparent py-4 pl-5 pr-14 text-lg text-sercha-ink-slate placeholder:text-sercha-fog-grey focus:outline-none"
                style={{
                  height: "auto",
                  overflowY: query.split("\n").length > 4 ? "auto" : "hidden",
                }}
                onInput={(e) => {
                  const target = e.target as HTMLTextAreaElement;
                  target.style.height = "auto";
                  const lineHeight = 28;
                  const maxLines = 4;
                  const maxHeight = lineHeight * maxLines + 32; // padding
                  target.style.height = `${Math.min(target.scrollHeight, maxHeight)}px`;
                }}
                autoFocus
              />
              <button
                type="submit"
                disabled={!query.trim()}
                className="absolute bottom-3 right-3 flex h-10 w-10 items-center justify-center rounded-full bg-sercha-indigo text-white transition-all hover:bg-sercha-indigo/90 disabled:bg-sercha-silverline disabled:text-sercha-fog-grey"
              >
                <Search size={20} />
              </button>
            </div>

            {/* AI Enhancement Toggles */}
            <div className="flex items-center justify-center gap-6">
              {loadingStatus ? (
                <div className="flex items-center gap-2 text-sercha-fog-grey">
                  <Loader2 size={16} className="animate-spin" />
                  <span className="text-sm">Loading AI status...</span>
                </div>
              ) : (
                <>
                  <Toggle
                    enabled={vectorEnabled}
                    onChange={setVectorEnabled}
                    disabled={!embeddingConfigured || !vectorSearchEnabled}
                    label="Vector Search"
                    tooltip={!embeddingConfigured
                      ? "Requires embedding provider to be configured."
                      : !vectorSearchEnabled
                        ? "Disabled by admin in Capabilities settings."
                        : "Use AI embeddings to find semantically similar content, even when exact keywords don't match."}
                  />
                </>
              )}
            </div>
          </form>
        </div>
      </main>

      {/* Footer */}
      <footer className="px-6 py-4">
        <p className="text-center text-sm text-sercha-fog-grey">
          Sercha v{process.env.APP_VERSION}
        </p>
      </footer>
    </div>
  );
}
