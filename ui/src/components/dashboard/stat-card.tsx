import { type LucideIcon } from "lucide-react";
import { cn } from "@/lib/utils";

interface StatCardProps {
  title: string;
  value: string | number;
  icon: LucideIcon;
  trend?: {
    value: number;
    label: string;
  };
  className?: string;
}

export function StatCard({ title, value, icon: Icon, trend, className }: StatCardProps) {
  return (
    <div
      className={cn(
        "rounded-2xl border-2 border-sercha-silverline bg-white p-6",
        className
      )}
    >
      <div className="flex items-start justify-between">
        <div>
          <p className="text-sm font-medium text-sercha-fog-grey">{title}</p>
          <p className="mt-2 text-3xl font-bold text-sercha-ink-slate">{value}</p>
          {trend && (
            <p
              className={cn(
                "mt-1 text-sm",
                trend.value >= 0 ? "text-emerald-500" : "text-red-500"
              )}
            >
              {trend.value >= 0 ? "+" : ""}
              {trend.value}% {trend.label}
            </p>
          )}
        </div>
        <div className="rounded-xl bg-sercha-indigo-soft p-3">
          <Icon className="h-6 w-6 text-sercha-indigo" />
        </div>
      </div>
    </div>
  );
}
