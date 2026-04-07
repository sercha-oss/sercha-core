"use client";

import { useState, useEffect, useCallback, useMemo } from "react";
import { AdminLayout } from "@/components/layout";
import { HealthCard } from "@/components/dashboard";
import {
  getHealth,
  getAdminStats,
  listSources,
  getJobs,
  getUpcomingJobs,
  getJobStats,
  getSearchAnalytics,
  getSearchMetrics,
  getSearchHistory,
  getAIStatus,
  getCapabilityPreferences,
  HealthResponse,
  AdminStatsResponse,
  SourceSummary,
  JobHistory,
  UpcomingJobs,
  JobStats,
  SearchAnalytics,
  SearchMetrics,
  SearchQuery,
  AISettingsStatus,
  CapabilityPreferencesResponse,
} from "@/lib/api";
import {
  RefreshCw,
  FileText,
  Clock,
  CheckCircle2,
  XCircle,
  AlertCircle,
  Calendar,
  Loader2,
  ChevronRight,
  Database,
  FolderSync,
  Search,
  Zap,
} from "lucide-react";
import Link from "next/link";
import Image from "next/image";

type HealthStatus = "healthy" | "degraded" | "unhealthy" | "loading";

function getProviderIcon(providerType: string): string {
  const icons: Record<string, string> = {
    github: "/logos/github/github_icon.png",
    gitlab: "/logos/gitlab/gitlab_logo.png",
    slack: "/logos/slack/Slack_icon_2019.svg.png",
    notion: "/logos/notion/notion_icon.png",
    confluence: "/logos/atlassian/confluence.svg",
    jira: "/logos/atlassian/jira.svg",
    google_drive: "/logos/google/google_drive_icon.png",
  };
  return icons[providerType] || "/icon.svg";
}

function formatRelativeTime(dateString: string): string {
  const date = new Date(dateString);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffSec = Math.floor(diffMs / 1000);
  const diffMin = Math.floor(diffSec / 60);
  const diffHour = Math.floor(diffMin / 60);
  const diffDay = Math.floor(diffHour / 24);

  if (diffSec < 60) return "just now";
  if (diffMin < 60) return `${diffMin}m ago`;
  if (diffHour < 24) return `${diffHour}h ago`;
  if (diffDay < 7) return `${diffDay}d ago`;
  return date.toLocaleDateString();
}

function formatDuration(ms: number): string {
  if (ms < 1000) return `${ms}ms`;
  if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`;
  return `${Math.floor(ms / 60000)}m ${Math.floor((ms % 60000) / 1000)}s`;
}

function JobTypeLabel({ type }: { type: string }) {
  const labels: Record<string, { label: string; color: string }> = {
    sync_all: { label: "Full Sync", color: "bg-indigo-100 text-indigo-700" },
    sync_source: { label: "Source Sync", color: "bg-blue-100 text-blue-700" },
    refresh_tokens: { label: "Token Refresh", color: "bg-purple-100 text-purple-700" },
  };
  const config = labels[type] || { label: type, color: "bg-gray-100 text-gray-700" };
  return (
    <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${config.color}`}>
      {config.label}
    </span>
  );
}

function JobStatusBadge({ status }: { status: string }) {
  const configs: Record<string, { icon: typeof CheckCircle2; color: string; bg: string }> = {
    completed: { icon: CheckCircle2, color: "text-emerald-600", bg: "bg-emerald-50" },
    failed: { icon: XCircle, color: "text-red-600", bg: "bg-red-50" },
    pending: { icon: Clock, color: "text-gray-600", bg: "bg-gray-50" },
    processing: { icon: Loader2, color: "text-indigo-600", bg: "bg-indigo-50" },
  };
  const config = configs[status] || configs.pending;
  const Icon = config.icon;
  return (
    <span className={`flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium ${config.bg} ${config.color}`}>
      <Icon className={`h-3 w-3 ${status === "processing" ? "animate-spin" : ""}`} />
      {status}
    </span>
  );
}

// Mini chart component for documents
function DocumentsChart({ total }: { total: number }) {
  // Generate mock trend data based on current total (in real app, this would come from API)
  const data = useMemo(() => {
    const points = 6;
    const result = [];
    for (let i = 0; i < points; i++) {
      // Simulate growth towards current total
      const factor = 0.5 + (i / (points - 1)) * 0.5;
      result.push(Math.floor(total * factor * (0.9 + Math.random() * 0.2)));
    }
    result[points - 1] = total; // Ensure last point is actual total
    return result;
  }, [total]);

  const max = Math.max(...data, 1);
  const height = 40;
  const width = 100;

  const points = data.map((v, i) => ({
    x: (i / (data.length - 1)) * width,
    y: height - (v / max) * height * 0.8,
  }));

  const linePath = points.map((p, i) => (i === 0 ? `M ${p.x},${p.y}` : `L ${p.x},${p.y}`)).join(" ");
  const areaPath = `${linePath} L ${width},${height} L 0,${height} Z`;

  return (
    <svg viewBox={`0 0 ${width} ${height}`} className="h-10 w-full" preserveAspectRatio="none">
      <defs>
        <linearGradient id="docGradient" x1="0" x2="0" y1="0" y2="1">
          <stop offset="0%" stopColor="#6675FF" stopOpacity="0.3" />
          <stop offset="100%" stopColor="#6675FF" stopOpacity="0" />
        </linearGradient>
      </defs>
      <path d={areaPath} fill="url(#docGradient)" />
      <path d={linePath} fill="none" stroke="#6675FF" strokeWidth="2" vectorEffect="non-scaling-stroke" />
    </svg>
  );
}

// Job queue summary component
function JobQueueSummary({ stats }: { stats: JobStats | null }) {
  if (!stats) {
    return (
      <div className="flex items-center gap-4 text-sm text-sercha-fog-grey">
        <Loader2 className="h-4 w-4 animate-spin" />
        Loading...
      </div>
    );
  }

  const total = stats.total_jobs;

  return (
    <div className="flex items-center gap-6">
      <div className="flex items-center gap-2">
        <div className="h-2 w-2 rounded-full bg-emerald-500" />
        <span className="text-sm text-sercha-fog-grey">{stats.completed_jobs} completed</span>
      </div>
      {stats.processing_jobs > 0 && (
        <div className="flex items-center gap-2">
          <div className="h-2 w-2 animate-pulse rounded-full bg-indigo-500" />
          <span className="text-sm text-sercha-fog-grey">{stats.processing_jobs} running</span>
        </div>
      )}
      {stats.pending_jobs > 0 && (
        <div className="flex items-center gap-2">
          <div className="h-2 w-2 rounded-full bg-amber-500" />
          <span className="text-sm text-sercha-fog-grey">{stats.pending_jobs} pending</span>
        </div>
      )}
      {stats.failed_jobs > 0 && (
        <div className="flex items-center gap-2">
          <div className="h-2 w-2 rounded-full bg-red-500" />
          <span className="text-sm text-sercha-fog-grey">{stats.failed_jobs} failed</span>
        </div>
      )}
      {total === 0 && (
        <span className="text-sm text-sercha-silverline">No jobs yet</span>
      )}
    </div>
  );
}

// Search metrics chart component - displays performance metrics visually
function SearchMetricsChart({ metrics }: { metrics: SearchMetrics | null }) {
  if (!metrics) {
    return (
      <div className="flex h-20 items-center justify-center text-sm text-sercha-fog-grey">
        No search data yet
      </div>
    );
  }

  // Display search speed distribution as a horizontal bar chart
  const total = metrics.fast_searches + metrics.medium_searches + metrics.slow_searches;
  if (total === 0) {
    return (
      <div className="flex h-20 items-center justify-center text-sm text-sercha-fog-grey">
        No search data yet
      </div>
    );
  }

  const fastPercent = (metrics.fast_searches / total) * 100;
  const mediumPercent = (metrics.medium_searches / total) * 100;
  const slowPercent = (metrics.slow_searches / total) * 100;

  return (
    <div className="space-y-3">
      {/* Performance distribution bar */}
      <div className="flex h-8 overflow-hidden rounded-lg">
        {fastPercent > 0 && (
          <div
            className="flex items-center justify-center bg-emerald-500 text-xs font-medium text-white"
            style={{ width: `${fastPercent}%` }}
          >
            {fastPercent > 15 && `${fastPercent.toFixed(0)}%`}
          </div>
        )}
        {mediumPercent > 0 && (
          <div
            className="flex items-center justify-center bg-amber-500 text-xs font-medium text-white"
            style={{ width: `${mediumPercent}%` }}
          >
            {mediumPercent > 15 && `${mediumPercent.toFixed(0)}%`}
          </div>
        )}
        {slowPercent > 0 && (
          <div
            className="flex items-center justify-center bg-red-500 text-xs font-medium text-white"
            style={{ width: `${slowPercent}%` }}
          >
            {slowPercent > 15 && `${slowPercent.toFixed(0)}%`}
          </div>
        )}
      </div>

      {/* Legend */}
      <div className="flex items-center justify-between text-xs text-sercha-fog-grey">
        <div className="flex items-center gap-2">
          <div className="h-2 w-2 rounded-full bg-emerald-500" />
          <span>Fast (&lt;100ms): {metrics.fast_searches}</span>
        </div>
        <div className="flex items-center gap-2">
          <div className="h-2 w-2 rounded-full bg-amber-500" />
          <span>Medium (100-500ms): {metrics.medium_searches}</span>
        </div>
        <div className="flex items-center gap-2">
          <div className="h-2 w-2 rounded-full bg-red-500" />
          <span>Slow (&gt;500ms): {metrics.slow_searches}</span>
        </div>
      </div>

      {/* Percentile stats */}
      <div className="mt-2 flex items-center justify-between text-xs text-sercha-fog-grey">
        <span>P50: {metrics.p50_duration_ms.toFixed(0)}ms</span>
        <span>P95: {metrics.p95_duration_ms.toFixed(0)}ms</span>
        <span>P99: {metrics.p99_duration_ms.toFixed(0)}ms</span>
      </div>
    </div>
  );
}

export default function AdminDashboardPage() {
  const [health, setHealth] = useState<HealthResponse | null>(null);
  const [stats, setStats] = useState<AdminStatsResponse | null>(null);
  const [sources, setSources] = useState<SourceSummary[]>([]);
  const [jobHistory, setJobHistory] = useState<JobHistory | null>(null);
  const [upcomingJobs, setUpcomingJobs] = useState<UpcomingJobs | null>(null);
  const [jobStats, setJobStats] = useState<JobStats | null>(null);
  const [searchAnalytics, setSearchAnalytics] = useState<SearchAnalytics | null>(null);
  const [searchMetrics, setSearchMetrics] = useState<SearchMetrics | null>(null);
  const [searchHistory, setSearchHistory] = useState<SearchQuery[] | null>(null);
  const [aiStatus, setAIStatus] = useState<AISettingsStatus | null>(null);
  const [capabilityPrefs, setCapabilityPrefs] = useState<CapabilityPreferencesResponse | null>(null);
  const [metricsPeriod, setMetricsPeriod] = useState<"24h" | "7d" | "30d">("24h");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchData = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);
      const [healthData, statsData, sourcesData, historyData, upcomingData, jobStatsData, searchAnalyticsData, searchMetricsData, searchHistoryData, aiStatusData, capabilityPrefsData] = await Promise.all([
        getHealth().catch(() => null),
        getAdminStats().catch(() => null),
        listSources().catch(() => []),
        getJobs({ limit: 10 }).catch(() => null),
        getUpcomingJobs().catch(() => null),
        getJobStats(metricsPeriod).catch(() => null),
        getSearchAnalytics(metricsPeriod).catch(() => null),
        getSearchMetrics(metricsPeriod).catch(() => null),
        getSearchHistory({ limit: 5 }).catch(() => null),
        getAIStatus().catch(() => null),
        getCapabilityPreferences().catch(() => null),
      ]);
      setHealth(healthData);
      setStats(statsData);
      setSources(sourcesData);
      setJobHistory(historyData);
      setUpcomingJobs(upcomingData);
      setJobStats(jobStatsData);
      setSearchAnalytics(searchAnalyticsData);
      setSearchMetrics(searchMetricsData);
      setSearchHistory(searchHistoryData || null);
      setAIStatus(aiStatusData);
      setCapabilityPrefs(capabilityPrefsData);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load dashboard");
    } finally {
      setLoading(false);
    }
  }, [metricsPeriod]);

  useEffect(() => {
    fetchData();
    const interval = setInterval(fetchData, 30000);
    return () => clearInterval(interval);
  }, [fetchData]);

  const getComponentHealth = (component: string): HealthStatus => {
    if (!health) return "loading";
    const comp = health.components?.[component];
    if (!comp) return "unhealthy";
    return comp.status as HealthStatus;
  };

  const getAIHealthStatus = (): HealthStatus => {
    if (!aiStatus) return "loading";
    if (aiStatus.embedding?.available) return "healthy";
    return "degraded"; // Not configured, but not broken
  };

  const getAIHealthMessage = (): string => {
    if (!aiStatus) return "Loading...";
    if (aiStatus.embedding?.available) {
      const provider = aiStatus.embedding.provider || "configured";
      return `Embedding: ${provider}${aiStatus.llm?.available ? ` | LLM: ${aiStatus.llm.provider || "configured"}` : ""}`;
    }
    return "Not configured";
  };

  return (
    <AdminLayout title="Dashboard" description="Overview of your search system">
      <div className="space-y-6">
        {/* Header with refresh */}
        <div className="flex items-center justify-between">
          <div />
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

        {/* System Health */}
        <section>
          <h2 className="mb-4 text-lg font-semibold text-sercha-ink-slate">
            System Health
          </h2>
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
            <HealthCard
              title="PostgreSQL"
              status={getComponentHealth("postgres")}
              message={health?.components?.postgres?.message}
            />
            <HealthCard
              title="Redis"
              status={health?.components?.redis ? getComponentHealth("redis") : "degraded"}
              message={health?.components?.redis?.message || "Not configured"}
            />
            <HealthCard
              title="API Server"
              status={getComponentHealth("server")}
              message={health?.components?.server?.message}
            />
            <HealthCard
              title="AI Services"
              status={getAIHealthStatus()}
              message={getAIHealthMessage()}
            />
          </div>
        </section>

        {/* Stats Cards */}
        <section className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          {/* Documents Indexed - with mini chart */}
          <div className="rounded-xl border border-sercha-silverline bg-white p-4">
            <div className="mb-2 flex items-center justify-between">
              <p className="text-sm font-medium text-sercha-fog-grey">Documents Indexed</p>
              <Database className="h-4 w-4 text-sercha-indigo" />
            </div>
            <p className="text-3xl font-bold text-sercha-ink-slate">
              {stats?.documents?.total?.toLocaleString() || "0"}
            </p>
            <div className="mt-2">
              <DocumentsChart total={stats?.documents?.total || 0} />
            </div>
          </div>

          {/* Sources */}
          <div className="rounded-xl border border-sercha-silverline bg-white p-4">
            <div className="mb-2 flex items-center justify-between">
              <p className="text-sm font-medium text-sercha-fog-grey">Connected Sources</p>
              <FolderSync className="h-4 w-4 text-emerald-500" />
            </div>
            <p className="text-3xl font-bold text-sercha-ink-slate">
              {stats?.sources?.total || 0}
            </p>
            <p className="mt-1 text-sm text-sercha-fog-grey">
              {stats?.sources?.enabled || 0} enabled
            </p>
          </div>

          {/* Searches */}
          <div className="rounded-xl border border-sercha-silverline bg-white p-4">
            <div className="mb-2 flex items-center justify-between">
              <p className="text-sm font-medium text-sercha-fog-grey">Searches</p>
              <Search className="h-4 w-4 text-purple-500" />
            </div>
            <p className="text-3xl font-bold text-sercha-ink-slate">
              {searchAnalytics?.total_searches || 0}
            </p>
            <p className="mt-1 text-sm text-sercha-fog-grey">
              {searchAnalytics?.unique_users || 0} users · {searchAnalytics?.average_duration_ms?.toFixed(0) || 0}ms avg
            </p>
          </div>

          {/* Job Queue Summary */}
          <div className="rounded-xl border border-sercha-silverline bg-white p-4">
            <div className="mb-2 flex items-center justify-between">
              <p className="text-sm font-medium text-sercha-fog-grey">Job Queue</p>
              <Clock className="h-4 w-4 text-amber-500" />
            </div>
            <p className="text-3xl font-bold text-sercha-ink-slate">
              {jobStats?.total_jobs || 0}
            </p>
            <p className="mt-1 text-sm text-sercha-fog-grey">
              {jobStats?.processing_jobs || 0} running, {jobStats?.pending_jobs || 0} pending
            </p>
          </div>
        </section>

        {/* Capabilities Overview */}
        <section>
          <div className="mb-4 flex items-center justify-between">
            <h2 className="text-lg font-semibold text-sercha-ink-slate">Capabilities</h2>
            <Link href="/admin/capabilities" className="text-sm text-sercha-indigo hover:underline">
              Manage capabilities
            </Link>
          </div>
          <div className="rounded-2xl border border-sercha-silverline bg-white p-6">
            <div className="flex items-center gap-4">
              <div className="flex h-12 w-12 items-center justify-center rounded-xl bg-sercha-indigo/10">
                <Zap className="h-6 w-6 text-sercha-indigo" />
              </div>
              <div className="flex-1">
                {capabilityPrefs ? (
                  <>
                    <div className="flex flex-wrap gap-3">
                      <span
                        className={`inline-flex items-center gap-1.5 rounded-full px-3 py-1 text-xs font-medium ${
                          capabilityPrefs.text_indexing_enabled
                            ? "bg-emerald-100 text-emerald-700"
                            : "bg-gray-100 text-gray-500"
                        }`}
                      >
                        <span
                          className={`h-1.5 w-1.5 rounded-full ${
                            capabilityPrefs.text_indexing_enabled ? "bg-emerald-500" : "bg-gray-400"
                          }`}
                        />
                        Text Indexing
                      </span>
                      <span
                        className={`inline-flex items-center gap-1.5 rounded-full px-3 py-1 text-xs font-medium ${
                          capabilityPrefs.embedding_indexing_enabled
                            ? "bg-emerald-100 text-emerald-700"
                            : "bg-gray-100 text-gray-500"
                        }`}
                      >
                        <span
                          className={`h-1.5 w-1.5 rounded-full ${
                            capabilityPrefs.embedding_indexing_enabled ? "bg-emerald-500" : "bg-gray-400"
                          }`}
                        />
                        Embedding Indexing
                      </span>
                      <span
                        className={`inline-flex items-center gap-1.5 rounded-full px-3 py-1 text-xs font-medium ${
                          capabilityPrefs.bm25_search_enabled
                            ? "bg-blue-100 text-blue-700"
                            : "bg-gray-100 text-gray-500"
                        }`}
                      >
                        <span
                          className={`h-1.5 w-1.5 rounded-full ${
                            capabilityPrefs.bm25_search_enabled ? "bg-blue-500" : "bg-gray-400"
                          }`}
                        />
                        BM25 Search
                      </span>
                      <span
                        className={`inline-flex items-center gap-1.5 rounded-full px-3 py-1 text-xs font-medium ${
                          capabilityPrefs.vector_search_enabled
                            ? "bg-blue-100 text-blue-700"
                            : "bg-gray-100 text-gray-500"
                        }`}
                      >
                        <span
                          className={`h-1.5 w-1.5 rounded-full ${
                            capabilityPrefs.vector_search_enabled ? "bg-blue-500" : "bg-gray-400"
                          }`}
                        />
                        Vector Search
                      </span>
                    </div>
                    <p className="mt-2 text-xs text-sercha-fog-grey">
                      {[
                        capabilityPrefs.text_indexing_enabled,
                        capabilityPrefs.embedding_indexing_enabled,
                        capabilityPrefs.bm25_search_enabled,
                        capabilityPrefs.vector_search_enabled,
                      ].filter(Boolean).length} of 4 capabilities enabled
                    </p>
                  </>
                ) : (
                  <p className="text-sm text-sercha-fog-grey">
                    Loading capabilities...
                  </p>
                )}
              </div>
            </div>
          </div>
        </section>

        {/* Jobs and Search Activity Section */}
        <section className="grid gap-6 lg:grid-cols-2">
          {/* Combined Jobs Card */}
          <div className="rounded-2xl border border-sercha-silverline bg-white">
            <div className="border-b border-sercha-silverline p-6">
              <div className="flex items-center justify-between">
                <h3 className="font-semibold text-sercha-ink-slate">Jobs</h3>
                <JobQueueSummary stats={jobStats} />
              </div>
            </div>
            <div className="divide-y divide-sercha-mist">
              {/* Recent Jobs Section */}
              <div className="p-6">
                <h4 className="mb-3 text-sm font-medium text-sercha-fog-grey">Recent</h4>
                {!jobHistory?.jobs?.length ? (
                  <div className="flex flex-col items-center justify-center py-4 text-center">
                    <Clock className="mb-2 h-6 w-6 text-sercha-silverline" />
                    <p className="text-xs text-sercha-fog-grey">No job history yet</p>
                  </div>
                ) : (
                  <div className="space-y-2">
                    {jobHistory.jobs.slice(0, 4).map((job) => {
                      // Calculate duration from started_at and completed_at
                      let durationMs: number | undefined;
                      if (job.started_at && job.completed_at) {
                        const start = new Date(job.started_at).getTime();
                        const end = new Date(job.completed_at).getTime();
                        durationMs = end - start;
                      }
                      return (
                        <div
                          key={job.id}
                          className="flex items-center justify-between rounded-lg border border-sercha-mist p-2.5"
                        >
                          <div className="flex items-center gap-2">
                            <JobStatusBadge status={job.status} />
                            <div>
                              <JobTypeLabel type={job.type} />
                              <p className="mt-0.5 text-xs text-sercha-fog-grey">
                                {formatRelativeTime(job.created_at)}
                                {durationMs && ` · ${formatDuration(durationMs)}`}
                              </p>
                            </div>
                          </div>
                          {job.error && (
                            <span className="max-w-[100px] truncate text-xs text-red-500" title={job.error}>
                              {job.error}
                            </span>
                          )}
                        </div>
                      );
                    })}
                  </div>
                )}
              </div>
              {/* Scheduled Jobs Section */}
              <div className="p-6">
                <h4 className="mb-3 text-sm font-medium text-sercha-fog-grey">Scheduled</h4>
                {!upcomingJobs?.scheduled_tasks?.length && !upcomingJobs?.pending_tasks?.length ? (
                  <div className="flex flex-col items-center justify-center py-4 text-center">
                    <Calendar className="mb-2 h-6 w-6 text-sercha-silverline" />
                    <p className="text-xs text-sercha-fog-grey">No scheduled tasks</p>
                  </div>
                ) : (
                  <div className="space-y-2">
                    {/* Pending tasks */}
                    {upcomingJobs?.pending_tasks?.slice(0, 2).map((task) => (
                      <div
                        key={task.id}
                        className="flex items-center justify-between rounded-lg border border-amber-100 bg-amber-50/50 p-2.5"
                      >
                        <div className="flex items-center gap-2">
                          <Clock className="h-4 w-4 text-amber-600" />
                          <div>
                            <JobTypeLabel type={task.type} />
                            <p className="mt-0.5 text-xs text-sercha-fog-grey">
                              {new Date(task.scheduled_for).toLocaleTimeString()}
                            </p>
                          </div>
                        </div>
                        <span className="text-xs text-amber-600">Pending</span>
                      </div>
                    ))}
                    {/* Scheduled recurring tasks */}
                    {upcomingJobs?.scheduled_tasks?.slice(0, 3).map((task) => {
                      // Convert interval from nanoseconds to minutes
                      const intervalMinutes = Math.floor(task.interval / (60 * 1000000000));
                      return (
                        <div
                          key={task.id}
                          className="flex items-center justify-between rounded-lg border border-sercha-mist p-2.5"
                        >
                          <div className="flex items-center gap-2">
                            <RefreshCw className={`h-4 w-4 ${task.enabled ? "text-sercha-indigo" : "text-gray-400"}`} />
                            <div>
                              <p className="text-sm font-medium text-sercha-ink-slate">{task.name}</p>
                              <p className="text-xs text-sercha-fog-grey">
                                Every {intervalMinutes}m
                                {task.next_run && ` · Next: ${new Date(task.next_run).toLocaleTimeString()}`}
                              </p>
                            </div>
                          </div>
                          <span className={`text-xs ${task.enabled ? "text-emerald-600" : "text-gray-400"}`}>
                            {task.enabled ? "Active" : "Disabled"}
                          </span>
                        </div>
                      );
                    })}
                  </div>
                )}
              </div>
            </div>
          </div>

          {/* Search Activity Chart */}
          <div className="rounded-2xl border border-sercha-silverline bg-white p-6">
            <div className="mb-4 flex items-center justify-between">
              <h3 className="font-semibold text-sercha-ink-slate">Search Performance</h3>
              <select
                value={metricsPeriod}
                onChange={(e) => setMetricsPeriod(e.target.value as "24h" | "7d" | "30d")}
                className="rounded-lg border border-sercha-silverline bg-white px-3 py-1.5 text-sm text-sercha-fog-grey focus:border-sercha-indigo focus:outline-none focus:ring-1 focus:ring-sercha-indigo"
              >
                <option value="24h">Last 24 Hours</option>
                <option value="7d">Last 7 Days</option>
                <option value="30d">Last 30 Days</option>
              </select>
            </div>
            <SearchMetricsChart metrics={searchMetrics} />
            {searchMetrics && searchMetrics.zero_result_searches > 0 && (
              <div className="mt-3 flex items-center gap-2 text-sm text-amber-600">
                <AlertCircle className="h-4 w-4" />
                <span>{searchMetrics.zero_result_searches} searches returned no results</span>
              </div>
            )}
            {/* Recent Searches */}
            {searchHistory && searchHistory.length > 0 && (
              <div className="mt-4 border-t border-sercha-mist pt-4">
                <h4 className="mb-3 text-xs font-medium uppercase tracking-wide text-sercha-fog-grey">Recent Searches</h4>
                <div className="space-y-2">
                  {searchHistory.map((search) => {
                    // Convert duration from nanoseconds to milliseconds
                    const durationMs = Math.floor(search.duration / 1000000);
                    return (
                      <div key={search.id} className="flex items-center justify-between text-sm">
                        <div className="flex min-w-0 items-center gap-2">
                          <Search className="h-3.5 w-3.5 flex-shrink-0 text-sercha-silverline" />
                          <span className="truncate text-sercha-ink-slate">{search.query}</span>
                        </div>
                        <div className="flex flex-shrink-0 items-center gap-2 text-xs text-sercha-fog-grey">
                          <span>{search.result_count} results</span>
                          <span>·</span>
                          <span>{durationMs}ms</span>
                          <span>·</span>
                          <span>{formatRelativeTime(search.created_at)}</span>
                        </div>
                      </div>
                    );
                  })}
                </div>
              </div>
            )}
          </div>
        </section>

        {/* Connected Sources */}
        <section>
          <div className="mb-4 flex items-center justify-between">
            <h2 className="text-lg font-semibold text-sercha-ink-slate">Connected Sources</h2>
            <Link href="/admin/sources" className="text-sm text-sercha-indigo hover:underline">
              Manage sources
            </Link>
          </div>
          {!sources.length ? (
            <div className="rounded-2xl border-2 border-dashed border-sercha-silverline bg-white p-8 text-center">
              <FileText className="mx-auto mb-3 h-10 w-10 text-sercha-silverline" />
              <p className="text-sm font-medium text-sercha-fog-grey">No sources connected</p>
              <p className="mb-4 text-xs text-sercha-silverline">Connect your first data source to start indexing</p>
              <Link
                href="/admin/sources/new"
                className="inline-flex items-center gap-2 rounded-lg bg-sercha-indigo px-4 py-2 text-sm font-medium text-white hover:bg-sercha-indigo/90"
              >
                Add Source
              </Link>
            </div>
          ) : (
            <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
              {sources.slice(0, 6).map((source) => (
                <Link
                  key={source.id}
                  href={`/admin/sources/view?id=${source.id}`}
                  className="group flex items-center gap-4 rounded-xl border border-sercha-silverline bg-white p-4 transition-all hover:border-sercha-indigo hover:shadow-md"
                >
                  <div className="flex h-12 w-12 flex-shrink-0 items-center justify-center rounded-lg bg-sercha-snow p-2">
                    <Image
                      src={getProviderIcon(source.provider_type)}
                      alt={source.name}
                      width={32}
                      height={32}
                      className="h-8 w-8 object-contain"
                    />
                  </div>
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-2">
                      <p className="text-sm font-semibold text-sercha-ink-slate">{source.name}</p>
                      {source.status === "syncing" && (
                        <span className="h-2 w-2 animate-pulse rounded-full bg-sercha-indigo" />
                      )}
                    </div>
                    <p className="text-xs text-sercha-fog-grey">
                      {source.document_count?.toLocaleString() || 0} documents
                    </p>
                  </div>
                  <ChevronRight className="h-4 w-4 text-sercha-silverline transition-colors group-hover:text-sercha-indigo" />
                </Link>
              ))}
            </div>
          )}
        </section>
      </div>
    </AdminLayout>
  );
}
