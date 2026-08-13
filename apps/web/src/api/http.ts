import axios from 'axios';
import { useAuthStore } from '../stores/auth';

const http = axios.create({
  baseURL: '/api',
  timeout: 15000,
});

http.interceptors.request.use((config) => {
  const auth = useAuthStore();
  if (auth.token) {
    config.headers.Authorization = `Bearer ${auth.token}`;
  }
  return config;
});

http.interceptors.response.use(
  (response) => response,
  async (error) => {
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
    return Promise.reject(new Error(Array.isArray(message) ? message.join(', ') : message));
  },
);

export default http;
