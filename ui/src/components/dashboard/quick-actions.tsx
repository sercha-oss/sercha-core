import Link from "next/link";
import { Plus, RefreshCw, Search } from "lucide-react";
import { cn } from "@/lib/utils";

interface QuickAction {
  label: string;
  href: string;
  icon: typeof Plus;
  variant?: "primary" | "secondary";
}

const actions: QuickAction[] = [
  { label: "Add Source", href: "/admin/sources/new", icon: Plus, variant: "primary" },
  { label: "Sync All", href: "#", icon: RefreshCw, variant: "secondary" },
  { label: "Go to Search", href: "/", icon: Search, variant: "secondary" },
];

export function QuickActions() {
  return (
    <div className="flex flex-wrap gap-3">
      {actions.map((action) => (
        <Link
          key={action.label}
          href={action.href}
          className={cn(
            "inline-flex items-center gap-2 rounded-full px-5 py-2.5 text-sm font-semibold transition-all",
            action.variant === "primary"
              ? "bg-sercha-indigo text-white hover:bg-sercha-indigo/90 hover:shadow-lg"
              : "border-2 border-sercha-silverline text-sercha-ink-slate hover:border-sercha-indigo hover:text-sercha-indigo"
          )}
        >
          <action.icon size={18} />
          {action.label}
        </Link>
      ))}
    </div>
  );
}
