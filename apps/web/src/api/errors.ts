export class ApiError extends Error {
  readonly status?: number;
  readonly handled: boolean;

  constructor(message: string, status?: number, handled = false) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.handled = handled;
  }
}

export function isHandledApiError(error: unknown): boolean {
  return error instanceof ApiError && error.handled;
}
