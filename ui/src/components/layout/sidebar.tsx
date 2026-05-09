"use client";

import Link from "next/link";
import Image from "next/image";
import { usePathname } from "next/navigation";
import {
  LayoutDashboard,
  Database,
  Settings,
  Users,
  ChevronLeft,
  ChevronRight,
  ChevronDown,
  Search,
  Sparkles,
  Zap,
  LucideIcon,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { useState } from "react";

interface NavSubItem {
  name: string;
  href: string;
  icon: LucideIcon;
}

interface NavItem {
  name: string;
  href?: string;
  icon: LucideIcon;
  children?: NavSubItem[];
}

const navigation: NavItem[] = [
  { name: "Dashboard", href: "/admin", icon: LayoutDashboard },
  { name: "Sources", href: "/admin/sources", icon: Database },
  { name: "Capabilities", href: "/admin/capabilities", icon: Zap },
  { name: "AI", href: "/admin/settings/ai", icon: Sparkles },
  { name: "Team", href: "/admin/team", icon: Users },
  { name: "Other", href: "/admin/settings", icon: Settings },
];

export function Sidebar() {
  const pathname = usePathname();
  const [collapsed, setCollapsed] = useState(false);
  const [expandedItems, setExpandedItems] = useState<string[]>([]);

  const toggleExpanded = (name: string) => {
    setExpandedItems((prev) =>
      prev.includes(name) ? prev.filter((n) => n !== name) : [...prev, name]
    );
  };

  const isChildActive = (item: NavItem) => {
    return item.children?.some((child) => pathname.startsWith(child.href));
  };

  return (
    <aside
      className={cn(
        "relative flex h-screen flex-col border-r border-sercha-silverline bg-white transition-all duration-200",
        collapsed ? "w-16" : "w-64"
      )}
    >
      {/* Collapse Toggle - positioned outside sidebar */}
      <button
        onClick={() => setCollapsed(!collapsed)}
        className="absolute -right-3 top-5 z-10 flex h-6 w-6 items-center justify-center rounded-full border border-sercha-silverline bg-white text-sercha-fog-grey shadow-sm hover:bg-sercha-mist hover:text-sercha-indigo"
      >
        {collapsed ? <ChevronRight size={14} /> : <ChevronLeft size={14} />}
      </button>

      {/* Logo */}
      <div className="flex h-16 items-center border-b border-sercha-silverline px-4">
        {collapsed ? (
          <Image
            src="/logo-icon-only.svg"
            alt="Sercha"
            width={28}
            height={28}
            className="mx-auto h-7 w-7"
          />
        ) : (
          <Image
            src="/logo-wordmark.png"
            alt="Sercha"
            width={100}
            height={28}
            className="h-7 w-auto"
          />
        )}
      </div>

      {/* Navigation */}
      <nav className="flex-1 space-y-1 p-3">
        {navigation.map((item) => {
          // Item with children (dropdown)
          if (item.children) {
            const isExpanded = expandedItems.includes(item.name);
            const hasActiveChild = isChildActive(item);

            return (
              <div key={item.name}>
                <button
                  onClick={() => toggleExpanded(item.name)}
                  className={cn(
                    "flex w-full items-center gap-3 rounded-lg px-3 py-2.5 text-sm font-medium transition-colors",
                    collapsed && "justify-center",
                    hasActiveChild
                      ? "bg-sercha-indigo-soft text-sercha-indigo"
                      : "text-sercha-fog-grey hover:bg-sercha-mist hover:text-sercha-ink-slate"
                  )}
                >
                  <item.icon size={20} />
                  {!collapsed && (
                    <>
                      <span className="flex-1 text-left">{item.name}</span>
                      <ChevronDown
                        size={16}
                        className={cn(
                          "transition-transform",
                          isExpanded && "rotate-180"
                        )}
                      />
                    </>
                  )}
                </button>
                {!collapsed && isExpanded && (
                  <div className="ml-4 mt-1 space-y-1 border-l border-sercha-mist pl-3">
                    {item.children.map((child) => {
                      const isActive = pathname.startsWith(child.href);
                      return (
                        <Link
                          key={child.name}
                          href={child.href}
                          className={cn(
                            "flex items-center gap-2 rounded-lg px-3 py-2 text-sm font-medium transition-colors",
                            isActive
                              ? "bg-sercha-indigo-soft text-sercha-indigo"
                              : "text-sercha-fog-grey hover:bg-sercha-mist hover:text-sercha-ink-slate"
                          )}
                        >
                          <child.icon size={16} />
                          <span>{child.name}</span>
                        </Link>
                      );
                    })}
                  </div>
                )}
              </div>
            );
          }

          // Regular item (no children)
          const isActive =
            pathname === item.href ||
            (item.href !== "/" && item.href !== "/admin" && pathname.startsWith(item.href!));

          return (
            <Link
              key={item.name}
              href={item.href!}
              className={cn(
                "flex items-center gap-3 rounded-lg px-3 py-2.5 text-sm font-medium transition-colors",
                collapsed && "justify-center",
                isActive
                  ? "bg-sercha-indigo-soft text-sercha-indigo"
                  : "text-sercha-fog-grey hover:bg-sercha-mist hover:text-sercha-ink-slate"
              )}
            >
              <item.icon size={20} />
              {!collapsed && <span>{item.name}</span>}
            </Link>
          );
        })}
      </nav>

      {/* Back to Search */}
      <div className="border-t border-sercha-silverline p-3">
        <Link
          href="/"
          className={cn(
            "flex items-center gap-3 rounded-lg px-3 py-2.5 text-sm font-medium text-sercha-fog-grey transition-colors hover:bg-sercha-mist hover:text-sercha-ink-slate",
            collapsed && "justify-center"
          )}
        >
          <Search size={20} />
          {!collapsed && <span>Back to Search</span>}
        </Link>
      </div>

      {/* Footer - only when expanded */}
      {!collapsed && (
        <div className="border-t border-sercha-silverline px-4 py-3">
          <p className="text-xs text-sercha-fog-grey">
            Sercha v{process.env.APP_VERSION}
          </p>
        </div>
      )}
    </aside>
  );
}
