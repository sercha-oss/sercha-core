"use client";

import {
  createContext,
  useContext,
  useEffect,
  useState,
  useCallback,
  type ReactNode,
} from "react";
import { useRouter, usePathname } from "next/navigation";
import {
  login as apiLogin,
  logout as apiLogout,
  getCurrentUser,
  clearTokens,
  type UserSummary,
  type LoginResponse,
  ApiError,
} from "./api";

interface AuthContextType {
  user: UserSummary | null;
  isLoading: boolean;
  isAuthenticated: boolean;
  isAdmin: boolean;
  login: (email: string, password: string) => Promise<LoginResponse>;
  logout: () => Promise<void>;
  refreshUser: () => Promise<void>;
}

const AuthContext = createContext<AuthContextType | null>(null);

// Public routes that don't require authentication
const PUBLIC_ROUTES = ["/login", "/setup", "/oauth/callback", "/oauth/complete", "/oauth/authorize"];

// Check if a path is public
function isPublicRoute(path: string): boolean {
  return PUBLIC_ROUTES.some((route) => path.startsWith(route));
}

interface AuthProviderProps {
  children: ReactNode;
}

export function AuthProvider({ children }: AuthProviderProps) {
  const [user, setUser] = useState<UserSummary | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const router = useRouter();
  const pathname = usePathname();

  // Check if we have a token and fetch user
  const checkAuth = useCallback(async () => {
    const token =
      typeof window !== "undefined"
        ? localStorage.getItem("sercha_token")
        : null;

    if (!token) {
      setUser(null);
      setIsLoading(false);
      return;
    }

    try {
      const currentUser = await getCurrentUser();
      setUser(currentUser);
    } catch (error) {
      // Token invalid or expired
      if (error instanceof ApiError && error.status === 401) {
        clearTokens();
      }
      setUser(null);
    } finally {
      setIsLoading(false);
    }
  }, []);

  // Initial auth check
  useEffect(() => {
    checkAuth();
  }, [checkAuth]);

  // Handle route protection
  useEffect(() => {
    if (isLoading) return;

    const isPublic = isPublicRoute(pathname);

    if (!user && !isPublic) {
      // Not authenticated and trying to access protected route
      router.push("/login");
    }
  }, [user, isLoading, pathname, router]);

  const login = useCallback(
    async (email: string, password: string): Promise<LoginResponse> => {
      const response = await apiLogin(email, password);
      setUser(response.user);

      // Redirect based on role
      if (response.user.role === "admin") {
        router.push("/admin");
      } else {
        router.push("/");
      }

      return response;
    },
    [router]
  );

  const logout = useCallback(async () => {
    try {
      await apiLogout();
    } catch {
      // Ignore errors, just clear local state
    }
    setUser(null);
    router.push("/login");
  }, [router]);

  const refreshUser = useCallback(async () => {
    try {
      const currentUser = await getCurrentUser();
      setUser(currentUser);
    } catch {
      setUser(null);
    }
  }, []);

  const value: AuthContextType = {
    user,
    isLoading,
    isAuthenticated: !!user,
    isAdmin: user?.role === "admin",
    login,
    logout,
    refreshUser,
  };

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthContextType {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error("useAuth must be used within an AuthProvider");
  }
  return context;
}

// Hook to require authentication
export function useRequireAuth(requireAdmin = false): AuthContextType {
  const auth = useAuth();
  const router = useRouter();

  useEffect(() => {
    if (!auth.isLoading) {
      if (!auth.isAuthenticated) {
        router.push("/login");
      } else if (requireAdmin && !auth.isAdmin) {
        router.push("/");
      }
    }
  }, [auth.isLoading, auth.isAuthenticated, auth.isAdmin, requireAdmin, router]);

  return auth;
}
