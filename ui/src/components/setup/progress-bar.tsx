"use client";

import { Check } from "lucide-react";
import { cn } from "@/lib/utils";

interface Step {
  number: number;
  label: string;
  optional?: boolean;
}

const SETUP_STEPS: Step[] = [
  { number: 1, label: "Account" },
  { number: 2, label: "AI", optional: true },
  { number: 3, label: "Sources", optional: true },
];

interface ProgressBarProps {
  currentStep: number;
  completedSteps: number[];
}

export function ProgressBar({ currentStep, completedSteps }: ProgressBarProps) {
  return (
    <div className="flex justify-center">
      <div className="flex items-start">
        {SETUP_STEPS.map((step, index) => {
          const isCompleted = completedSteps.includes(step.number);
          const isCurrent = currentStep === step.number;
          const isLast = index === SETUP_STEPS.length - 1;

          return (
            <div key={step.number} className="flex items-start">
              {/* Step */}
              <div className="flex w-16 flex-col items-center">
                <div
                  className={cn(
                    "flex h-8 w-8 items-center justify-center rounded-full text-sm font-medium transition-colors",
                    isCompleted
                      ? "bg-sercha-indigo text-white"
                      : isCurrent
                        ? "border-2 border-sercha-indigo bg-white text-sercha-indigo"
                        : "border-2 border-sercha-silverline bg-white text-sercha-fog-grey"
                  )}
                >
                  {isCompleted ? (
                    <Check size={16} strokeWidth={3} />
                  ) : (
                    step.number
                  )}
                </div>
                <div className="mt-2 flex flex-col items-center">
                  <span
                    className={cn(
                      "text-xs font-medium",
                      isCurrent || isCompleted
                        ? "text-sercha-ink-slate"
                        : "text-sercha-fog-grey"
                    )}
                  >
                    {step.label}
                  </span>
                  {step.optional && (
                    <span className="text-[10px] text-sercha-silverline">
                      Optional
                    </span>
                  )}
                </div>
              </div>

              {/* Connector Line */}
              {!isLast && (
                <div
                  className={cn(
                    "mt-4 h-0.5 w-12",
                    isCompleted ? "bg-sercha-indigo" : "bg-sercha-silverline"
                  )}
                />
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}
