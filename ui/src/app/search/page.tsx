"use client";

import { useState, useEffect, Suspense } from "react";
import { useSearchParams, useRouter } from "next/navigation";
import Link from "next/link";
import Image from "next/image";
import {
  Search,
  FileText,
  Code,
  Github,
  Clock,
  ChevronDown,
  Filter,
  Loader2,
  AlertCircle,
  ExternalLink,
  User,
  LogOut,
  Settings,
} from "lucide-react";
import { search, getDocumentURL, SearchResultItem } from "@/lib/api";
import { useAuth } from "@/lib/auth";

type SearchMode = "text" | "hybrid" | "semantic";

const mimeTypeIcons: Record<string, typeof FileText> = {
  "text/typescript": Code,
  "text/typescript-jsx": Code,
  "text/javascript": Code,
  "text/x-python": Code,
  "text/x-go": Code,
  "text/markdown": FileText,
  "application/json": Code,
  "text/yaml": Code,
  "text/plain": FileText,
};

function getFileIcon(mimeType: string) {
  return mimeTypeIcons[mimeType] || FileText;
}

function highlightQuery(content: string, query: string): string {
  if (!query.trim()) return content;

  const words = query.split(/\s+/).filter(w => w.length > 2);
  if (words.length === 0) return content;

  const regex = new RegExp(`(${words.map(w => w.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')).join('|')})`, 'gi');
  return content.replace(regex, '<mark class="bg-yellow-200 text-sercha-ink-slate">$1</mark>');
}

function truncateContent(content: string, maxLength: number = 300): string {
  if (content.length <= maxLength) return content;
  return content.substring(0, maxLength).trim() + "...";
}

function formatDate(dateString: string): string {
  const date = new Date(dateString);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffHours = Math.floor(diffMs / (1000 * 60 * 60));
  const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24));

  if (diffHours < 1) return "Just now";
  if (diffHours < 24) return `${diffHours} hour${diffHours > 1 ? "s" : ""} ago`;
  if (diffDays < 7) return `${diffDays} day${diffDays > 1 ? "s" : ""} ago`;
  return date.toLocaleDateString();
}

function SearchResultsContent() {
  const searchParams = useSearchParams();
  const router = useRouter();
  const query = searchParams.get("q") || "";
  const mode = (searchParams.get("mode") as SearchMode) || "text";

  const [searchInput, setSearchInput] = useState(query);
  const [selectedMode] = useState<SearchMode>(mode);
  const [results, setResults] = useState<SearchResultItem[]>([]);
  const [totalCount, setTotalCount] = useState(0);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [searched, setSearched] = useState(false);
  const [openingDoc, setOpeningDoc] = useState<string | null>(null);
  const [userMenuOpen, setUserMenuOpen] = useState(false);
  const { user, logout, isAdmin } = useAuth();

  const handleOpenDocument = async (e: React.MouseEvent, docId: string) => {
    e.preventDefault();
    setOpeningDoc(docId);
    try {
      const url = await getDocumentURL(docId);
      window.open(url, '_blank');
    } catch (err) {
      console.error('Failed to open document:', err);
    } finally {
      setOpeningDoc(null);
    }
  };

  useEffect(() => {
    if (!query) {
      setResults([]);
      setTotalCount(0);
      setSearched(false);
      return;
    }

    const controller = new AbortController();
    setLoading(true);
    setError(null);
    setSearched(true);

    search({ query, mode: selectedMode, limit: 20 }, controller.signal)
      .then((response) => {
        setResults(response.results || []);
        setTotalCount(response.total_count || response.results?.length || 0);
      })
      .catch((err) => {
        if (err instanceof DOMException && err.name === "AbortError") return;
        setError(err instanceof Error ? err.message : "Search failed");
        setResults([]);
        setTotalCount(0);
      })
      .finally(() => {
        if (!controller.signal.aborted) {
          setLoading(false);
        }
      });

    return () => controller.abort();
  }, [query, selectedMode]);

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault();
    if (searchInput.trim()) {
      router.push(`/search?q=${encodeURIComponent(searchInput)}&mode=${selectedMode}`);
    }
  };

  return (
    <div className="flex min-h-screen flex-col bg-sercha-snow">
      {/* Header with Search */}
      <header className="sticky top-0 z-10 border-b border-sercha-silverline bg-white">
        <div className="flex h-16 items-center gap-4 px-6">
          {/* Logo */}
          <Link href="/">
            <Image
              src="/logo-icon-only.svg"
              alt="Sercha"
              width={32}
              height={32}
              className="h-8 w-8"
            />
          </Link>

          {/* Search Form */}
          <form onSubmit={handleSearch} className="flex flex-1 items-center gap-2">
            <div className="relative flex-1 max-w-2xl">
              <Search
                size={18}
                className="absolute left-3 top-1/2 -translate-y-1/2 text-sercha-fog-grey"
              />
              <input
                type="text"
                value={searchInput}
                onChange={(e) => setSearchInput(e.target.value)}
                placeholder="Search..."
                className="w-full rounded-full border border-sercha-silverline bg-white py-2 pl-10 pr-4 text-sm text-sercha-ink-slate placeholder:text-sercha-fog-grey focus:border-sercha-indigo focus:outline-none focus:ring-2 focus:ring-sercha-indigo-soft"
              />
            </div>
            <button
              type="submit"
              disabled={loading}
              className="rounded-full bg-sercha-indigo px-4 py-2 text-sm font-semibold text-white transition-all hover:bg-sercha-indigo/90 disabled:opacity-50"
            >
              {loading ? <Loader2 className="h-4 w-4 animate-spin" /> : "Search"}
            </button>
          </form>

          {/* Admin Link */}
          {isAdmin && (
            <Link
              href="/admin"
              className="flex items-center gap-1.5 rounded-lg px-3 py-2 text-sm text-sercha-fog-grey hover:bg-sercha-mist hover:text-sercha-indigo"
            >
              <Settings size={18} />
              Admin
            </Link>
          )}

          {/* User Menu */}
          <div className="relative">
            <button
              onClick={() => setUserMenuOpen(!userMenuOpen)}
              className="flex items-center gap-1 rounded-lg px-2 py-2 text-sercha-fog-grey hover:bg-sercha-mist hover:text-sercha-indigo"
            >
              <User size={20} />
              <ChevronDown size={14} />
            </button>

            {userMenuOpen && (
              <>
                <div className="fixed inset-0 z-10" onClick={() => setUserMenuOpen(false)} />
                <div className="absolute right-0 z-20 mt-1 w-56 rounded-lg border border-sercha-silverline bg-white py-1 shadow-lg">
                  {/* User info */}
                  <div className="border-b border-sercha-mist px-4 py-3">
                    <p className="text-sm font-medium text-sercha-ink-slate">{user?.name}</p>
                    <p className="text-xs text-sercha-fog-grey">{user?.email}</p>
                  </div>
                  {/* Logout */}
                  <button
                    onClick={() => {
                      logout();
                      setUserMenuOpen(false);
                    }}
                    className="flex w-full items-center gap-2 px-4 py-2 text-sm text-red-600 hover:bg-red-50"
                  >
                    <LogOut size={16} />
                    Logout
                  </button>
                </div>
              </>
            )}
          </div>
        </div>

        {/* Filters Bar */}
        {searched && (
          <div className="flex items-center gap-4 border-t border-sercha-mist px-6 py-2">
            <span className="text-sm text-sercha-fog-grey">
              {loading ? (
                "Searching..."
              ) : (
                <>
                  {totalCount} result{totalCount !== 1 ? "s" : ""} for &quot;{query}&quot;
                </>
              )}
            </span>
            <span className="text-sercha-silverline">|</span>
            <span className="text-sm text-sercha-fog-grey">Mode: {mode}</span>
            <div className="flex-1" />
            <button className="flex items-center gap-1 rounded-lg px-3 py-1.5 text-sm text-sercha-fog-grey hover:bg-sercha-mist">
              <Filter size={16} />
              Filters
              <ChevronDown size={16} />
            </button>
          </div>
        )}
      </header>

      {/* Main Content */}
      <main className="flex-1 px-6 py-6">
        <div className="mx-auto max-w-3xl space-y-4">
          {/* Error State */}
          {error && (
            <div className="flex items-center gap-2 rounded-lg bg-red-50 p-4 text-red-600">
              <AlertCircle className="h-5 w-5" />
              {error}
            </div>
          )}

          {/* Loading State */}
          {loading && (
            <div className="flex items-center justify-center py-12">
              <Loader2 className="h-8 w-8 animate-spin text-sercha-indigo" />
            </div>
          )}

          {/* Empty State - No Query */}
          {!searched && !loading && (
            <div className="flex flex-col items-center justify-center rounded-2xl border-2 border-dashed border-sercha-silverline bg-white py-16">
              <Search className="mb-4 h-12 w-12 text-sercha-silverline" />
              <h3 className="mb-2 text-lg font-semibold text-sercha-ink-slate">
                Enter a search query
              </h3>
              <p className="text-sercha-fog-grey">
                Search across all your connected data sources
              </p>
            </div>
          )}

          {/* Empty State - No Results */}
          {searched && !loading && results.length === 0 && !error && (
            <div className="flex flex-col items-center justify-center rounded-2xl border-2 border-dashed border-sercha-silverline bg-white py-16">
              <Search className="mb-4 h-12 w-12 text-sercha-silverline" />
              <h3 className="mb-2 text-lg font-semibold text-sercha-ink-slate">
                No results found
              </h3>
              <p className="text-sercha-fog-grey">
                Try adjusting your search terms or filters
              </p>
            </div>
          )}

          {/* Results List */}
          {!loading &&
            results.map((result) => {
              const FileIcon = getFileIcon(result.mime_type);
              const isGitHub = (() => {
                try {
                  const host = new URL(result.path ?? "").hostname;
                  return host === "github.com" || host.endsWith(".github.com");
                } catch {
                  return false;
                }
              })();

              return (
                <article
                  key={result.document_id}
                  className="rounded-2xl border border-sercha-silverline bg-white p-6 transition-all hover:border-sercha-indigo hover:shadow-md"
                >
                  {/* Title & Source */}
                  <div className="mb-2 flex items-start justify-between">
                    <div className="flex items-center gap-2">
                      <FileIcon size={18} className="text-sercha-fog-grey" />
                      <h3 className="font-semibold text-sercha-ink-slate hover:text-sercha-indigo">
                        <a
                          href="#"
                          onClick={(e) => handleOpenDocument(e, result.document_id)}
                          className="flex items-center gap-1 cursor-pointer"
                        >
                          {result.title}
                          {openingDoc === result.document_id ? (
                            <Loader2 size={14} className="animate-spin opacity-50" />
                          ) : (
                            <ExternalLink size={14} className="opacity-50" />
                          )}
                        </a>
                      </h3>
                    </div>
                    <div className="flex items-center gap-2">
                      <span className="rounded-full bg-sercha-indigo-soft px-2.5 py-0.5 text-xs font-medium text-sercha-indigo">
                        {Math.round(result.score)}%
                      </span>
                      {isGitHub && (
                        <div className="flex items-center gap-1.5 rounded-full bg-sercha-mist px-2.5 py-1 text-xs font-medium text-sercha-fog-grey">
                          <Github size={14} />
                          GitHub
                        </div>
                      )}
                    </div>
                  </div>

                  {/* Path */}
                  <p className="mb-3 text-sm text-sercha-fog-grey font-mono">
                    {result.path || result.title}
                  </p>

                  {/* Content Snippet */}
                  {result.snippet && (
                    <div
                      className="mb-3 text-sm text-sercha-ink-slate leading-relaxed whitespace-pre-wrap font-mono bg-sercha-snow rounded-lg p-3 overflow-x-auto"
                      dangerouslySetInnerHTML={{
                        __html: highlightQuery(
                          truncateContent(result.snippet, 400),
                          query
                        ),
                      }}
                    />
                  )}

                  {/* Meta */}
                  <div className="flex items-center gap-4 text-xs text-sercha-fog-grey">
                    <span className="flex items-center gap-1">
                      <Clock size={14} />
                      {formatDate(result.indexed_at)}
                    </span>
                    <span className="text-sercha-silverline">
                      {result.mime_type}
                    </span>
                  </div>
                </article>
              );
            })}
        </div>
      </main>

      {/* Footer */}
      <footer className="border-t border-sercha-silverline bg-white px-6 py-4">
        <div className="flex items-center justify-center text-sm text-sercha-fog-grey">
          Sercha v{process.env.APP_VERSION}
        </div>
      </footer>
    </div>
  );
}

export default function SearchResultsPage() {
  return (
    <Suspense
      fallback={
        <div className="flex min-h-screen items-center justify-center bg-sercha-snow">
          <Loader2 className="h-8 w-8 animate-spin text-sercha-indigo" />
        </div>
      }
    >
      <SearchResultsContent />
    </Suspense>
  );
}
