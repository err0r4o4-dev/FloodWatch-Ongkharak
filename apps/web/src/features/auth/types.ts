export type AccountRole = "USER" | "ADMIN";

export type Account = {
  id: string;
  email: string;
  username?: string | null;
  role: AccountRole;
  createdAt: string;
};

export type Credentials = {
  email: string;
  password: string;
};

export type RegisterInput = {
  email: string;
  username: string;
  password: string;
};
