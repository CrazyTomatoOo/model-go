let csrfToken: string | null = null;

export type AuthResponse = {
  user: unknown;
  csrf_token: string;
};

export function getCsrfToken(): string | null {
  return csrfToken;
}

export function setCsrfToken(token: string | null): void {
  csrfToken = token;
}

export function applyAuthToken(response: AuthResponse): void {
  setCsrfToken(response.csrf_token);
}
