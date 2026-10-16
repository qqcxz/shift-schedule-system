import { createApp } from 'vue';
import { createPinia } from 'pinia';
import ElementPlus, { ElMessage } from 'element-plus';
import 'element-plus/dist/index.css';
import zhCn from 'element-plus/es/locale/lang/zh-cn';
import App from './App.vue';
import router from './router';
import { useAuthStore } from './stores/auth';
import { setSessionExpiredHandler } from './api/http';
import { createSessionExpiredHandler } from './router/session-expired';
import { initializeAuth } from './router/auth-guard';
import './styles/main.css';

const app = createApp(App);
const pinia = createPinia();
app.use(pinia);
const auth = useAuthStore(pinia);
setSessionExpiredHandler(createSessionExpiredHandler(
  router,
  (message) => ElMessage.warning(message),
  (url) => window.location.replace(url),
  () => auth.isLoggedIn,
));
app.use(router);
app.use(ElementPlus, { locale: zhCn });

initializeAuth(router, auth).finally(() => {
  app.mount('#app');
});
