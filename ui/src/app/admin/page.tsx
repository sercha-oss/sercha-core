"use client";

import { useState, useEffect, useCallback, useMemo } from "react";
import { AdminLayout } from "@/components/layout";
import { HealthCard } from "@/components/dashboard";
import {
  getHealth,
  getAdminStats,
  listSources,
  getJobHistory,
  getUpcomingJobs,
  getJobStats,
  getSearchAnalytics,
  getSearchMetrics,
  getSearchHistory,
  getAIStatus,
  HealthResponse,
  AdminStatsResponse,
  SourceSummary,
  JobHistoryResponse,
  UpcomingJobsResponse,
  JobStats,
  SearchAnalytics,
  SearchMetrics,
  SearchQueryRecord,
  AISettingsStatus,
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

  const total = stats.completed + stats.failed + stats.pending + stats.processing;

  return (
    <div className="flex items-center gap-6">
      <div className="flex items-center gap-2">
        <div className="h-2 w-2 rounded-full bg-emerald-500" />
        <span className="text-sm text-sercha-fog-grey">{stats.completed} completed</span>
      </div>
      {stats.processing > 0 && (
        <div className="flex items-center gap-2">
          <div className="h-2 w-2 animate-pulse rounded-full bg-indigo-500" />
          <span className="text-sm text-sercha-fog-grey">{stats.processing} running</span>
        </div>
      )}
      {stats.pending > 0 && (
        <div className="flex items-center gap-2">
          <div className="h-2 w-2 rounded-full bg-amber-500" />
          <span className="text-sm text-sercha-fog-grey">{stats.pending} pending</span>
        </div>
      )}
      {stats.failed > 0 && (
        <div className="flex items-center gap-2">
          <div className="h-2 w-2 rounded-full bg-red-500" />
          <span className="text-sm text-sercha-fog-grey">{stats.failed} failed</span>
        </div>
      )}
      {total === 0 && (
        <span className="text-sm text-sercha-silverline">No jobs yet</span>
      )}
    </div>
  );
}

// Search metrics chart component
function SearchMetricsChart({ metrics }: { metrics: SearchMetrics | null }) {
  const chartData = useMemo(() => {
    if (!metrics?.points?.length) return null;

    const points = metrics.points;
    const maxCount = Math.max(...points.map((p) => p.search_count), 1);
    const width = 300;
    const height = 80;
    const paddingTop = 20; // Extra space for labels above bars
    const paddingBottom = 10;
    const paddingX = 10;

    const pathPoints = points.map((point, i) => {
      const x = paddingX + (i / Math.max(points.length - 1, 1)) * (width - 2 * paddingX);
      const y = paddingTop + (1 - point.search_count / maxCount) * (height - paddingTop - paddingBottom);
      return { x, y, count: point.search_count, timestamp: point.timestamp };
    });

    return { pathPoints, width, height, paddingTop, paddingBottom, paddingX };
  }, [metrics]);

  if (!chartData) {
    return (
      <div className="flex h-20 items-center justify-center text-sm text-sercha-fog-grey">
        No search data yet
      </div>
    );
  }

  // For single point, show a bar instead of a line
  if (chartData.pathPoints.length === 1) {
    const p = chartData.pathPoints[0];
    const barWidth = 40;
    const barHeight = chartData.height - chartData.paddingBottom - p.y;
    const textY = Math.max(14, p.y - 6); // Ensure text is never cut off at top
    return (
      <svg viewBox={`0 0 ${chartData.width} ${chartData.height}`} className="h-20 w-full">
        <defs>
          <linearGradient id="searchGradient" x1="0%" y1="0%" x2="0%" y2="100%">
            <stop offset="0%" stopColor="rgb(147, 51, 234)" stopOpacity="0.3" />
            <stop offset="100%" stopColor="rgb(147, 51, 234)" stopOpacity="0" />
          </linearGradient>
        </defs>
        <rect
          x={(chartData.width - barWidth) / 2}
          y={p.y}
          width={barWidth}
          height={barHeight}
          fill="url(#searchGradient)"
          rx="4"
        />
        <rect
          x={(chartData.width - barWidth) / 2}
          y={p.y}
          width={barWidth}
          height={4}
          fill="rgb(147, 51, 234)"
          rx="2"
        />
        <text
          x={chartData.width / 2}
          y={textY}
          textAnchor="middle"
          className="fill-purple-600 text-xs font-medium"
          fontSize="12"
        >
          {p.count}
        </text>
      </svg>
    );
  }

  const linePath = chartData.pathPoints
    .map((p, i) => `${i === 0 ? "M" : "L"} ${p.x} ${p.y}`)
    .join(" ");
  const areaPath = `${linePath} L ${chartData.pathPoints[chartData.pathPoints.length - 1].x} ${chartData.height - chartData.paddingBottom} L ${chartData.pathPoints[0].x} ${chartData.height - chartData.paddingBottom} Z`;

  return (
    <svg viewBox={`0 0 ${chartData.width} ${chartData.height}`} className="h-20 w-full">
      <defs>
        <linearGradient id="searchGradient" x1="0%" y1="0%" x2="0%" y2="100%">
          <stop offset="0%" stopColor="rgb(147, 51, 234)" stopOpacity="0.3" />
          <stop offset="100%" stopColor="rgb(147, 51, 234)" stopOpacity="0" />
        </linearGradient>
      </defs>
      <path d={areaPath} fill="url(#searchGradient)" />
      <path d={linePath} fill="none" stroke="rgb(147, 51, 234)" strokeWidth="2" />
      {chartData.pathPoints.map((p, i) => (
        <circle key={i} cx={p.x} cy={p.y} r="3" fill="rgb(147, 51, 234)" />
      ))}
    </svg>
  );
}

export default function AdminDashboardPage() {
  const [health, setHealth] = useState<HealthResponse | null>(null);
  const [stats, setStats] = useState<AdminStatsResponse | null>(null);
  const [sources, setSources] = useState<SourceSummary[]>([]);
  const [jobHistory, setJobHistory] = useState<JobHistoryResponse | null>(null);
  const [upcomingJobs, setUpcomingJobs] = useState<UpcomingJobsResponse | null>(null);
  const [jobStats, setJobStats] = useState<JobStats | null>(null);
  const [searchAnalytics, setSearchAnalytics] = useState<SearchAnalytics | null>(null);
  const [searchMetrics, setSearchMetrics] = useState<SearchMetrics | null>(null);
  const [searchHistory, setSearchHistory] = useState<SearchQueryRecord[] | null>(null);
  const [aiStatus, setAIStatus] = useState<AISettingsStatus | null>(null);
  const [metricsPeriod, setMetricsPeriod] = useState<"hourly" | "daily">("hourly");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchData = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);
      const [healthData, statsData, sourcesData, historyData, upcomingData, jobStatsData, searchAnalyticsData, searchMetricsData, searchHistoryData, aiStatusData] = await Promise.all([
        getHealth().catch(() => null),
        getAdminStats().catch(() => null),
        listSources().catch(() => []),
        getJobHistory({ limit: 10 }).catch(() => null),
        getUpcomingJobs().catch(() => null),
        getJobStats().catch(() => null),
        getSearchAnalytics().catch(() => null),
        getSearchMetrics(metricsPeriod).catch(() => null),
        getSearchHistory({ limit: 5 }).catch(() => null),
        getAIStatus().catch(() => null),
      ]);
      setHealth(healthData);
      setStats(statsData);
      setSources(sourcesData);
      setJobHistory(historyData);
      setUpcomingJobs(upcomingData);
      setJobStats(jobStatsData);
      setSearchAnalytics(searchAnalyticsData);
      setSearchMetrics(searchMetricsData);
      setSearchHistory(searchHistoryData?.searches || null);
      setAIStatus(aiStatusData);
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
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-5">
            <HealthCard
              title="PostgreSQL"
              status={getComponentHealth("postgres")}
              message={health?.components?.postgres?.message}
            />
            <HealthCard
              title="Vespa"
              status={getComponentHealth("vespa")}
              message={health?.components?.vespa?.message}
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
              <p className="text-sm font-medium text-sercha-fog-grey">Searches Today</p>
              <Search className="h-4 w-4 text-purple-500" />
            </div>
            <p className="text-3xl font-bold text-sercha-ink-slate">
              {searchAnalytics?.searches_today || 0}
            </p>
            <p className="mt-1 text-sm text-sercha-fog-grey">
              {searchAnalytics?.total_searches || 0} total ({searchAnalytics?.unique_users || 0} users)
            </p>
          </div>

          {/* Job Queue Summary */}
          <div className="rounded-xl border border-sercha-silverline bg-white p-4">
            <div className="mb-2 flex items-center justify-between">
              <p className="text-sm font-medium text-sercha-fog-grey">Job Queue</p>
              <Clock className="h-4 w-4 text-amber-500" />
            </div>
            <p className="text-3xl font-bold text-sercha-ink-slate">
              {jobStats ? (jobStats.completed + jobStats.failed + jobStats.pending + jobStats.processing) : 0}
            </p>
            <p className="mt-1 text-sm text-sercha-fog-grey">
              {jobStats?.processing || 0} running, {jobStats?.pending || 0} pending
            </p>
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
                    {jobHistory.jobs.slice(0, 4).map((job) => (
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
                              {job.duration_ms && ` · ${formatDuration(job.duration_ms)}`}
                            </p>
                          </div>
                        </div>
                        {job.error && (
                          <span className="max-w-[100px] truncate text-xs text-red-500" title={job.error}>
                            {job.error}
                          </span>
                        )}
                      </div>
                    ))}
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
                    {upcomingJobs?.scheduled_tasks?.slice(0, 3).map((task) => (
                      <div
                        key={task.id}
                        className="flex items-center justify-between rounded-lg border border-sercha-mist p-2.5"
                      >
                        <div className="flex items-center gap-2">
                          <RefreshCw className={`h-4 w-4 ${task.enabled ? "text-sercha-indigo" : "text-gray-400"}`} />
                          <div>
                            <p className="text-sm font-medium text-sercha-ink-slate">{task.name}</p>
                            <p className="text-xs text-sercha-fog-grey">
                              Every {task.interval_minutes}m
                              {task.next_run && ` · Next: ${new Date(task.next_run).toLocaleTimeString()}`}
                            </p>
                          </div>
                        </div>
                        <span className={`text-xs ${task.enabled ? "text-emerald-600" : "text-gray-400"}`}>
                          {task.enabled ? "Active" : "Disabled"}
                        </span>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            </div>
          </div>

          {/* Search Activity Chart */}
          <div className="rounded-2xl border border-sercha-silverline bg-white p-6">
            <div className="mb-4 flex items-center justify-between">
              <h3 className="font-semibold text-sercha-ink-slate">Search Activity</h3>
              <select
                value={metricsPeriod}
                onChange={(e) => setMetricsPeriod(e.target.value as "hourly" | "daily")}
                className="rounded-lg border border-sercha-silverline bg-white px-3 py-1.5 text-sm text-sercha-fog-grey focus:border-sercha-indigo focus:outline-none focus:ring-1 focus:ring-sercha-indigo"
              >
                <option value="hourly">Last 24 Hours</option>
                <option value="daily">Last 30 Days</option>
              </select>
            </div>
            <SearchMetricsChart metrics={searchMetrics} />
            <div className="mt-4 flex items-center justify-between text-sm">
              <span className="text-sercha-fog-grey">
                {searchMetrics?.total_count || 0} searches in period
              </span>
              {searchMetrics?.points?.length ? (
                <span className="text-sercha-fog-grey">
                  {metricsPeriod === "hourly" ? "By hour" : "By day"}
                </span>
              ) : null}
            </div>
            {/* Recent Searches */}
            {searchHistory && searchHistory.length > 0 && (
              <div className="mt-4 border-t border-sercha-mist pt-4">
                <h4 className="mb-3 text-xs font-medium uppercase tracking-wide text-sercha-fog-grey">Recent Searches</h4>
                <div className="space-y-2">
                  {searchHistory.map((search) => (
                    <div key={search.id} className="flex items-center justify-between text-sm">
                      <div className="flex min-w-0 items-center gap-2">
                        <Search className="h-3.5 w-3.5 flex-shrink-0 text-sercha-silverline" />
                        <span className="truncate text-sercha-ink-slate">{search.query}</span>
                      </div>
                      <div className="flex flex-shrink-0 items-center gap-2 text-xs text-sercha-fog-grey">
                        <span>{search.result_count} results</span>
                        <span>·</span>
                        <span>{formatRelativeTime(search.created_at)}</span>
                      </div>
                    </div>
                  ))}
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
