"use client";

import { useEffect, useState, Suspense } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { Loader2, CheckCircle2, XCircle } from "lucide-react";
import Image from "next/image";
import { handleOAuthCallback, ApiError } from "@/lib/api";

type CallbackStatus = "loading" | "success" | "error";

function OAuthCallbackContent() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const [status, setStatus] = useState<CallbackStatus>("loading");
  const [error, setError] = useState<string | null>(null);
  const [connectionName, setInstallationName] = useState<string | null>(null);

  useEffect(() => {
    const processCallback = async () => {
      const code = searchParams.get("code");
      const state = searchParams.get("state");
      const errorParam = searchParams.get("error");
      const errorDescription = searchParams.get("error_description");

      // Check for OAuth error from provider
      if (errorParam) {
        setStatus("error");
        setError(errorDescription || errorParam || "Authorization was denied");
        return;
      }

      // Check for missing parameters
      if (!code || !state) {
        setStatus("error");
        setError("Invalid callback - missing authorization code or state");
        return;
      }

      // Retrieve the provider from sessionStorage (stored during authorize)
      const provider = sessionStorage.getItem("oauth_provider");
      if (!provider) {
        setStatus("error");
        setError("Session expired - please restart the authorization flow");
        return;
      }

      try {
        const result = await handleOAuthCallback(code, state, provider);
        setInstallationName(result.connection.name);
        setStatus("success");

        // Retrieve the return URL from state or default to sources page
        const storedReturnUrl = sessionStorage.getItem("oauth_return_url");

        // Clean up session storage
        sessionStorage.removeItem("oauth_return_url");
        sessionStorage.removeItem("oauth_provider");
        sessionStorage.removeItem("oauth_connection_pending");

        // Wait a moment to show success, then redirect
        setTimeout(() => {
          if (storedReturnUrl) {
            // Append connection_id to return URL
            const returnUrl = new URL(storedReturnUrl, window.location.origin);
            returnUrl.searchParams.set("connection_id", result.connection.id);
            router.push(returnUrl.pathname + returnUrl.search);
          } else {
            router.push("/admin/sources");
          }
        }, 1500);
      } catch (err) {
        setStatus("error");
        if (err instanceof ApiError) {
          setError(err.message || "Failed to complete authorization");
        } else {
          setError("An unexpected error occurred");
        }
      }
    };

    processCallback();
  }, [searchParams, router]);

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

        {/* Status Card */}
        <div className="rounded-2xl border border-sercha-silverline bg-white p-8 shadow-sm">
          {status === "loading" && (
            <div className="flex flex-col items-center text-center">
              <Loader2 className="mb-4 h-12 w-12 animate-spin text-sercha-indigo" />
              <h1 className="text-xl font-semibold text-sercha-ink-slate">
                Completing Authorization
              </h1>
              <p className="mt-2 text-sm text-sercha-fog-grey">
                Please wait while we connect your account...
              </p>
            </div>
          )}

          {status === "success" && (
            <div className="flex flex-col items-center text-center">
              <div className="mb-4 flex h-12 w-12 items-center justify-center rounded-full bg-emerald-100">
                <CheckCircle2 className="h-8 w-8 text-emerald-600" />
              </div>
              <h1 className="text-xl font-semibold text-sercha-ink-slate">
                Connected Successfully
              </h1>
              <p className="mt-2 text-sm text-sercha-fog-grey">
                {connectionName
                  ? `"${connectionName}" has been connected.`
                  : "Your account has been connected."}
              </p>
              <p className="mt-1 text-xs text-sercha-silverline">
                Redirecting...
              </p>
            </div>
          )}

          {status === "error" && (
            <div className="flex flex-col items-center text-center">
              <div className="mb-4 flex h-12 w-12 items-center justify-center rounded-full bg-red-100">
                <XCircle className="h-8 w-8 text-red-600" />
              </div>
              <h1 className="text-xl font-semibold text-sercha-ink-slate">
                Authorization Failed
              </h1>
              <p className="mt-2 text-sm text-sercha-fog-grey">{error}</p>
              <button
                onClick={() => router.push("/admin/sources")}
                className="mt-6 rounded-lg bg-sercha-indigo px-6 py-2.5 text-sm font-medium text-white transition-colors hover:bg-sercha-indigo/90"
              >
                Go to Sources
              </button>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

export default function OAuthCallbackPage() {
  return (
    <Suspense
      fallback={
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
                  Loading...
                </h1>
              </div>
            </div>
          </div>
        </div>
      }
    >
      <OAuthCallbackContent />
    </Suspense>
  );
}
