import { CheckCircle2, XCircle, AlertCircle, Loader2 } from "lucide-react";
import { cn } from "@/lib/utils";

type HealthStatus = "healthy" | "degraded" | "unhealthy" | "loading";

interface HealthCardProps {
  title: string;
  status: HealthStatus;
  message?: string;
  className?: string;
}

const statusConfig: Record<
  HealthStatus,
  { icon: typeof CheckCircle2; color: string; bg: string; label: string }
> = {
  healthy: {
    icon: CheckCircle2,
    color: "text-emerald-500",
    bg: "bg-emerald-50",
    label: "Healthy",
  },
  degraded: {
    icon: AlertCircle,
    color: "text-amber-500",
    bg: "bg-amber-50",
    label: "Degraded",
  },
  unhealthy: {
    icon: XCircle,
    color: "text-red-500",
    bg: "bg-red-50",
    label: "Unhealthy",
  },
  loading: {
    icon: Loader2,
    color: "text-sercha-fog-grey",
    bg: "bg-sercha-mist",
    label: "Checking...",
  },
};

export function HealthCard({ title, status, message, className }: HealthCardProps) {
  const config = statusConfig[status];
  const Icon = config.icon;

  return (
    <div
      className={cn(
        "flex items-center gap-4 rounded-xl border border-sercha-silverline bg-white p-4",
        className
      )}
    >
      <div className={cn("rounded-lg p-2", config.bg)}>
        <Icon
          className={cn(
            "h-5 w-5",
            config.color,
            status === "loading" && "animate-spin"
          )}
        />
      </div>
      <div className="flex-1">
        <p className="font-medium text-sercha-ink-slate">{title}</p>
        <p className={cn("text-sm", config.color)}>{message || config.label}</p>
      </div>
    </div>
  );
}
