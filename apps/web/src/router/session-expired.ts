import type { Router } from 'vue-router';

export function createSessionExpiredHandler(
  router: Pick<Router, 'currentRoute' | 'replace' | 'resolve'>,
  notify: (message: string) => void,
  fallback: (url: string) => void,
  hasSession: () => boolean = () => false,
) {
  return async () => {
    const current = router.currentRoute.value;
    if (current.name === 'login') return;
    const target = { path: '/login', query: { redirect: current.fullPath } };
    notify('登录已失效，请重新登录');
    try {
      const failure = await router.replace(target);
      if (!hasSession() && (failure || router.currentRoute.value.name !== 'login')) {
        fallback(router.resolve(target).href);
      }
    } catch {
      if (!hasSession()) fallback(router.resolve(target).href);
    }
  };
}
