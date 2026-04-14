"use client";

import { useState, useEffect, Suspense } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import Image from "next/image";
import Link from "next/link";
import { Loader2, CheckCircle, AlertCircle } from "lucide-react";
import {
  ProgressBar,
  StepWelcome,
  StepAI,
  StepCapabilities,
  StepDataSources,
  StepComplete,
} from "@/components/setup";
import { getSetupStatus, ApiError } from "@/lib/api";

function SetupWizardContent() {
  const router = useRouter();
  const searchParams = useSearchParams();

  const stepParam = searchParams.get("step");
  const initialStep = stepParam ? parseInt(stepParam, 10) : 1;

  const [currentStep, setCurrentStep] = useState(initialStep);
  const [completedSteps, setCompletedSteps] = useState<number[]>([]);
  const [checkingStatus, setCheckingStatus] = useState(true);
  const [setupAlreadyComplete, setSetupAlreadyComplete] = useState(false);
  const [setupCheckError, setSetupCheckError] = useState<string | null>(null);

  // Check if setup is already complete — but only gate on step 1.
  // If URL has step > 1 or connection_id, the user is mid-FTUE (e.g. returning
  // from OAuth redirect) and must not be blocked by the "already complete" screen.
  useEffect(() => {
    const isMidFlow = initialStep > 1 || searchParams.get("connection_id");

    if (isMidFlow) {
      // User is mid-setup — skip the gate, let them continue
      setCheckingStatus(false);
      return;
    }

    const checkSetupStatus = async () => {
      try {
        const status = await getSetupStatus();
        if (status.setup_complete) {
          setSetupAlreadyComplete(true);
        }
      } catch (err) {
        if (err instanceof ApiError && (err.status === 403 || err.status === 409)) {
          // Server explicitly says setup is done
          setSetupAlreadyComplete(true);
        } else {
          // Network error, server down, etc. — don't assume setup is complete
          console.error("Failed to check setup status:", err);
          setSetupCheckError("Unable to connect to the server. Please check that the backend is running.");
        }
      } finally {
        setCheckingStatus(false);
      }
    };
    checkSetupStatus();
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  // Sync URL with step
  useEffect(() => {
    const newParams = new URLSearchParams(searchParams.toString());
    newParams.set("step", currentStep.toString());

    // Clear OAuth params when reaching completion step
    if (currentStep === 5) {
      newParams.delete("connection_id");
      newParams.delete("provider");
    }

    router.replace(`/setup?${newParams.toString()}`, { scroll: false });
  }, [currentStep, router, searchParams]);

  const goToStep = (step: number) => {
    setCurrentStep(step);
  };

  const completeStep = (step: number) => {
    if (!completedSteps.includes(step)) {
      setCompletedSteps([...completedSteps, step]);
    }
    goToStep(step + 1);
  };

  const skipStep = (step: number) => {
    goToStep(step + 1);
  };

  const renderStep = () => {
    switch (currentStep) {
      case 1:
        return <StepWelcome onComplete={() => completeStep(1)} />;
      case 2:
        return (
          <StepAI
            onComplete={() => completeStep(2)}
            onSkip={() => skipStep(2)}
          />
        );
      case 3:
        return (
          <StepCapabilities
            onComplete={() => completeStep(3)}
            onSkip={() => skipStep(3)}
          />
        );
      case 4:
        return (
          <StepDataSources
            connectionId={searchParams.get("connection_id") || undefined}
            provider={searchParams.get("provider") || undefined}
            onComplete={() => completeStep(4)}
            onSkip={() => skipStep(4)}
          />
        );
      case 5:
        return <StepComplete completedSteps={completedSteps} />;
      default:
        return <StepWelcome onComplete={() => completeStep(1)} />;
    }
  };

  // Show loading while checking setup status
  if (checkingStatus) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <Loader2 className="h-8 w-8 animate-spin text-sercha-indigo" />
      </div>
    );
  }

  // Show error if setup status check failed (network/server error)
  if (setupCheckError) {
    return (
      <div className="flex min-h-screen flex-col items-center justify-center px-4">
        <Image
          src="/logo-wordmark.png"
          alt="Sercha"
          width={140}
          height={36}
          className="mb-8 h-9 w-auto"
          priority
        />
        <div className="flex flex-col items-center rounded-2xl border border-sercha-silverline bg-white p-8 text-center shadow-sm">
          <div className="mb-4 rounded-full bg-red-100 p-3">
            <AlertCircle className="h-8 w-8 text-red-600" />
          </div>
          <h2 className="mb-2 text-xl font-semibold text-sercha-ink-slate">
            Connection Error
          </h2>
          <p className="mb-6 text-sercha-fog-grey">
            {setupCheckError}
          </p>
          <button
            onClick={() => window.location.reload()}
            className="rounded-full bg-sercha-indigo px-6 py-2.5 text-sm font-semibold text-white transition-all hover:bg-sercha-indigo/90"
          >
            Retry
          </button>
        </div>
      </div>
    );
  }

  // Show message if setup is already complete
  if (setupAlreadyComplete) {
    return (
      <div className="flex min-h-screen flex-col items-center justify-center px-4">
        <Image
          src="/logo-wordmark.png"
          alt="Sercha"
          width={140}
          height={36}
          className="mb-8 h-9 w-auto"
          priority
        />
        <div className="flex flex-col items-center rounded-2xl border border-sercha-silverline bg-white p-8 text-center shadow-sm">
          <div className="mb-4 rounded-full bg-emerald-100 p-3">
            <CheckCircle className="h-8 w-8 text-emerald-600" />
          </div>
          <h2 className="mb-2 text-xl font-semibold text-sercha-ink-slate">
            Setup Already Complete
          </h2>
          <p className="mb-6 text-sercha-fog-grey">
            Sercha has already been configured. Please log in to continue.
          </p>
          <Link
            href="/login"
            className="rounded-full bg-sercha-indigo px-6 py-2.5 text-sm font-semibold text-white transition-all hover:bg-sercha-indigo/90"
          >
            Go to Login
          </Link>
        </div>
      </div>
    );
  }

  return (
    <div className="flex min-h-screen flex-col px-4 py-8">
      {/* Header */}
      <div className="mx-auto mb-8 w-full max-w-3xl">
        <div className="flex justify-center">
          <Image
            src="/logo-wordmark.png"
            alt="Sercha"
            width={140}
            height={36}
            className="h-9 w-auto"
            priority
          />
        </div>
      </div>

      {/* Progress Bar - Hide on step 5 (complete) */}
      {currentStep < 5 && (
        <div className="mx-auto mb-12 w-full max-w-2xl">
          <ProgressBar
            currentStep={currentStep}
            completedSteps={completedSteps}
          />
        </div>
      )}

      {/* Step Content */}
      <div className="mx-auto w-full max-w-3xl flex-1">{renderStep()}</div>

      {/* Footer */}
      <div className="mx-auto mt-8 w-full max-w-3xl text-center">
        <p className="text-xs text-sercha-silverline">
          Need help?{" "}
          <a
            href="https://docs.sercha.dev"
            target="_blank"
            rel="noopener noreferrer"
            className="text-sercha-fog-grey hover:underline"
          >
            View documentation
          </a>
        </p>
      </div>
    </div>
  );
}

export default function SetupPage() {
  return (
    <Suspense
      fallback={
        <div className="flex min-h-screen items-center justify-center">
          <Loader2 className="h-8 w-8 animate-spin text-sercha-indigo" />
        </div>
      }
    >
      <SetupWizardContent />
    </Suspense>
  );
}
