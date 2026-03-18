"use client";

import { Sidebar } from "./sidebar";
import { Header } from "./header";
import { useRequireAuth } from "@/lib/auth";
import { Loader2 } from "lucide-react";

interface AdminLayoutProps {
  children: React.ReactNode;
  title: string;
  description?: string;
}

export function AdminLayout({ children, title, description }: AdminLayoutProps) {
  const { isLoading, isAdmin } = useRequireAuth(true);

  // Show loading while checking auth
  if (isLoading) {
    return (
      <div className="flex h-screen items-center justify-center bg-sercha-snow">
        <Loader2 className="h-8 w-8 animate-spin text-sercha-indigo" />
      </div>
    );
  }

  // Don't render admin content for non-admins (redirect happens in useRequireAuth)
  if (!isAdmin) {
    return (
      <div className="flex h-screen items-center justify-center bg-sercha-snow">
        <Loader2 className="h-8 w-8 animate-spin text-sercha-indigo" />
      </div>
    );
  }

  return (
    <div className="flex h-screen bg-sercha-snow">
      <Sidebar />
      <div className="flex flex-1 flex-col overflow-hidden">
        <Header title={title} description={description} />
        <main className="flex-1 overflow-auto p-6">{children}</main>
      </div>
    </div>
  );
}
