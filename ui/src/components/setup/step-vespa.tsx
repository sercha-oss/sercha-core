"use client";

import { useState, useEffect } from "react";
import { Loader2, CheckCircle2, XCircle, Server, Info } from "lucide-react";
import { connectVespa, getVespaStatus, ApiError, type VespaStatus } from "@/lib/api";

interface StepVespaProps {
  onComplete: () => void;
}

type ConnectionStatus = "idle" | "connecting" | "connected" | "error";

export function StepVespa({ onComplete }: StepVespaProps) {
  const [endpoint, setEndpoint] = useState("http://vespa:19071");
  const [devMode, setDevMode] = useState(true);
  const [status, setStatus] = useState<ConnectionStatus>("idle");
  const [vespaStatus, setVespaStatus] = useState<VespaStatus | null>(null);
  const [error, setError] = useState<string | null>(null);

  // Check if Vespa is already connected on mount
  useEffect(() => {
    const checkExisting = async () => {
      try {
        const existing = await getVespaStatus();
        if (existing.connected && existing.healthy) {
          setVespaStatus(existing);
          setEndpoint(existing.endpoint);
          setDevMode(existing.dev_mode);
          setStatus("connected");
        }
      } catch {
        // Not connected yet, that's fine
      }
    };
    checkExisting();
  }, []);

  const handleConnect = async () => {
    setStatus("connecting");
    setError(null);

    try {
      const result = await connectVespa({ endpoint, dev_mode: devMode });
      setVespaStatus(result);

      if (result.connected && result.healthy) {
        setStatus("connected");
      } else {
        setStatus("error");
        setError("Vespa connected but is not healthy. Please check the service.");
      }
    } catch (err) {
      setStatus("error");
      if (err instanceof ApiError) {
        setError(err.message || "Failed to connect to Vespa");
      } else {
        setError("Unable to connect. Please check the endpoint and try again.");
      }
    }
  };

  const isConnected = status === "connected";

  return (
    <div className="mx-auto max-w-lg">
      <div className="mb-8 text-center">
        <h1 className="text-2xl font-semibold text-sercha-ink-slate">
          Connect Vespa
        </h1>
        <p className="mt-2 text-sm text-sercha-fog-grey">
          Vespa is the search engine that powers Sercha. Connect to your Vespa
          instance to enable search functionality.
        </p>
      </div>

      {/* Info Card */}
      <div className="mb-6 rounded-lg border border-sercha-indigo/20 bg-sercha-indigo/5 p-4">
        <div className="flex gap-3">
          <Info className="mt-0.5 h-5 w-5 flex-shrink-0 text-sercha-indigo" />
          <div className="text-sm text-sercha-fog-grey">
            <p className="font-medium text-sercha-ink-slate">What is Vespa?</p>
            <p className="mt-1">
              Vespa is a scalable search and vector database. In development
              mode, it runs as a single container. For production, it can be
              deployed as a distributed cluster.
            </p>
          </div>
        </div>
      </div>

      {/* Connection Form */}
      <div className="space-y-5">
        {/* Endpoint Input */}
        <div>
          <label
            htmlFor="endpoint"
            className="mb-1.5 block text-sm font-medium text-sercha-ink-slate"
          >
            Vespa Endpoint
          </label>
          <input
            id="endpoint"
            type="text"
            value={endpoint}
            onChange={(e) => setEndpoint(e.target.value)}
            className="w-full rounded-lg border border-sercha-silverline bg-white px-4 py-2.5 text-sm text-sercha-ink-slate placeholder:text-sercha-fog-grey focus:border-sercha-indigo focus:outline-none focus:ring-2 focus:ring-sercha-indigo/20 disabled:bg-sercha-mist disabled:text-sercha-fog-grey"
            placeholder="http://vespa:19071"
            disabled={status === "connecting" || isConnected}
          />
        </div>

        {/* Dev Mode Toggle */}
        <div className="flex items-center justify-between rounded-lg border border-sercha-silverline bg-white px-4 py-3">
          <div>
            <p className="text-sm font-medium text-sercha-ink-slate">
              Development Mode
            </p>
            <p className="text-xs text-sercha-fog-grey">
              Single-container mode for local development
            </p>
          </div>
          <button
            type="button"
            role="switch"
            aria-checked={devMode}
            onClick={() => setDevMode(!devMode)}
            disabled={status === "connecting" || isConnected}
            className={`relative h-6 w-11 flex-shrink-0 rounded-full transition-colors ${
              devMode ? "bg-sercha-indigo" : "bg-sercha-silverline"
            } disabled:opacity-50`}
          >
            <span
              className={`absolute left-0.5 top-0.5 h-5 w-5 rounded-full bg-white shadow transition-transform ${
                devMode ? "translate-x-5" : "translate-x-0"
              }`}
            />
          </button>
        </div>

        {/* Status Display */}
        {status !== "idle" && (
          <div
            className={`flex items-center gap-3 rounded-lg p-4 ${
              status === "connecting"
                ? "border border-sercha-silverline bg-sercha-mist"
                : status === "connected"
                  ? "border border-emerald-200 bg-emerald-50"
                  : "border border-red-200 bg-red-50"
            }`}
          >
            {status === "connecting" && (
              <>
                <Loader2 className="h-5 w-5 animate-spin text-sercha-indigo" />
                <span className="text-sm text-sercha-ink-slate">
                  Connecting to Vespa...
                </span>
              </>
            )}
            {status === "connected" && (
              <>
                <CheckCircle2 className="h-5 w-5 text-emerald-600" />
                <div>
                  <span className="text-sm font-medium text-emerald-700">
                    Connected
                  </span>
                  {vespaStatus && (
                    <p className="text-xs text-emerald-600">
                      Mode: {vespaStatus.schema_mode} | Embeddings:{" "}
                      {vespaStatus.embeddings_enabled ? "Enabled" : "Disabled"}
                    </p>
                  )}
                </div>
              </>
            )}
            {status === "error" && (
              <>
                <XCircle className="h-5 w-5 text-red-600" />
                <span className="text-sm text-red-700">{error}</span>
              </>
            )}
          </div>
        )}

        {/* Error Message */}
        {error && status !== "error" && (
          <div className="rounded-lg bg-red-50 px-4 py-3 text-sm text-red-600">
            {error}
          </div>
        )}

        {/* Action Buttons */}
        <div className="flex gap-3">
          {!isConnected ? (
            <button
              onClick={handleConnect}
              disabled={status === "connecting" || !endpoint}
              className="flex flex-1 items-center justify-center rounded-lg bg-sercha-indigo px-4 py-2.5 text-sm font-medium text-white transition-colors hover:bg-sercha-indigo/90 focus:outline-none focus:ring-2 focus:ring-sercha-indigo/50 disabled:cursor-not-allowed disabled:opacity-60"
            >
              {status === "connecting" ? (
                <>
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                  Connecting...
                </>
              ) : (
                <>
                  <Server className="mr-2 h-4 w-4" />
                  Connect Vespa
                </>
              )}
            </button>
          ) : (
            <button
              onClick={onComplete}
              className="flex flex-1 items-center justify-center rounded-lg bg-sercha-indigo px-4 py-2.5 text-sm font-medium text-white transition-colors hover:bg-sercha-indigo/90 focus:outline-none focus:ring-2 focus:ring-sercha-indigo/50"
            >
              Continue
            </button>
          )}
        </div>
      </div>
    </div>
  );
}
