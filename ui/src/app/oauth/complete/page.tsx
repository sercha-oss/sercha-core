"use client";

import { useEffect, useState, Suspense } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { Loader2, CheckCircle2, XCircle } from "lucide-react";
import Image from "next/image";
import { getConnection, ApiError } from "@/lib/api";

type CompletionStatus = "loading" | "success" | "error";

function OAuthCompleteContent() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const [status, setStatus] = useState<CompletionStatus>("loading");
  const [error, setError] = useState<string | null>(null);
  const [connectionName, setInstallationName] = useState<string | null>(null);

  useEffect(() => {
    const processCompletion = async () => {
      // Read params from URL (set by API redirect)
      const installationId = searchParams.get("connection_id");
      const provider = searchParams.get("provider");
      const returnContext = searchParams.get("return_context");
      const errorCode = searchParams.get("error");
      const errorDescription = searchParams.get("error_description");

      // Check for error from OAuth flow
      if (errorCode) {
        setStatus("error");
        setError(errorDescription || errorCode || "Authorization failed");
        return;
      }

      // Check for missing success params
      if (!installationId) {
        setStatus("error");
        setError("Invalid callback - missing installation information");
        return;
      }

      try {
        // Fetch installation details to display name
        const installation = await getConnection(installationId);
        setInstallationName(installation.name);
        setStatus("success");

        // Wait a moment to show success, then redirect based on return_context
        setTimeout(() => {
          if (returnContext === "setup") {
            // User started OAuth from FTUE - go back to setup step 4
            router.push(`/setup?step=4&connection_id=${installationId}&provider=${provider || ""}`);
          } else if (returnContext === "admin-sources") {
            // User started OAuth from admin add source flow - go back to source wizard
            router.push(`/admin/sources/new?connection_id=${installationId}&provider=${provider || ""}`);
          } else {
            // Default: go to admin sources list
            router.push(`/admin/sources?connection_id=${installationId}`);
          }
        }, 1500);
      } catch (err) {
        setStatus("error");
        if (err instanceof ApiError) {
          if (err.status === 401) {
            // Not authenticated - go to login
            setError("Session expired - please log in again");
            setTimeout(() => router.push("/login"), 2000);
            return;
          }
          setError(err.message || "Failed to complete authorization");
        } else {
          setError("An unexpected error occurred");
        }
      }
    };

    processCompletion();
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
                Please wait while we finalize your connection...
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
                onClick={() => {
                  const returnContext = searchParams.get("return_context");
                  if (returnContext === "setup") {
                    router.push("/setup?step=4");
                  } else {
                    router.push("/admin/sources");
                  }
                }}
                className="mt-6 rounded-lg bg-sercha-indigo px-6 py-2.5 text-sm font-medium text-white transition-colors hover:bg-sercha-indigo/90"
              >
                {searchParams.get("return_context") === "setup" ? "Back to Setup" : "Go to Sources"}
              </button>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

export default function OAuthCompletePage() {
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
      <OAuthCompleteContent />
    </Suspense>
  );
}
