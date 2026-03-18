"use client";

import { useState } from "react";
import Image from "next/image";
import Link from "next/link";
import { Eye, EyeOff, Loader2 } from "lucide-react";
import { useAuth } from "@/lib/auth";
import { ApiError } from "@/lib/api";

export default function LoginPage() {
  const { login, isLoading: authLoading } = useAuth();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setIsSubmitting(true);

    try {
      await login(email, password);
      // Redirect happens in the auth context
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

  // Show loading while checking auth state
  if (authLoading) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <Loader2 className="h-8 w-8 animate-spin text-sercha-indigo" />
      </div>
    );
  }

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

        {/* Login Card */}
        <div className="rounded-2xl border border-sercha-silverline bg-white p-8 shadow-sm">
          <div className="mb-6 text-center">
            <h1 className="text-2xl font-semibold text-sercha-ink-slate">
              Welcome back
            </h1>
            <p className="mt-1 text-sm text-sercha-fog-grey">
              Sign in to your account
            </p>
          </div>

          <form onSubmit={handleSubmit} className="space-y-5">
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

        {/* Setup Link */}
        <p className="mt-6 text-center text-sm text-sercha-fog-grey">
          First time setup?{" "}
          <Link
            href="/setup"
            className="font-medium text-sercha-indigo hover:underline"
          >
            Configure Sercha
          </Link>
        </p>
      </div>
    </div>
  );
}
