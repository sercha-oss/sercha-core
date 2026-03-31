// API client for Sercha backend

// Types matching backend API contracts
export interface UserSummary {
  id: string;
  email: string;
  name: string;
  role: "admin" | "member";
  active: boolean;
  created_at?: string;
  last_login_at?: string;
}

export interface UpdateUserRequest {
  name?: string;
  role?: "admin" | "member";
  active?: boolean;
}

export interface LoginRequest {
  email: string;
  password: string;
}

export interface LoginResponse {
  token: string;
  refresh_token: string;
  expires_at: string;
  user: UserSummary;
}

export interface SetupRequest {
  email: string;
  password: string;
  name: string;
}

export interface SetupResponse {
  user: UserSummary;
  message: string;
}

export interface VespaStatus {
  connected: boolean;
  endpoint: string;
  dev_mode: boolean;
  schema_mode: "hybrid" | "bm25";
  embeddings_enabled: boolean;
  embedding_dim: number;
  healthy: boolean;
  indexed_chunks: number;
  can_upgrade?: boolean;
  reindex_required?: boolean;
}

export interface VespaMetrics {
  documents: {
    total: number;
    ready: number;
    active: number;
    removed: number;
  };
  storage: {
    disk_used_bytes: number;
    disk_used_percent: number;
    memory_used_bytes: number;
    memory_used_percent: number;
  };
  query_performance: {
    total_queries: number;
    queries_per_second: number;
    avg_latency_ms: number;
    failed_queries: number;
    degraded_queries: number;
    empty_results: number;
  };
  feed: {
    total_operations: number;
    succeeded_operations: number;
    failed_operations: number;
    pending_operations: number;
    avg_latency_ms: number;
  };
  nodes: VespaNodeMetrics[];
  timestamp: number;
}

export interface VespaNodeMetrics {
  hostname: string;
  role: "container" | "content";
  document_count: number;
  disk_used_bytes: number;
  disk_used_percent: number;
  memory_used_bytes: number;
  memory_used_percent: number;
}

export interface VespaConnectRequest {
  endpoint: string;
  dev_mode?: boolean;
}

export type AIProvider = "openai" | "anthropic" | "ollama" | "cohere" | "voyage";

export interface AIProviderConfig {
  provider: AIProvider;
  model: string;
  api_key?: string;
  base_url?: string;
}

export interface AISettingsRequest {
  embedding?: AIProviderConfig;
  llm?: AIProviderConfig;
}

// Response from GET /api/v1/settings/ai
export interface AIProviderInfo {
  provider?: AIProvider;
  model?: string;
  base_url?: string;
  has_api_key: boolean;
  is_configured: boolean;
}

export interface AISettingsResponse {
  embedding: AIProviderInfo;
  llm: AIProviderInfo;
}

// Vespa service status within AI settings context
export interface VespaServiceStatus {
  connected: boolean;
  schema_mode: "hybrid" | "bm25" | "";
  embeddings_enabled: boolean;
  embedding_dim?: number;
  can_upgrade: boolean;
  healthy: boolean;
}

// Response from GET /api/v1/settings/ai/status
export interface AISettingsStatus {
  embedding: { available: boolean; provider?: string; embedding_dim?: number };
  llm: { available: boolean; provider?: string };
  vespa: VespaServiceStatus;
  effective_search_mode: string;
  reindex_required: boolean;
  reindex_reason?: string;
  schema_upgrade_required?: boolean;
  schema_upgrade_reason?: string;
}

// Response from GET /api/v1/settings/ai/providers
export interface AIModelMeta {
  id: string;
  name: string;
  dimensions?: number;
}

export interface AIProviderMeta {
  id: AIProvider;
  name: string;
  models: AIModelMeta[];
  requires_api_key: boolean;
  requires_base_url: boolean;
  api_key_url?: string;
}

export interface AIProvidersResponse {
  embedding: AIProviderMeta[];
  llm: AIProviderMeta[];
}

export interface ProviderListItem {
  type: string;
  name: string;
  description: string;
  auth_methods: string[];
  configured: boolean;
  enabled: boolean;
}

export interface ProviderConfigRequest {
  client_id: string;
  client_secret: string;
}

export interface ProviderConfigResponse {
  type: string;
  configured: boolean;
  enabled: boolean;
}

export interface CapabilitiesResponse {
  oauth_providers: string[];
  ai_providers: {
    embedding: string[];
    llm: string[];
  };
  features: {
    semantic_search: boolean;
    vector_indexing: boolean;
  };
  limits: {
    sync_min_interval: number;
    sync_max_interval: number;
    max_workers: number;
    max_results_per_page: number;
  };
}

export interface OAuthAuthorizeResponse {
  authorization_url: string;
  state: string;
}

export interface InstallationSummary {
  id: string;
  name: string;
  provider_type: string;
  auth_method: string;
  account_id: string;
  oauth_expiry?: string;
  created_at: string;
  source_count: number;
}

export interface Container {
  id: string;
  name: string;
  description?: string;
  type: string;
  parent_id?: string;
  has_children?: boolean;
  metadata?: Record<string, unknown>;
}

export interface ContainerListResponse {
  containers: Container[];
  next_cursor?: string;
  has_more: boolean;
}

export interface CreateSourceRequest {
  name: string;
  provider_type: string;
  config?: Record<string, unknown>;
  installation_id: string;
  selected_containers: string[];
}

export interface Source {
  id: string;
  name: string;
  provider_type: string;
  enabled: boolean;
  document_count: number;
  last_synced?: string;
  status: "healthy" | "syncing" | "error";
}

// Matches backend domain.SourceSummary
export interface SourceSummaryResponse {
  source: {
    id: string;
    name: string;
    provider_type: string;
    config: Record<string, unknown>;
    enabled: boolean;
    installation_id?: string;
    selected_containers?: string[];
    created_at: string;
    updated_at: string;
    created_by?: string;
  };
  document_count: number;
  last_sync_at?: string;
  sync_status: string;
}

// Flattened version for easier frontend use
export interface SourceSummary {
  id: string;
  name: string;
  provider_type: string;
  enabled: boolean;
  document_count: number;
  last_synced?: string;
  status: "healthy" | "syncing" | "error";
}

export interface SyncState {
  source_id: string;
  status: "idle" | "syncing" | "success" | "error";
  last_sync_time?: string;
  documents_synced: number;
  error_message?: string;
}

export interface ComponentHealth {
  status: string;
  message?: string;
}

export interface HealthResponse {
  status: string;
  components: Record<string, ComponentHealth>;
}

export interface AdminStatsResponse {
  documents: { total: number };
  chunks: { total: number };
  sources: { total: number; enabled: number };
  installations: { total: number };
  users: { total: number };
  // Computed property for convenience
  total_documents?: number;
}

export interface SearchRequest {
  query: string;
  mode?: "hybrid" | "text" | "semantic";
  limit?: number;
  offset?: number;
  source_ids?: string[];
}

export interface SearchChunk {
  id: string;
  document_id: string;
  source_id: string;
  content: string;
  position: number;
  start_char: number;
  end_char: number;
  created_at: string;
}

export interface SearchDocument {
  id: string;
  source_id: string;
  external_id: string;
  path: string;
  title: string;
  mime_type: string;
  metadata: Record<string, string>;
  created_at: string;
  updated_at: string;
  indexed_at: string;
}

export interface SearchResultItem {
  chunk: SearchChunk;
  document: SearchDocument;
  score: number;
  highlights?: string[];
}

export interface SearchResponse {
  query: string;
  mode: string;
  results: SearchResultItem[];
  total_count?: number;
  took?: number;
}

// Additional types for missing endpoints

export interface SyncExclusionPattern {
  pattern: string;
  description: string;
  category: "folder" | "file";
}

export interface SyncExclusionSettings {
  enabled_patterns: string[];
  disabled_patterns: string[];
  custom_patterns: string[];
}

export interface Settings {
  team_id: string;
  default_search_mode: "hybrid" | "text" | "semantic";
  results_per_page: number;
  max_results_per_page: number;
  sync_interval_minutes: number;
  sync_enabled: boolean;
  semantic_search_enabled: boolean;
  auto_suggest_enabled: boolean;
  sync_exclusions?: SyncExclusionSettings;
  updated_at: string;
  updated_by: string;
}

export interface UpdateSettingsRequest {
  default_search_mode?: "hybrid" | "text" | "semantic";
  results_per_page?: number;
  sync_interval_minutes?: number;
  sync_enabled?: boolean;
  semantic_search_enabled?: boolean;
  auto_suggest_enabled?: boolean;
  sync_exclusions?: SyncExclusionSettings;
}

// Default exclusion patterns with metadata for UI display
export const DEFAULT_SYNC_EXCLUSION_PATTERNS: SyncExclusionPattern[] = [
  // Development folders
  { pattern: "node_modules/", description: "Node.js dependencies", category: "folder" },
  { pattern: ".git/", description: "Git repository data", category: "folder" },
  { pattern: ".svn/", description: "Subversion data", category: "folder" },
  { pattern: ".hg/", description: "Mercurial data", category: "folder" },
  // Python
  { pattern: "__pycache__/", description: "Python bytecode cache", category: "folder" },
  { pattern: ".venv/", description: "Python virtual environment", category: "folder" },
  { pattern: "venv/", description: "Python virtual environment", category: "folder" },
  { pattern: "env/", description: "Python virtual environment", category: "folder" },
  { pattern: ".env/", description: "Environment folder", category: "folder" },
  { pattern: "site-packages/", description: "Python packages", category: "folder" },
  { pattern: ".tox/", description: "Tox testing", category: "folder" },
  { pattern: ".pytest_cache/", description: "Pytest cache", category: "folder" },
  { pattern: ".mypy_cache/", description: "Mypy type checking cache", category: "folder" },
  { pattern: "*.egg-info/", description: "Python package info", category: "folder" },
  // IDE/Editor
  { pattern: ".idea/", description: "JetBrains IDE settings", category: "folder" },
  { pattern: ".vscode/", description: "VS Code settings", category: "folder" },
  // Build outputs
  { pattern: "dist/", description: "Distribution folder", category: "folder" },
  { pattern: "build/", description: "Build output", category: "folder" },
  { pattern: ".cache/", description: "Cache folder", category: "folder" },
  { pattern: "coverage/", description: "Coverage reports", category: "folder" },
  { pattern: "htmlcov/", description: "HTML coverage reports", category: "folder" },
  // OS files
  { pattern: ".DS_Store", description: "macOS folder metadata", category: "file" },
  { pattern: "Thumbs.db", description: "Windows thumbnail cache", category: "file" },
  // Compiled files
  { pattern: "*.pyc", description: "Python compiled files", category: "file" },
  { pattern: "*.pyo", description: "Python optimized files", category: "file" },
  { pattern: "*.class", description: "Java class files", category: "file" },
  { pattern: "*.o", description: "Object files", category: "file" },
  { pattern: "*.obj", description: "Object files", category: "file" },
  { pattern: "*.dll", description: "Windows DLL files", category: "file" },
  { pattern: "*.exe", description: "Windows executables", category: "file" },
  { pattern: "*.so", description: "Shared libraries", category: "file" },
  { pattern: "*.dylib", description: "macOS dynamic libraries", category: "file" },
  // Temp files
  { pattern: "*.log", description: "Log files", category: "file" },
  { pattern: "*.tmp", description: "Temporary files", category: "file" },
  { pattern: "*.temp", description: "Temporary files", category: "file" },
  { pattern: "*.swp", description: "Vim swap files", category: "file" },
  { pattern: "*.swo", description: "Vim swap files", category: "file" },
  { pattern: "*~", description: "Backup files", category: "file" },
];

export interface Document {
  id: string;
  source_id: string;
  external_id: string;
  title: string;
  url?: string;
  content_type: string;
  metadata?: Record<string, unknown>;
  created_at: string;
  updated_at: string;
}

export interface DocumentWithChunks extends Document {
  chunks: Chunk[];
}

export interface Chunk {
  id: string;
  document_id: string;
  content: string;
  chunk_index: number;
  token_count: number;
  metadata?: Record<string, unknown>;
}

export interface DocumentListResponse {
  documents: Document[];
  total: number;
  offset: number;
  limit: number;
}

export interface UpdateSourceRequest {
  name?: string;
  config?: Record<string, unknown>;
  enabled?: boolean;
}

export interface UpdateSourceSelectionRequest {
  selected_containers: string[];
}

export interface VersionResponse {
  version: string;
  commit?: string;
  build_time?: string;
}

export interface ReadyResponse {
  ready: boolean;
  components?: Record<string, { ready: boolean; message?: string }>;
}

export interface SetupStatusResponse {
  setup_complete: boolean;
  has_users: boolean;
  has_sources: boolean;
  vespa_connected: boolean;
}

// API Error class
export class ApiError extends Error {
  constructor(
    public status: number,
    public code: string,
    message: string
  ) {
    super(message);
    this.name = "ApiError";
  }
}

// Get API base URL from environment or default
// Uses NEXT_PUBLIC_API_URL at build time, with localStorage override for custom deployments
function getBaseUrl(): string {
  if (typeof window !== "undefined") {
    return localStorage.getItem("sercha_api_url") || process.env.NEXT_PUBLIC_API_URL || "";
  }
  return process.env.NEXT_PUBLIC_API_URL || "";
}

// Token management
function getToken(): string | null {
  if (typeof window === "undefined") return null;
  return localStorage.getItem("sercha_token");
}

function getRefreshToken(): string | null {
  if (typeof window === "undefined") return null;
  return localStorage.getItem("sercha_refresh_token");
}

export function setTokens(token: string, refreshToken: string): void {
  if (typeof window === "undefined") return;
  localStorage.setItem("sercha_token", token);
  localStorage.setItem("sercha_refresh_token", refreshToken);
}

export function clearTokens(): void {
  if (typeof window === "undefined") return;
  localStorage.removeItem("sercha_token");
  localStorage.removeItem("sercha_refresh_token");
}

export function setApiUrl(url: string): void {
  if (typeof window === "undefined") return;
  localStorage.setItem("sercha_api_url", url);
}

// Base fetch wrapper with auth headers
async function apiFetch<T>(
  path: string,
  options: RequestInit = {}
): Promise<T> {
  const baseUrl = getBaseUrl();
  const token = getToken();

  const headers: HeadersInit = {
    "Content-Type": "application/json",
    ...options.headers,
  };

  if (token) {
    (headers as Record<string, string>)["Authorization"] = `Bearer ${token}`;
  }

  const response = await fetch(`${baseUrl}${path}`, {
    ...options,
    headers,
  });

  // Handle 401 - try to refresh token
  if (response.status === 401 && getRefreshToken()) {
    const refreshed = await refreshAuthToken();
    if (refreshed) {
      // Retry with new token
      const newToken = getToken();
      if (newToken) {
        (headers as Record<string, string>)["Authorization"] = `Bearer ${newToken}`;
      }
      const retryResponse = await fetch(`${baseUrl}${path}`, {
        ...options,
        headers,
      });
      if (!retryResponse.ok) {
        const error = await retryResponse.json().catch(() => ({}));
        throw new ApiError(
          retryResponse.status,
          error.code || "UNKNOWN",
          error.message || "Request failed"
        );
      }
      return retryResponse.json();
    }
  }

  if (!response.ok) {
    const error = await response.json().catch(() => ({}));
    throw new ApiError(
      response.status,
      error.code || "UNKNOWN",
      error.message || "Request failed"
    );
  }

  // Handle 204 No Content
  if (response.status === 204) {
    return {} as T;
  }

  return response.json();
}

// Refresh token helper
async function refreshAuthToken(): Promise<boolean> {
  const refreshToken = getRefreshToken();
  if (!refreshToken) return false;

  try {
    const response = await fetch(`${getBaseUrl()}/api/v1/auth/refresh`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ refresh_token: refreshToken }),
    });

    if (!response.ok) {
      clearTokens();
      return false;
    }

    const data: LoginResponse = await response.json();
    setTokens(data.token, data.refresh_token);
    return true;
  } catch {
    clearTokens();
    return false;
  }
}

// ========== Auth API ==========

export async function login(email: string, password: string): Promise<LoginResponse> {
  const response = await apiFetch<LoginResponse>("/api/v1/auth/login", {
    method: "POST",
    body: JSON.stringify({ email, password }),
  });
  setTokens(response.token, response.refresh_token);
  return response;
}

export async function logout(): Promise<void> {
  try {
    await apiFetch("/api/v1/auth/logout", { method: "POST" });
  } finally {
    clearTokens();
  }
}

export async function getCurrentUser(): Promise<UserSummary> {
  return apiFetch<UserSummary>("/api/v1/me");
}

// ========== Setup API ==========

export async function setup(data: SetupRequest): Promise<SetupResponse> {
  return apiFetch<SetupResponse>("/api/v1/setup", {
    method: "POST",
    body: JSON.stringify(data),
  });
}

export async function getSetupStatus(): Promise<SetupStatusResponse> {
  // Public endpoint - no authentication required
  const baseUrl = typeof window !== "undefined"
    ? localStorage.getItem("sercha_api_url") || process.env.NEXT_PUBLIC_API_URL || ""
    : process.env.NEXT_PUBLIC_API_URL || "";

  const response = await fetch(`${baseUrl}/api/v1/setup/status`);
  if (!response.ok) {
    const error = await response.json().catch(() => ({}));
    throw new ApiError(
      response.status,
      error.code || "UNKNOWN",
      error.message || "Failed to get setup status"
    );
  }
  return response.json();
}

// ========== Vespa API ==========

export async function getVespaStatus(): Promise<VespaStatus> {
  return apiFetch<VespaStatus>("/api/v1/admin/vespa/status");
}

export async function connectVespa(data: VespaConnectRequest): Promise<VespaStatus> {
  return apiFetch<VespaStatus>("/api/v1/admin/vespa/connect", {
    method: "POST",
    body: JSON.stringify(data),
  });
}

export async function disconnectVespa(): Promise<{ status: string }> {
  return apiFetch<{ status: string }>("/api/v1/admin/vespa", {
    method: "DELETE",
  });
}

export async function checkVespaHealth(): Promise<{ status: string }> {
  return apiFetch<{ status: string }>("/api/v1/admin/vespa/health");
}

export async function getVespaMetrics(): Promise<VespaMetrics> {
  return apiFetch<VespaMetrics>("/api/v1/admin/vespa/metrics");
}

// ========== AI Settings API ==========

export async function getAISettings(): Promise<AISettingsResponse> {
  return apiFetch<AISettingsResponse>("/api/v1/settings/ai");
}

export async function updateAISettings(data: AISettingsRequest): Promise<AISettingsResponse> {
  return apiFetch<AISettingsResponse>("/api/v1/settings/ai", {
    method: "PUT",
    body: JSON.stringify(data),
  });
}

export async function testAIConnection(): Promise<{ status: string }> {
  return apiFetch<{ status: string }>("/api/v1/settings/ai/test", {
    method: "POST",
  });
}

export async function deleteEmbeddingConfig(): Promise<{ status: string }> {
  return apiFetch<{ status: string }>("/api/v1/settings/ai/embedding", {
    method: "DELETE",
  });
}

export async function deleteLLMConfig(): Promise<{ status: string }> {
  return apiFetch<{ status: string }>("/api/v1/settings/ai/llm", {
    method: "DELETE",
  });
}

export async function getAIProviders(): Promise<AIProvidersResponse> {
  return apiFetch<AIProvidersResponse>("/api/v1/settings/ai/providers");
}

// ========== Providers API ==========

export async function listProviders(): Promise<ProviderListItem[]> {
  return apiFetch<ProviderListItem[]>("/api/v1/providers");
}

export async function getProviderConfig(type: string): Promise<ProviderConfigResponse> {
  return apiFetch<ProviderConfigResponse>(`/api/v1/providers/${type}/config`);
}

// Deprecated: OAuth credentials now managed via environment variables
// export async function saveProviderConfig(
//   type: string,
//   data: ProviderConfigRequest
// ): Promise<ProviderConfigResponse> {
//   return apiFetch<ProviderConfigResponse>(`/api/v1/providers/${type}/config`, {
//     method: "POST",
//     body: JSON.stringify(data),
//   });
// }

// Deprecated: OAuth credentials now managed via environment variables
// export async function deleteProviderConfig(type: string): Promise<void> {
//   await apiFetch(`/api/v1/providers/${type}/config`, { method: "DELETE" });
// }

export async function getCapabilities(): Promise<CapabilitiesResponse> {
  return apiFetch<CapabilitiesResponse>("/api/v1/capabilities");
}

// ========== OAuth API ==========

export async function startOAuth(
  provider: string,
  installationName?: string,
  returnContext?: string
): Promise<OAuthAuthorizeResponse> {
  return apiFetch<OAuthAuthorizeResponse>(`/api/v1/oauth/${provider}/authorize`, {
    method: "POST",
    body: JSON.stringify({
      provider_type: provider,
      installation_name: installationName,
      return_context: returnContext,
    }),
  });
}

export async function handleOAuthCallback(
  code: string,
  state: string,
  provider: string
): Promise<{ installation: InstallationSummary; message: string }> {
  return apiFetch<{ installation: InstallationSummary; message: string }>(
    `/api/v1/oauth/${provider}/exchange`,
    {
      method: "POST",
      body: JSON.stringify({ code, state }),
    }
  );
}

// ========== Installations API ==========

export async function listInstallations(): Promise<InstallationSummary[]> {
  return apiFetch<InstallationSummary[]>("/api/v1/installations");
}

export async function getInstallationContainers(
  id: string,
  cursor?: string,
  parentId?: string
): Promise<ContainerListResponse> {
  const params = new URLSearchParams();
  if (cursor) params.set("cursor", cursor);
  if (parentId) params.set("parent", parentId);
  const queryString = params.toString();
  return apiFetch<ContainerListResponse>(`/api/v1/installations/${id}/containers${queryString ? `?${queryString}` : ""}`);
}

// ========== Sources API ==========

export async function listSources(): Promise<SourceSummary[]> {
  // API returns SourceSummaryResponse[], transform to flattened SourceSummary[]
  const response = await apiFetch<SourceSummaryResponse[]>("/api/v1/sources");
  return response.map((item) => ({
    id: item.source.id,
    name: item.source.name,
    provider_type: item.source.provider_type,
    enabled: item.source.enabled,
    document_count: item.document_count,
    last_synced: item.last_sync_at,
    status: mapSyncStatus(item.sync_status),
  }));
}

function mapSyncStatus(syncStatus: string): "healthy" | "syncing" | "error" {
  switch (syncStatus) {
    case "syncing":
      return "syncing";
    case "error":
      return "error";
    default:
      return "healthy";
  }
}

export async function createSource(data: CreateSourceRequest): Promise<Source> {
  return apiFetch<Source>("/api/v1/sources", {
    method: "POST",
    body: JSON.stringify(data),
  });
}

export async function deleteSource(id: string): Promise<void> {
  await apiFetch(`/api/v1/sources/${id}`, { method: "DELETE" });
}

export async function triggerSync(id: string): Promise<{ status: string; task_id: string }> {
  return apiFetch<{ status: string; task_id: string }>(`/api/v1/sources/${id}/sync`, {
    method: "POST",
  });
}

export async function enableSource(id: string): Promise<void> {
  await apiFetch(`/api/v1/sources/${id}/enable`, { method: "POST" });
}

export async function disableSource(id: string): Promise<void> {
  await apiFetch(`/api/v1/sources/${id}/disable`, { method: "POST" });
}

export async function getSyncStates(): Promise<SyncState[]> {
  return apiFetch<SyncState[]>("/api/v1/sources/sync-states");
}

// ========== Health API ==========

export async function getHealth(): Promise<HealthResponse> {
  return apiFetch<HealthResponse>("/health");
}

// ========== Admin API ==========

export async function getAdminStats(): Promise<AdminStatsResponse> {
  return apiFetch<AdminStatsResponse>("/api/v1/admin/stats");
}

// ========== Document API ==========

export async function getDocumentURL(id: string): Promise<string> {
  const response = await apiFetch<{ url: string }>(`/api/v1/documents/${id}/open`);
  return response.url;
}

// ========== Search API ==========

export async function search(data: SearchRequest): Promise<SearchResponse> {
  return apiFetch<SearchResponse>("/api/v1/search", {
    method: "POST",
    body: JSON.stringify(data),
  });
}

// ========== Users API ==========

export async function listUsers(): Promise<UserSummary[]> {
  return apiFetch<UserSummary[]>("/api/v1/users");
}

export async function getUser(id: string): Promise<UserSummary> {
  return apiFetch<UserSummary>(`/api/v1/users/${id}`);
}

export async function createUser(data: {
  email: string;
  password: string;
  name: string;
  role: "admin" | "member";
}): Promise<UserSummary> {
  return apiFetch<UserSummary>("/api/v1/users", {
    method: "POST",
    body: JSON.stringify(data),
  });
}

export async function updateUser(id: string, data: UpdateUserRequest): Promise<UserSummary> {
  return apiFetch<UserSummary>(`/api/v1/users/${id}`, {
    method: "PUT",
    body: JSON.stringify(data),
  });
}

export async function deleteUser(id: string): Promise<void> {
  await apiFetch(`/api/v1/users/${id}`, { method: "DELETE" });
}

export async function resetUserPassword(id: string, newPassword: string): Promise<void> {
  await apiFetch(`/api/v1/users/${id}/reset-password`, {
    method: "POST",
    body: JSON.stringify({ new_password: newPassword }),
  });
}

// ========== AI Status API ==========

export async function getAIStatus(): Promise<AISettingsStatus> {
  return apiFetch<AISettingsStatus>("/api/v1/settings/ai/status");
}

// ========== Additional Installation APIs ==========

export async function getInstallation(id: string): Promise<InstallationSummary> {
  return apiFetch<InstallationSummary>(`/api/v1/installations/${id}`);
}

export async function deleteInstallation(id: string): Promise<void> {
  await apiFetch(`/api/v1/installations/${id}`, { method: "DELETE" });
}

export async function testInstallation(id: string): Promise<{ status: string; message?: string }> {
  return apiFetch<{ status: string; message?: string }>(`/api/v1/installations/${id}/test`, {
    method: "POST",
  });
}

export interface InstallationSourceSummary {
  source: {
    id: string;
    name: string;
    provider_type: string;
    enabled: boolean;
  };
  document_count: number;
  sync_status: string;
  last_sync_at?: string;
}

export async function getInstallationSources(id: string): Promise<InstallationSourceSummary[]> {
  return apiFetch<InstallationSourceSummary[]>(`/api/v1/installations/${id}/sources`);
}

// ========== Additional Source APIs ==========

export async function getSource(id: string): Promise<Source> {
  return apiFetch<Source>(`/api/v1/sources/${id}`);
}

export async function updateSource(id: string, data: UpdateSourceRequest): Promise<Source> {
  return apiFetch<Source>(`/api/v1/sources/${id}`, {
    method: "PUT",
    body: JSON.stringify(data),
  });
}

export async function updateSourceSelection(
  id: string,
  data: UpdateSourceSelectionRequest
): Promise<Source> {
  return apiFetch<Source>(`/api/v1/sources/${id}/selection`, {
    method: "PUT",
    body: JSON.stringify(data),
  });
}

export async function getSourceDocuments(
  id: string,
  options?: { limit?: number; offset?: number }
): Promise<DocumentListResponse> {
  const params = new URLSearchParams();
  if (options?.limit) params.set("limit", options.limit.toString());
  if (options?.offset) params.set("offset", options.offset.toString());
  const query = params.toString() ? `?${params.toString()}` : "";
  return apiFetch<DocumentListResponse>(`/api/v1/sources/${id}/documents${query}`);
}

export async function getSourceSyncState(id: string): Promise<SyncState> {
  return apiFetch<SyncState>(`/api/v1/sources/${id}/sync`);
}

// ========== Settings APIs ==========

export async function getSettings(): Promise<Settings> {
  return apiFetch<Settings>("/api/v1/settings");
}

export async function updateSettings(data: UpdateSettingsRequest): Promise<Settings> {
  return apiFetch<Settings>("/api/v1/settings", {
    method: "PUT",
    body: JSON.stringify(data),
  });
}

// ========== Document APIs ==========

export async function getDocument(id: string): Promise<DocumentWithChunks> {
  return apiFetch<DocumentWithChunks>(`/api/v1/documents/${id}`);
}

export async function getDocumentChunks(id: string): Promise<Chunk[]> {
  return apiFetch<Chunk[]>(`/api/v1/documents/${id}/chunks`);
}

// ========== Additional Health APIs ==========

export async function getReady(): Promise<ReadyResponse> {
  return apiFetch<ReadyResponse>("/ready");
}

export async function getVersion(): Promise<VersionResponse> {
  return apiFetch<VersionResponse>("/version");
}

// ========== Job History API ==========

export interface JobSummary {
  id: string;
  type: string;
  status: "pending" | "processing" | "completed" | "failed";
  created_at: string;
  started_at?: string;
  completed_at?: string;
  duration_ms?: number;
  attempts: number;
  error?: string;
}

export interface JobHistoryResponse {
  jobs: JobSummary[];
  total: number;
  limit: number;
  offset: number;
}

export interface PendingJob {
  id: string;
  type: string;
  status: string;
  created_at: string;
  scheduled_for: string;
  priority: number;
  payload?: Record<string, string>;
}

export interface ScheduledJob {
  id: string;
  name: string;
  type: string;
  interval_minutes: number;
  enabled: boolean;
  last_run?: string;
  next_run: string;
  last_error?: string;
}

export interface UpcomingJobsResponse {
  pending_tasks: PendingJob[];
  scheduled_tasks: ScheduledJob[];
}

export interface JobDetail {
  id: string;
  type: string;
  status: string;
  team_id: string;
  payload?: Record<string, string>;
  priority: number;
  attempts: number;
  max_attempts: number;
  error?: string;
  created_at: string;
  updated_at: string;
  started_at?: string;
  completed_at?: string;
  scheduled_for: string;
  duration_ms?: number;
}

export interface JobStats {
  pending: number;
  processing: number;
  completed: number;
  failed: number;
  oldest_pending_age_seconds: number;
}

export interface ListJobHistoryParams {
  limit?: number;
  offset?: number;
  status?: "completed" | "failed";
  type?: string;
}

export async function getJobHistory(params?: ListJobHistoryParams): Promise<JobHistoryResponse> {
  const searchParams = new URLSearchParams();
  if (params?.limit) searchParams.set("limit", params.limit.toString());
  if (params?.offset) searchParams.set("offset", params.offset.toString());
  if (params?.status) searchParams.set("status", params.status);
  if (params?.type) searchParams.set("type", params.type);
  const query = searchParams.toString() ? `?${searchParams.toString()}` : "";
  return apiFetch<JobHistoryResponse>(`/api/v1/admin/jobs${query}`);
}

export async function getUpcomingJobs(): Promise<UpcomingJobsResponse> {
  return apiFetch<UpcomingJobsResponse>("/api/v1/admin/jobs/upcoming");
}

export async function getJob(id: string): Promise<JobDetail> {
  return apiFetch<JobDetail>(`/api/v1/admin/jobs/${id}`);
}

export async function getJobStats(): Promise<JobStats> {
  return apiFetch<JobStats>("/api/v1/admin/jobs/stats");
}

// ========== Search Analytics API ==========

export interface QueryCount {
  query: string;
  count: number;
}

export interface SearchAnalytics {
  total_searches: number;
  searches_today: number;
  searches_this_week: number;
  avg_latency_ms: number;
  avg_result_count: number;
  unique_users: number;
  top_queries: QueryCount[];
}

export interface SearchQueryRecord {
  id: string;
  user_id: string;
  team_id: string;
  query: string;
  mode: string;
  result_count: number;
  latency_ms: number;
  source_ids?: string[];
  created_at: string;
}

export interface SearchHistoryResponse {
  searches: SearchQueryRecord[];
  total: number;
  limit: number;
  offset: number;
}

export interface ListSearchHistoryParams {
  limit?: number;
  offset?: number;
}

export async function getSearchAnalytics(): Promise<SearchAnalytics> {
  return apiFetch<SearchAnalytics>("/api/v1/admin/search/analytics");
}

export async function getSearchHistory(params?: ListSearchHistoryParams): Promise<SearchHistoryResponse> {
  const searchParams = new URLSearchParams();
  if (params?.limit) searchParams.set("limit", params.limit.toString());
  if (params?.offset) searchParams.set("offset", params.offset.toString());
  const query = searchParams.toString() ? `?${searchParams.toString()}` : "";
  return apiFetch<SearchHistoryResponse>(`/api/v1/admin/search/history${query}`);
}

// Search Metrics types and API

export interface SearchMetricPoint {
  timestamp: string;
  search_count: number;
}

export interface SearchMetrics {
  points: SearchMetricPoint[];
  total_count: number;
  period: "hourly" | "daily";
}

export async function getSearchMetrics(period: "hourly" | "daily" = "hourly"): Promise<SearchMetrics> {
  return apiFetch<SearchMetrics>(`/api/v1/admin/search/metrics?period=${period}`);
}

// ========== Reindex API ==========

export interface ReindexResponse {
  message: string;
  task_id: string;
}

export async function triggerReindex(): Promise<ReindexResponse> {
  return apiFetch<ReindexResponse>("/api/v1/admin/reindex", {
    method: "POST",
    body: JSON.stringify({ confirm: true }),
  });
}
