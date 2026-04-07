"use client";

import { useState, useEffect, Suspense } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import Image from "next/image";
import Link from "next/link";
import { Loader2, CheckCircle } from "lucide-react";
import {
  ProgressBar,
  StepWelcome,
  StepAI,
  StepDataSources,
  StepComplete,
} from "@/components/setup";
import { getSetupStatus } from "@/lib/api";

function SetupWizardContent() {
  const router = useRouter();
  const searchParams = useSearchParams();

  const stepParam = searchParams.get("step");
  const initialStep = stepParam ? parseInt(stepParam, 10) : 1;

  const [currentStep, setCurrentStep] = useState(initialStep);
  const [completedSteps, setCompletedSteps] = useState<number[]>([]);
  const [checkingStatus, setCheckingStatus] = useState(true);
  const [setupAlreadyComplete, setSetupAlreadyComplete] = useState(false);

  // Check if setup is already complete
  useEffect(() => {
    const checkSetupStatus = async () => {
      try {
        const status = await getSetupStatus();
        if (status.setup_complete) {
          setSetupAlreadyComplete(true);
        }
      } catch (err) {
        // If we get an error (like 403), setup is likely already complete
        console.error("Failed to check setup status:", err);
        setSetupAlreadyComplete(true);
      } finally {
        setCheckingStatus(false);
      }
    };
    checkSetupStatus();
  }, []);

  // Sync URL with step
  useEffect(() => {
    const newParams = new URLSearchParams(searchParams.toString());
    newParams.set("step", currentStep.toString());

    // Clear OAuth params when reaching completion step
    if (currentStep === 4) {
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
          <StepDataSources
            connectionId={searchParams.get("connection_id") || undefined}
            provider={searchParams.get("provider") || undefined}
            onComplete={() => completeStep(3)}
            onSkip={() => skipStep(3)}
          />
        );
      case 4:
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

      {/* Progress Bar - Hide on step 4 (complete) */}
      {currentStep < 4 && (
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
