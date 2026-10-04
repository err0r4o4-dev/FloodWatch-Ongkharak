import { createContext, useCallback, useContext, useEffect, useMemo, useState } from "react";
import { accountApi as defaultAccountApi, authApi } from "./api";
import type { AccountApi, AuthApi } from "./api";
import type { Account, Credentials, RegisterInput } from "./types";

type AuthStatus = "loading" | "anonymous" | "authenticated";

type AuthContextValue = {
  account: Account | null;
  login(input: Credentials): Promise<Account>;
  logout(): Promise<void>;
  register(input: RegisterInput): Promise<Account>;
  status: AuthStatus;
  updateUsername(username: string): Promise<void>;
};

type AuthProviderProps = {
  api?: AuthApi;
  accountApi?: AccountApi;
  children: React.ReactNode;
};

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ api = authApi, accountApi = defaultAccountApi, children }: AuthProviderProps) {
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
    const loggedInAccount = currentAccount ?? session.user;
    setAccount(loggedInAccount);
    setStatus("authenticated");
    return loggedInAccount;
  }, [api]);

  const register = useCallback((input: RegisterInput) => api.register(input), [api]);

  const updateUsername = useCallback(async (username: string) => {
    if (!account) throw new Error("Authentication is unavailable.");
    const updatedAccount = await accountApi.updateUsername(username);
    if (updatedAccount.id !== account.id) throw new Error("Account changed while updating username.");
    setAccount((currentAccount) => currentAccount === account ? updatedAccount : currentAccount);
  }, [account, accountApi]);

  const value = useMemo(() => ({ account, login, logout, register, status, updateUsername }), [account, login, logout, register, status, updateUsername]);
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
    login: async () => { throw new Error("Authentication is unavailable."); },
    logout: async () => undefined,
    register: async () => { throw new Error("Authentication is unavailable."); },
    updateUsername: async () => { throw new Error("Authentication is unavailable."); },
    status: "anonymous"
  };
}
