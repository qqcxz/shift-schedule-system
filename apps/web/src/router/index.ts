import { createRouter, createWebHistory } from 'vue-router';
import { useAuthStore } from '../stores/auth';
import { getAuthRedirect } from './auth-guard';
import LoginView from '../views/LoginView.vue';
import LayoutView from '../views/LayoutView.vue';
import ScheduleView from '../views/ScheduleView.vue';
import RequestsView from '../views/RequestsView.vue';
import NotificationsView from '../views/NotificationsView.vue';
import ShiftsView from '../views/ShiftsView.vue';
import StaffView from '../views/StaffView.vue';
import ChangePasswordView from '../views/ChangePasswordView.vue';
import PushSettingsView from '../views/PushSettingsView.vue';

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/login',
      name: 'login',
      component: LoginView,
      meta: { public: true },
    },
    {
      path: '/change-password',
      name: 'change-password',
      component: ChangePasswordView,
    },
    {
      path: '/',
      component: LayoutView,
      children: [
        { path: '', redirect: '/schedule' },
        { path: 'schedule', name: 'schedule', component: ScheduleView },
        { path: 'requests', name: 'requests', component: RequestsView },
        { path: 'notifications', name: 'notifications', component: NotificationsView },
        { path: 'settings', name: 'settings', component: PushSettingsView },
        {
          path: 'shifts',
          name: 'shifts',
          component: ShiftsView,
          meta: { managerOnly: true },
        },
        {
          path: 'staff',
          name: 'staff',
          component: StaffView,
          meta: { managerOnly: true },
        },
      ],
    },
  ],
});

router.beforeEach((to) => getAuthRedirect(to, useAuthStore()) ?? true);

export default router;
