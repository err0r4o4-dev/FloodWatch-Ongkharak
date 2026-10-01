export type AccountRole = "USER" | "ADMIN";

export type Account = {
  id: string;
  email: string;
  role: AccountRole;
  createdAt: string;
};

export type Credentials = {
  email: string;
  password: string;
};

export type RegisterInput = Credentials;
