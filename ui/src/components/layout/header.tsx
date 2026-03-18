"use client";

import { useState } from "react";
import { Bell, User, LogOut, ChevronDown } from "lucide-react";
import { useAuth } from "@/lib/auth";

interface HeaderProps {
  title: string;
  description?: string;
}

export function Header({ title, description }: HeaderProps) {
  const { user, logout } = useAuth();
  const [userMenuOpen, setUserMenuOpen] = useState(false);

  return (
    <header className="flex h-16 items-center justify-between border-b border-sercha-silverline bg-white px-6">
      <div>
        <h1 className="text-xl font-bold text-sercha-ink-slate">{title}</h1>
        {description && (
          <p className="text-sm text-sercha-fog-grey">{description}</p>
        )}
      </div>

      <div className="flex items-center gap-2">
        <button className="rounded-lg p-2 text-sercha-fog-grey hover:bg-sercha-mist hover:text-sercha-indigo">
          <Bell size={20} />
        </button>

        {/* User Menu */}
        <div className="relative">
          <button
            onClick={() => setUserMenuOpen(!userMenuOpen)}
            className="flex items-center gap-1 rounded-lg px-2 py-2 text-sercha-fog-grey hover:bg-sercha-mist hover:text-sercha-indigo"
          >
            <User size={20} />
            <ChevronDown size={14} />
          </button>

          {userMenuOpen && (
            <>
              <div className="fixed inset-0 z-10" onClick={() => setUserMenuOpen(false)} />
              <div className="absolute right-0 z-20 mt-1 w-56 rounded-lg border border-sercha-silverline bg-white py-1 shadow-lg">
                {/* User info */}
                <div className="border-b border-sercha-mist px-4 py-3">
                  <p className="text-sm font-medium text-sercha-ink-slate">{user?.name}</p>
                  <p className="text-xs text-sercha-fog-grey">{user?.email}</p>
                </div>
                {/* Logout */}
                <button
                  onClick={() => {
                    logout();
                    setUserMenuOpen(false);
                  }}
                  className="flex w-full items-center gap-2 px-4 py-2 text-sm text-red-600 hover:bg-red-50"
                >
                  <LogOut size={16} />
                  Logout
                </button>
              </div>
            </>
          )}
        </div>
      </div>
    </header>
  );
}
