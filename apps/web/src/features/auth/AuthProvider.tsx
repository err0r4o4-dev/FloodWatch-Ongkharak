import { createContext, useCallback, useContext, useEffect, useMemo, useState } from "react";
import { authApi } from "./api";
import type { AuthApi } from "./api";
import type { Account } from "./types";

type AuthStatus = "loading" | "anonymous" | "authenticated";

type AuthContextValue = {
  account: Account | null;
  logout(): Promise<void>;
  status: AuthStatus;
};

type AuthProviderProps = {
  api?: AuthApi;
  children: React.ReactNode;
};

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ api = authApi, children }: AuthProviderProps) {
  const [account, setAccount] = useState<Account | null>(null);
  const [status, setStatus] = useState<AuthStatus>("loading");

  useEffect(() => {
    let active = true;

    const bootstrap = async () => {
      try {
        await api.refresh();
        const currentAccount = await api.me();
        if (active) {
          setAccount(currentAccount);
          setStatus("authenticated");
        }
      } catch {
        if (active) {
          setAccount(null);
          setStatus("anonymous");
        }
      }
    };

    void bootstrap();
    return () => {
      active = false;
    };
  }, [api]);

  const logout = useCallback(async () => {
    try {
      await api.logout();
    } finally {
      setAccount(null);
      setStatus("anonymous");
    }
  }, [api]);

  const value = useMemo(() => ({ account, logout, status }), [account, logout, status]);
  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error("useAuth must be used within AuthProvider.");
  }
  return context;
}
