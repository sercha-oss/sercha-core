import type { Metadata } from "next";
import { AuthProvider } from "@/lib/auth";
import "./globals.css";

export const metadata: Metadata = {
  title: "Sercha",
  description: "Enterprise Search Platform",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en">
      <body className="min-h-screen bg-sercha-snow antialiased">
        <AuthProvider>{children}</AuthProvider>
      </body>
    </html>
  );
}
