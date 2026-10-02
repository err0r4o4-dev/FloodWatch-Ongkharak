import { createContext, useCallback, useContext, useEffect, useMemo, useState } from "react";
import { authApi } from "./api";
import type { AuthApi } from "./api";
import type { Account, Credentials, RegisterInput } from "./types";

type AuthStatus = "loading" | "anonymous" | "authenticated";

type AuthContextValue = {
  account: Account | null;
  login(input: Credentials): Promise<void>;
  logout(): Promise<void>;
  register(input: RegisterInput): Promise<Account>;
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

  const login = useCallback(async (input: Credentials) => {
    const session = await api.login(input);
    const currentAccount = await api.me();
    setAccount(currentAccount ?? session.user);
    setStatus("authenticated");
  }, [api]);

  const register = useCallback((input: RegisterInput) => api.register(input), [api]);

  const value = useMemo(() => ({ account, login, logout, register, status }), [account, login, logout, register, status]);
  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error("useAuth must be used within AuthProvider.");
  }
  return context;
}

export function useOptionalAuth(): AuthContextValue {
  const context = useContext(AuthContext);
  return context ?? {
    account: null,
    login: async () => undefined,
    logout: async () => undefined,
    register: async () => { throw new Error("Authentication is unavailable."); },
    status: "anonymous"
  };
}
