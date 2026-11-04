import { getCsrfToken, setCsrfToken } from '../../lib/auth';

export type LoginUser = {
  id: number;
  username: string;
  role: string;
  status: string;
  must_change_password: boolean;
};

export type LoginResponse = {
  user: LoginUser;
  csrf_token: string;
};

export type LoginErrorCode =
  | 'INVALID_CREDENTIALS'
  | 'ACCOUNT_PENDING'
  | 'ACCOUNT_DISABLED'
  | 'ACCOUNT_LOCKED'
  | 'RATE_LIMITED'
  | 'NETWORK_ERROR';

const LOGIN_ERROR_CODES: readonly LoginErrorCode[] = [
  'INVALID_CREDENTIALS',
  'ACCOUNT_PENDING',
  'ACCOUNT_DISABLED',
  'ACCOUNT_LOCKED',
  'RATE_LIMITED',
  'NETWORK_ERROR',
];

function isLoginErrorCode(value: string): value is LoginErrorCode {
  return (LOGIN_ERROR_CODES as readonly string[]).includes(value);
}

export class LoginError extends Error {
  readonly code: LoginErrorCode;

  constructor(code: LoginErrorCode, message: string) {
    super(message);
    this.name = 'LoginError';
    this.code = code;
  }
}

async function readLoginError(response: Response): Promise<LoginError> {
  const raw = await response.text();
  let code: LoginErrorCode = 'NETWORK_ERROR';
  let message = `Request failed: ${response.status}`;
  if (raw) {
    try {
      const parsed: unknown = JSON.parse(raw);
      if (typeof parsed === 'object' && parsed !== null) {
        const { code: bodyCode, message: bodyMessage } = parsed as {
          code?: unknown;
          message?: unknown;
        };
        if (typeof bodyCode === 'string' && isLoginErrorCode(bodyCode)) {
          code = bodyCode;
        }
        if (typeof bodyMessage === 'string') {
          message = bodyMessage;
        }
      }
    } catch {
      // non-JSON error body; keep the NETWORK_ERROR fallback
    }
  }
  return new LoginError(code, message);
}

/**
 * POST /api/v1/auth/login (spec §4.1), storing the returned CSRF token in
 * memory. Throws LoginError carrying the §4.5 code for code-keyed i18n.
 *
 * Uses its own fetch rather than apiFetch: the wrapper's ApiError drops the
 * response `code`, and login is the only endpoint whose codes are
 * status-ambiguous (403 = pending/disabled/locked). The envelope mirrors
 * apiFetch, so both merge once ApiError carries the code.
 */
export async function login(username: string, password: string): Promise<LoginResponse> {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' };
  const csrfToken = getCsrfToken();
  if (csrfToken !== null) {
    headers['X-CSRF-Token'] = csrfToken;
  }

  let response: Response;
  try {
    response = await fetch('/api/v1/auth/login', {
      method: 'POST',
      headers,
      body: JSON.stringify({ username, password }),
    });
  } catch {
    throw new LoginError('NETWORK_ERROR', 'Network request failed');
  }

  if (!response.ok) {
    throw await readLoginError(response);
  }

  const data = (await response.json()) as LoginResponse;
  setCsrfToken(data.csrf_token);

  return data;
}
