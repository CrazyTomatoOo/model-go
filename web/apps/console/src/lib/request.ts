import { getCsrfToken } from './auth';

export class ApiError extends Error {
  readonly status: number;

  constructor(status: number, message: string) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
  }
}

export type ApiRequestOptions = {
  method?: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE';
  body?: Record<string, unknown>;
  csrfToken?: string | null;
};

type JsonObject = Record<string, unknown>;

async function readError(response: Response): Promise<ApiError> {
  const raw = await response.text();
  let message: string | null = null;
  if (raw) {
    try {
      const parsed: unknown = JSON.parse(raw);
      if (
        typeof parsed === 'object' &&
        parsed !== null &&
        typeof (parsed as JsonObject).message === 'string'
      ) {
        message = (parsed as JsonObject).message as string;
      }
    } catch {
      // non-JSON error body; fall back to the generic message below
    }
  }
  return new ApiError(response.status, message ?? `Request failed: ${response.status}`);
}

export async function apiFetch<T>(path: string, options: ApiRequestOptions = {}): Promise<T> {
  const { method = 'GET', body } = options;
  // Spec §4.1: X-CSRF-Token must match the session's token on state-changing requests.
  const csrfToken = options.csrfToken !== undefined ? options.csrfToken : getCsrfToken();

  const headers: Record<string, string> = {};
  if (body !== undefined) {
    headers['Content-Type'] = 'application/json';
  }
  if (method !== 'GET' && csrfToken !== null) {
    headers['X-CSRF-Token'] = csrfToken;
  }

  const response = await fetch(path, {
    method,
    headers,
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });

  if (!response.ok) {
    throw await readError(response);
  }

  return (await response.json()) as T;
}
