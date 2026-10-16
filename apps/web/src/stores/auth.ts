import { defineStore } from 'pinia';
import { computed, ref } from 'vue';
import type { User } from '../api/index.ts';
import { loginApi, meApi } from '../api/index.ts';
import { connectSocket, disconnectSocket, getSocket } from '../realtime.ts';

const TOKEN_KEY = 'shift_token';
const USER_KEY = 'shift_user';

export const useAuthStore = defineStore('auth', () => {
  const token = ref<string>(localStorage.getItem(TOKEN_KEY) || '');
  const user = ref<User | null>(
    localStorage.getItem(USER_KEY) ? JSON.parse(localStorage.getItem(USER_KEY) as string) : null,
  );

  const isManager = computed(() => user.value?.role === 'manager');
  const isLoggedIn = computed(() => Boolean(token.value && user.value));
  const mustChangePassword = computed(() => Boolean(user.value?.mustChangePassword));

  async function login(username: string, password: string) {
    const { data } = await loginApi(username, password);
    token.value = data.accessToken;
    user.value = data.user;
    localStorage.setItem(TOKEN_KEY, data.accessToken);
    localStorage.setItem(USER_KEY, JSON.stringify(data.user));
    if (data.user.mustChangePassword) {
      disconnectSocket();
    } else {
      connectSocket(data.accessToken);
    }
  }

  async function restore() {
    if (!token.value) return;
    const restoringToken = token.value;
    try {
      const { data } = await meApi();
      if (token.value !== restoringToken) return;
      user.value = data;
      localStorage.setItem(USER_KEY, JSON.stringify(data));
      if (data.mustChangePassword) {
        disconnectSocket();
      } else {
        connectSocket(token.value);
      }
    } catch {
      // HTTP 401 is handled centrally. A temporary network/server failure must
      // not erase a valid cached session or sign out a newer login.
      // Initialize before pages subscribe; Socket.IO will retry when the network
      // recovers. Leave any existing socket (and its listeners) intact.
      if (token.value === restoringToken && isLoggedIn.value && !mustChangePassword.value && !getSocket()) {
        connectSocket(restoringToken);
      }
    }
  }

  function syncUser(nextUser: User) {
    if (user.value?.id !== nextUser.id) return;
    user.value = nextUser;
    localStorage.setItem(USER_KEY, JSON.stringify(nextUser));
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
    mustChangePassword,
    isLoggedIn,
    login,
    restore,
    syncUser,
    logout,
  };
});
