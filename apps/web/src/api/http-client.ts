import axios from 'axios';
import { ApiError } from './errors.ts';

declare module 'axios' {
  interface AxiosRequestConfig {
    skipAuth?: boolean;
  }
}

type AuthSession = {
  token: string;
  logout: () => void;
};

type HttpClientOptions = {
  getAuth: () => AuthSession;
  onSessionExpired: () => void | Promise<void>;
};

export function createHttpClient({ getAuth, onSessionExpired }: HttpClientOptions) {
  const http = axios.create({ baseURL: '/api', timeout: 15000 });
  let sessionExpired = false;

  http.interceptors.request.use((config) => {
    if (config.skipAuth) {
      config.headers.delete('Authorization');
      return config;
    }
    const auth = getAuth();
    if (auth.token) {
      config.headers.set('Authorization', `Bearer ${auth.token}`);
      sessionExpired = false;
    }
    return config;
  });

  http.interceptors.response.use(
    (response) => response,
    async (error) => {
      const status = error?.response?.status as number | undefined;
      const config = error?.config;
      if (status === 401 && !config?.skipAuth) {
        const auth = getAuth();
        const authorization = config?.headers?.get('Authorization');
        const requestToken = typeof authorization === 'string'
          ? authorization.replace(/^Bearer\s+/i, '')
          : '';
        // Only the session that sent this request can be invalidated. An old
        // request must not sign out a user who has already logged in again.
        if (auth.token === requestToken && !sessionExpired) {
          sessionExpired = true;
          auth.logout();
          await onSessionExpired();
        }
        throw new ApiError('登录已失效，请重新登录', status, true);
      }

      const responseData = error?.response?.data;
      let message = responseData?.message;
      if (!message && typeof Blob !== 'undefined' && responseData instanceof Blob) {
        try {
          const text = await responseData.text();
          const parsed = JSON.parse(text) as { message?: string | string[] };
          message = parsed.message || text;
        } catch {
          message = undefined;
        }
      }
      message = message || error.message || '请求失败';
      throw new ApiError(Array.isArray(message) ? message.join(', ') : message, status);
    },
  );

  return http;
}
