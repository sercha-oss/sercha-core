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
  Server,
  LucideIcon,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { useState, useEffect } from "react";

interface NavItem {
  name: string;
  href: string;
  icon: LucideIcon;
  children?: { name: string; href: string }[];
}

const navigation: NavItem[] = [
  { name: "Dashboard", href: "/admin", icon: LayoutDashboard },
  { name: "Sources", href: "/admin/sources", icon: Database },
  { name: "Vespa", href: "/admin/vespa", icon: Server },
  {
    name: "Settings",
    href: "/admin/settings",
    icon: Settings,
    children: [
      { name: "General", href: "/admin/settings" },
      { name: "Sync", href: "/admin/settings/sync" },
      { name: "OAuth", href: "/admin/settings/oauth" },
      { name: "AI", href: "/admin/settings/ai" },
    ],
  },
  { name: "Team", href: "/admin/team", icon: Users },
];

export function Sidebar() {
  const pathname = usePathname();
  const [collapsed, setCollapsed] = useState(false);
  const [settingsExpanded, setSettingsExpanded] = useState(false);

  // Auto-expand Settings when on a settings page
  useEffect(() => {
    if (pathname.startsWith("/admin/settings")) {
      setSettingsExpanded(true);
    }
  }, [pathname]);

  // When clicking Settings in collapsed mode, expand the sidebar
  const handleSettingsClick = (item: NavItem) => {
    if (item.children) {
      if (collapsed) {
        setCollapsed(false);
        setSettingsExpanded(true);
      } else {
        setSettingsExpanded(!settingsExpanded);
      }
    }
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
          const isActive =
            pathname === item.href ||
            (item.href !== "/" && item.href !== "/admin" && pathname.startsWith(item.href));

          // Items with children (Settings) render differently
          if (item.children) {
            const isChildActive = item.children.some(
              (child) =>
                pathname === child.href ||
                (child.href !== "/admin/settings" && pathname.startsWith(child.href))
            );

            return (
              <div key={item.name}>
                {/* Parent item - clickable to expand/collapse */}
                <button
                  onClick={() => handleSettingsClick(item)}
                  className={cn(
                    "flex w-full items-center gap-3 rounded-lg px-3 py-2.5 text-sm font-medium transition-colors",
                    collapsed && "justify-center",
                    isChildActive
                      ? "text-sercha-indigo"
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
                          settingsExpanded && "rotate-180"
                        )}
                      />
                    </>
                  )}
                </button>

                {/* Children - only show when expanded and sidebar not collapsed */}
                {settingsExpanded && !collapsed && (
                  <div className="ml-6 mt-1 space-y-1 border-l border-sercha-silverline pl-3">
                    {item.children.map((child) => {
                      const isSubActive = pathname === child.href;
                      return (
                        <Link
                          key={child.href}
                          href={child.href}
                          className={cn(
                            "block rounded-lg px-3 py-2 text-sm font-medium transition-colors",
                            isSubActive
                              ? "bg-sercha-indigo-soft text-sercha-indigo"
                              : "text-sercha-fog-grey hover:bg-sercha-mist hover:text-sercha-ink-slate"
                          )}
                        >
                          {child.name}
                        </Link>
                      );
                    })}
                  </div>
                )}
              </div>
            );
          }

          // Regular items without children
          return (
            <Link
              key={item.name}
              href={item.href}
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
