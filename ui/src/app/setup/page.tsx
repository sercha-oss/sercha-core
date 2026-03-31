"use client";

import { useState, useEffect, Suspense } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import Image from "next/image";
import { Loader2 } from "lucide-react";
import {
  ProgressBar,
  StepWelcome,
  StepVespa,
  StepAI,
  StepDataSources,
  StepComplete,
} from "@/components/setup";

function SetupWizardContent() {
  const router = useRouter();
  const searchParams = useSearchParams();

  const stepParam = searchParams.get("step");
  const initialStep = stepParam ? parseInt(stepParam, 10) : 1;

  const [currentStep, setCurrentStep] = useState(initialStep);
  const [completedSteps, setCompletedSteps] = useState<number[]>([]);

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
        return <StepVespa onComplete={() => completeStep(2)} />;
      case 3:
        return (
          <StepAI
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
