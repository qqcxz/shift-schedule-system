import { useAuthStore } from '../stores/auth.ts';
import { createHttpClient } from './http-client.ts';

let sessionExpiredHandler: () => void | Promise<void> = () => {};

export function setSessionExpiredHandler(handler: () => void | Promise<void>) {
  sessionExpiredHandler = handler;
}

const http = createHttpClient({
  getAuth: () => useAuthStore(),
  onSessionExpired: () => sessionExpiredHandler(),
});

export default http;
