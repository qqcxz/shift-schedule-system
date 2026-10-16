import type { RouteLocationNormalized, Router } from 'vue-router';

type AuthState = {
  isLoggedIn: boolean;
  mustChangePassword: boolean;
  isManager: boolean;
};

export function getAuthRedirect(to: RouteLocationNormalized, auth: AuthState) {
  if (!to.meta.public && !auth.isLoggedIn) {
    return { path: '/login', query: { redirect: to.fullPath } };
  }
  if (to.name === 'login' && auth.isLoggedIn) {
    return auth.mustChangePassword ? '/change-password' : '/schedule';
  }
  if (auth.mustChangePassword && to.name !== 'change-password') {
    return '/change-password';
  }
  if (to.name === 'change-password' && !auth.mustChangePassword) {
    return '/schedule';
  }
  if (to.meta.managerOnly && !auth.isManager) {
    return '/schedule';
  }
}

export async function initializeAuth(router: Router, auth: AuthState & { restore: () => Promise<void> }) {
  // Resolve the initial URL first so a failed restore can retain the exact
  // return path without racing the initial navigation.
  await router.isReady();
  await auth.restore();
  // The server may have changed role/password requirements since the cached
  // user was saved. Recheck before mounting any protected page.
  const redirect = getAuthRedirect(router.currentRoute.value, auth);
  if (redirect) await router.replace(redirect);
}
