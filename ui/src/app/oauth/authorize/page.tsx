"use client";

import { useEffect, useState, Suspense } from "react";
import { useSearchParams } from "next/navigation";
import { Loader2, Eye, EyeOff, Shield, Search, FileText, List } from "lucide-react";
import Image from "next/image";
import {
  login as apiLogin,
  completeOAuthAuthorize,
  getCurrentUser,
  getOAuthClientInfo,
  setTokens,
  clearTokens,
  ApiError,
  type OAuthAuthorizeParams
} from "@/lib/api";

type PageState = "loading" | "login" | "consent" | "redirecting" | "error";

interface ScopeDescription {
  icon: React.ElementType;
  title: string;
  description: string;
}

const SCOPE_DESCRIPTIONS: Record<string, ScopeDescription> = {
  "mcp:search": {
    icon: Search,
    title: "Search Documents",
    description: "Search across your documents",
  },
  "mcp:doc:read": {
    icon: FileText,
    title: "Read Document Content",
    description: "Read document content",
  },
  "mcp:documents:read": {
    icon: FileText,
    title: "Read Document Content",
    description: "Read document content",
  },
  "mcp:sources:list": {
    icon: List,
    title: "View Available Sources",
    description: "View available sources",
  },
};

function OAuthAuthorizeContent() {
  const searchParams = useSearchParams();
  const [state, setState] = useState<PageState>("loading");
  const [error, setError] = useState<string | null>(null);

  // OAuth params from URL
  const [oauthParams, setOAuthParams] = useState<OAuthAuthorizeParams | null>(null);

  // Login form state
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [clientName, setClientName] = useState<string | null>(null);

  useEffect(() => {
    // Read OAuth params from URL
    const response_type = searchParams.get("response_type");
    const client_id = searchParams.get("client_id");
    const redirect_uri = searchParams.get("redirect_uri");
    const scope = searchParams.get("scope");
    const urlState = searchParams.get("state");
    const code_challenge = searchParams.get("code_challenge");
    const code_challenge_method = searchParams.get("code_challenge_method");
    const resource = searchParams.get("resource");

    // Validate required params
    if (!response_type || !client_id || !redirect_uri) {
      setState("error");
      setError("Invalid authorization request - missing required parameters");
      return;
    }

    const params: OAuthAuthorizeParams = {
      response_type,
      client_id,
      redirect_uri,
      scope: scope || undefined,
      state: urlState || undefined,
      code_challenge: code_challenge || undefined,
      code_challenge_method: code_challenge_method || undefined,
      resource: resource || undefined,
    };

    setOAuthParams(params);

    // Fetch client display name
    getOAuthClientInfo(client_id)
      .then((info) => setClientName(info.name))
      .catch(() => {
        // Silently fail — UI will fall back to showing client_id
      });

    // Check if user is logged in with a valid token
    const token = typeof window !== "undefined" ? localStorage.getItem("sercha_token") : null;
    if (token) {
      // Validate token by calling /api/v1/me
      getCurrentUser()
        .then(() => setState("consent"))
        .catch(() => {
          clearTokens();
          setState("login");
        });
    } else {
      setState("login");
    }
  }, [searchParams]);

  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!oauthParams) return;

    setError(null);
    setIsSubmitting(true);

    try {
      const response = await apiLogin(email, password);
      setTokens(response.token, response.refresh_token);
      setState("consent");
    } catch (err) {
      if (err instanceof ApiError) {
        if (err.status === 401) {
          setError("Invalid email or password");
        } else {
          setError(err.message || "Login failed. Please try again.");
        }
      } else {
        setError("Unable to connect to server. Please check your connection.");
      }
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleAllow = async () => {
    if (!oauthParams) return;

    setIsSubmitting(true);
    setError(null);
    setState("redirecting");

    try {
      const response = await completeOAuthAuthorize(oauthParams);
      window.location.href = response.redirect_url;
    } catch (err) {
      setState("consent");
      if (err instanceof ApiError) {
        setError(err.message || "Authorization failed");
      } else {
        setError("An unexpected error occurred");
      }
      setIsSubmitting(false);
    }
  };

  const handleDeny = () => {
    if (!oauthParams) return;

    const url = new URL(oauthParams.redirect_uri);
    url.searchParams.set("error", "access_denied");
    if (oauthParams.state) {
      url.searchParams.set("state", oauthParams.state);
    }
    window.location.href = url.toString();
  };

  const getScopes = (): string[] => {
    if (!oauthParams?.scope) return [];
    return oauthParams.scope.split(" ").filter(Boolean);
  };

  if (state === "loading") {
    return (
      <div className="flex min-h-screen items-center justify-center bg-gradient-to-b from-sercha-snow to-sercha-mist">
        <Loader2 className="h-8 w-8 animate-spin text-sercha-indigo" />
      </div>
    );
  }

  if (state === "error") {
    return (
      <div className="flex min-h-screen flex-col items-center justify-center bg-gradient-to-b from-sercha-snow to-sercha-mist px-4">
        <div className="w-full max-w-md">
          <div className="mb-8 flex justify-center">
            <Image
              src="/logo-wordmark.png"
              alt="Sercha"
              width={180}
              height={48}
              className="h-12 w-auto"
              priority
            />
          </div>
          <div className="rounded-2xl border border-sercha-silverline bg-white p-8 shadow-sm">
            <div className="flex flex-col items-center text-center">
              <div className="mb-4 flex h-12 w-12 items-center justify-center rounded-full bg-red-100">
                <Shield className="h-6 w-6 text-red-600" />
              </div>
              <h1 className="text-xl font-semibold text-sercha-ink-slate">
                Authorization Error
              </h1>
              <p className="mt-2 text-sm text-sercha-fog-grey">{error}</p>
            </div>
          </div>
        </div>
      </div>
    );
  }

  if (state === "redirecting") {
    return (
      <div className="flex min-h-screen flex-col items-center justify-center bg-gradient-to-b from-sercha-snow to-sercha-mist px-4">
        <div className="w-full max-w-md">
          <div className="mb-8 flex justify-center">
            <Image
              src="/logo-wordmark.png"
              alt="Sercha"
              width={180}
              height={48}
              className="h-12 w-auto"
              priority
            />
          </div>
          <div className="rounded-2xl border border-sercha-silverline bg-white p-8 shadow-sm">
            <div className="flex flex-col items-center text-center">
              <Loader2 className="mb-4 h-12 w-12 animate-spin text-sercha-indigo" />
              <h1 className="text-xl font-semibold text-sercha-ink-slate">
                Redirecting...
              </h1>
              <p className="mt-2 text-sm text-sercha-fog-grey">
                Please wait while we complete the authorization.
              </p>
            </div>
          </div>
        </div>
      </div>
    );
  }

  if (state === "login") {
    return (
      <div className="flex min-h-screen flex-col items-center justify-center bg-gradient-to-b from-sercha-snow to-sercha-mist px-4">
        <div className="w-full max-w-md">
          {/* Logo */}
          <div className="mb-8 flex justify-center">
            <Image
              src="/logo-wordmark.png"
              alt="Sercha"
              width={180}
              height={48}
              className="h-12 w-auto"
              priority
            />
          </div>

          {/* Authorization Banner */}
          <div className="mb-4 rounded-xl border border-sercha-indigo/20 bg-sercha-indigo-soft p-4">
            <div className="flex items-start gap-3">
              <Shield className="mt-0.5 h-5 w-5 flex-shrink-0 text-sercha-indigo" />
              <div>
                <h2 className="text-sm font-medium text-sercha-indigo">
                  Authorization Request
                </h2>
                <p className="mt-1 text-xs text-sercha-indigo/80">
                  An application is requesting access to your Sercha account
                </p>
              </div>
            </div>
          </div>

          {/* Login Card */}
          <div className="rounded-2xl border border-sercha-silverline bg-white p-8 shadow-sm">
            <div className="mb-6 text-center">
              <h1 className="text-2xl font-semibold text-sercha-ink-slate">
                Sign in to continue
              </h1>
              <p className="mt-1 text-sm text-sercha-fog-grey">
                Sign in to authorize the application
              </p>
            </div>

            <form onSubmit={handleLogin} className="space-y-5">
              {/* Email Field */}
              <div>
                <label
                  htmlFor="email"
                  className="mb-1.5 block text-sm font-medium text-sercha-ink-slate"
                >
                  Email
                </label>
                <input
                  id="email"
                  type="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  className="w-full rounded-lg border border-sercha-silverline bg-white px-4 py-2.5 text-sm text-sercha-ink-slate placeholder:text-sercha-fog-grey focus:border-sercha-indigo focus:outline-none focus:ring-2 focus:ring-sercha-indigo/20"
                  placeholder="you@example.com"
                  required
                  autoComplete="email"
                  disabled={isSubmitting}
                />
              </div>

              {/* Password Field */}
              <div>
                <label
                  htmlFor="password"
                  className="mb-1.5 block text-sm font-medium text-sercha-ink-slate"
                >
                  Password
                </label>
                <div className="relative">
                  <input
                    id="password"
                    type={showPassword ? "text" : "password"}
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    className="w-full rounded-lg border border-sercha-silverline bg-white px-4 py-2.5 pr-10 text-sm text-sercha-ink-slate placeholder:text-sercha-fog-grey focus:border-sercha-indigo focus:outline-none focus:ring-2 focus:ring-sercha-indigo/20"
                    placeholder="Enter your password"
                    required
                    autoComplete="current-password"
                    disabled={isSubmitting}
                  />
                  <button
                    type="button"
                    onClick={() => setShowPassword(!showPassword)}
                    className="absolute right-3 top-1/2 -translate-y-1/2 text-sercha-fog-grey hover:text-sercha-ink-slate"
                    tabIndex={-1}
                  >
                    {showPassword ? <EyeOff size={18} /> : <Eye size={18} />}
                  </button>
                </div>
              </div>

              {/* Error Message */}
              {error && (
                <div className="rounded-lg bg-red-50 px-4 py-3 text-sm text-red-600">
                  {error}
                </div>
              )}

              {/* Submit Button */}
              <button
                type="submit"
                disabled={isSubmitting}
                className="flex w-full items-center justify-center rounded-lg bg-sercha-indigo px-4 py-2.5 text-sm font-medium text-white transition-colors hover:bg-sercha-indigo/90 focus:outline-none focus:ring-2 focus:ring-sercha-indigo/50 disabled:cursor-not-allowed disabled:opacity-60"
              >
                {isSubmitting ? (
                  <>
                    <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                    Signing in...
                  </>
                ) : (
                  "Sign in"
                )}
              </button>
            </form>
          </div>
        </div>
      </div>
    );
  }

  if (state === "consent") {
    const scopes = getScopes();
    const displayName = clientName || oauthParams?.client_id || "Unknown Application";

    return (
      <div className="flex min-h-screen flex-col items-center justify-center bg-gradient-to-b from-sercha-snow to-sercha-mist px-4">
        <div className="w-full max-w-md">
          {/* Logo */}
          <div className="mb-8 flex justify-center">
            <Image
              src="/logo-wordmark.png"
              alt="Sercha"
              width={180}
              height={48}
              className="h-12 w-auto"
              priority
            />
          </div>

          {/* Consent Card */}
          <div className="rounded-2xl border border-sercha-silverline bg-white p-8 shadow-sm">
            <div className="mb-6 text-center">
              <div className="mx-auto mb-4 flex h-12 w-12 items-center justify-center rounded-full bg-sercha-indigo-soft">
                <Shield className="h-6 w-6 text-sercha-indigo" />
              </div>
              <h1 className="text-xl font-semibold text-sercha-ink-slate">
                Authorize Application
              </h1>
              <p className="mt-2 text-sm text-sercha-fog-grey">
                <span className="font-medium text-sercha-ink-slate">{displayName}</span>
                {" "}is requesting access to your Sercha account
              </p>
            </div>

            {/* Permissions */}
            {scopes.length > 0 && (
              <div className="mb-6">
                <h2 className="mb-3 text-sm font-medium text-sercha-ink-slate">
                  This application will be able to:
                </h2>
                <div className="space-y-3">
                  {scopes.map((scope) => {
                    const info = SCOPE_DESCRIPTIONS[scope] || {
                      icon: Shield,
                      title: scope,
                      description: scope,
                    };
                    const Icon = info.icon;
                    return (
                      <div
                        key={scope}
                        className="flex items-start gap-3 rounded-lg bg-sercha-snow p-3"
                      >
                        <div className="flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-full bg-white">
                          <Icon className="h-4 w-4 text-sercha-indigo" />
                        </div>
                        <div className="min-w-0 flex-1">
                          <p className="text-sm font-medium text-sercha-ink-slate">
                            {info.title}
                          </p>
                          <p className="mt-0.5 text-xs text-sercha-fog-grey">
                            {info.description}
                          </p>
                        </div>
                      </div>
                    );
                  })}
                </div>
              </div>
            )}

            {/* Error Message */}
            {error && (
              <div className="mb-6 rounded-lg bg-red-50 px-4 py-3 text-sm text-red-600">
                {error}
              </div>
            )}

            {/* Action Buttons */}
            <div className="flex gap-3">
              <button
                onClick={handleDeny}
                disabled={isSubmitting}
                className="flex-1 rounded-lg border border-sercha-silverline bg-white px-4 py-2.5 text-sm font-medium text-sercha-ink-slate transition-colors hover:bg-sercha-snow focus:outline-none focus:ring-2 focus:ring-sercha-silverline disabled:cursor-not-allowed disabled:opacity-60"
              >
                Deny
              </button>
              <button
                onClick={handleAllow}
                disabled={isSubmitting}
                className="flex flex-1 items-center justify-center rounded-lg bg-sercha-indigo px-4 py-2.5 text-sm font-medium text-white transition-colors hover:bg-sercha-indigo/90 focus:outline-none focus:ring-2 focus:ring-sercha-indigo/50 disabled:cursor-not-allowed disabled:opacity-60"
              >
                {isSubmitting ? (
                  <>
                    <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                    Authorizing...
                  </>
                ) : (
                  "Allow"
                )}
              </button>
            </div>
          </div>
        </div>
      </div>
    );
  }

  return null;
}

export default function OAuthAuthorizePage() {
  return (
    <Suspense
      fallback={
        <div className="flex min-h-screen items-center justify-center bg-gradient-to-b from-sercha-snow to-sercha-mist">
          <Loader2 className="h-8 w-8 animate-spin text-sercha-indigo" />
        </div>
      }
    >
      <OAuthAuthorizeContent />
    </Suspense>
  );
}
