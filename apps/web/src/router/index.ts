import { createRouter, createWebHistory } from 'vue-router';
import { useAuthStore } from '../stores/auth';
import LoginView from '../views/LoginView.vue';
import LayoutView from '../views/LayoutView.vue';
import ScheduleView from '../views/ScheduleView.vue';
import RequestsView from '../views/RequestsView.vue';
import NotificationsView from '../views/NotificationsView.vue';

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', name: 'login', component: LoginView, meta: { public: true } },
    {
      path: '/',
      component: LayoutView,
      children: [
        { path: '', redirect: '/schedule' },
        { path: 'schedule', name: 'schedule', component: ScheduleView },
        { path: 'requests', name: 'requests', component: RequestsView },
        { path: 'notifications', name: 'notifications', component: NotificationsView },
      ],
    },
  ],
});

router.beforeEach(async (to) => {
  const auth = useAuthStore();
  if (!auth.user && auth.token) {
    await auth.restore();
  }

  if (!to.meta.public && !auth.isLoggedIn) {
    return '/login';
  }
  if (to.path === '/login' && auth.isLoggedIn) {
    return '/schedule';
  }
  return true;
});

export default router;
