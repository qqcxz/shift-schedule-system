import { defineStore } from 'pinia';
import { computed, ref } from 'vue';
import type { User } from '../api';
import { loginApi, meApi } from '../api';
import { connectSocket, disconnectSocket } from '../realtime';

const TOKEN_KEY = 'shift_token';
const USER_KEY = 'shift_user';

export const useAuthStore = defineStore('auth', () => {
  const token = ref<string>(localStorage.getItem(TOKEN_KEY) || '');
  const user = ref<User | null>(
    localStorage.getItem(USER_KEY) ? JSON.parse(localStorage.getItem(USER_KEY) as string) : null,
  );

  const isManager = computed(() => user.value?.role === 'manager');
  const isLoggedIn = computed(() => Boolean(token.value && user.value));

  async function login(username: string, password: string) {
    const { data } = await loginApi(username, password);
    token.value = data.accessToken;
    user.value = data.user;
    localStorage.setItem(TOKEN_KEY, data.accessToken);
    localStorage.setItem(USER_KEY, JSON.stringify(data.user));
    connectSocket(data.accessToken);
  }

  async function restore() {
    if (!token.value) return;
    try {
      const { data } = await meApi();
      user.value = data;
      localStorage.setItem(USER_KEY, JSON.stringify(data));
      connectSocket(token.value);
    } catch {
      logout();
    }
  }

  function logout() {
    token.value = '';
    user.value = null;
    localStorage.removeItem(TOKEN_KEY);
    localStorage.removeItem(USER_KEY);
    disconnectSocket();
  }

  return {
    token,
    user,
    isManager,
    isLoggedIn,
    login,
    restore,
    logout,
  };
});
