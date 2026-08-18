<template>
  <div class="layout" :class="{ 'nav-open': mobileNavOpen }">
    <div v-if="mobileNavOpen" class="nav-mask" @click="mobileNavOpen = false" />

    <aside class="sider">
      <div class="brand">
        <div class="brand-mark" aria-hidden="true"><Command :size="20" :stroke-width="1.8" /></div>
        <div class="brand-copy">
          <div class="brand-kicker">OPERATIONS / 01</div>
          <div class="brand-title">SHIFTBOARD</div>
          <div class="brand-sub">门店协作中枢</div>
        </div>
      </div>

      <div class="nav-label">工作台</div>
      <el-menu :default-active="route.path" router class="menu" @select="mobileNavOpen = false">
        <el-menu-item index="/schedule">
          <CalendarDays :size="17" :stroke-width="1.8" /><span>排班表</span><ChevronRight class="nav-arrow" :size="14" />
        </el-menu-item>
        <el-menu-item index="/requests">
          <ClipboardCheck :size="17" :stroke-width="1.8" /><span>申请审批</span><ChevronRight class="nav-arrow" :size="14" />
        </el-menu-item>
        <el-menu-item index="/notifications">
          <Bell :size="17" :stroke-width="1.8" /><span>消息中心</span><ChevronRight class="nav-arrow" :size="14" />
        </el-menu-item>
        <el-menu-item index="/settings">
          <Send :size="17" :stroke-width="1.8" /><span>微信推送</span><ChevronRight class="nav-arrow" :size="14" />
        </el-menu-item>
        <div class="nav-label nav-label-manager">管理</div>
        <el-menu-item v-if="auth.isManager" index="/shifts">
          <Clock3 :size="17" :stroke-width="1.8" /><span>班次管理</span><ChevronRight class="nav-arrow" :size="14" />
        </el-menu-item>
        <el-menu-item v-if="auth.isManager" index="/staff">
          <Users :size="17" :stroke-width="1.8" /><span>员工管理</span><ChevronRight class="nav-arrow" :size="14" />
        </el-menu-item>
      </el-menu>

      <div class="sider-footer">
        <div class="live-mark"><span class="live-dot" /> LIVE SYNC</div>
        <div class="sider-footnote">所有变更实时同步至团队</div>
      </div>
    </aside>

    <main class="main">
      <header class="header">
        <div class="header-left">
          <el-button class="menu-btn" aria-label="打开导航" @click="mobileNavOpen = !mobileNavOpen">
            <PanelLeft :size="18" />
          </el-button>
          <div class="heading-stack">
            <div class="crumb">WORKSPACE <span>/</span> {{ pageTitle }}</div>
            <div class="title">{{ pageTitle }}</div>
            <div class="store-line">{{ storeName || '当前门店' }} <span>·</span> 数据实时互通</div>
          </div>
        </div>
        <div class="header-right">
          <div class="sync-state"><span class="live-dot" /> 实时连接</div>
          <el-tag class="role-tag" :type="auth.isManager ? 'warning' : 'success'">
            <ShieldCheck :size="13" />{{ auth.isManager ? '店长' : '店员' }}
          </el-tag>
          <span class="user-name">{{ auth.user?.displayName }}</span>
          <el-button class="header-action" aria-label="修改密码" @click="passwordVisible = true"><KeyRound :size="16" /><span>修改密码</span></el-button>
          <el-button class="header-action logout-action" aria-label="退出登录" @click="onLogout"><LogOut :size="16" /><span>退出</span></el-button>
        </div>
      </header>
      <section class="content">
        <router-view />
      </section>
    </main>

    <el-dialog
      v-model="passwordVisible"
      title="修改密码"
      class="responsive-dialog"
      width="420px"
      @closed="resetPasswordForm"
    >
      <el-form label-width="96px" class="mobile-form">
        <el-form-item label="当前密码">
          <el-input v-model="passwordForm.currentPassword" type="password" show-password />
        </el-form-item>
        <el-form-item label="新密码">
          <el-input v-model="passwordForm.newPassword" type="password" show-password />
        </el-form-item>
        <el-form-item label="确认密码">
          <el-input v-model="passwordForm.confirmPassword" type="password" show-password />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="passwordVisible = false">取消</el-button>
        <el-button type="primary" :loading="passwordSaving" @click="onChangePassword">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { ElMessage } from 'element-plus';
import {
  Bell,
  CalendarDays,
  ChevronRight,
  ClipboardCheck,
  Clock3,
  Command,
  KeyRound,
  LogOut,
  PanelLeft,
  Send,
  ShieldCheck,
  Users,
} from 'lucide-vue-next';
import { changePasswordApi, storeApi } from '../api';
import { useAuthStore } from '../stores/auth';

const auth = useAuthStore();
const route = useRoute();
const router = useRouter();
const storeName = ref('');
const passwordVisible = ref(false);
const passwordSaving = ref(false);
const mobileNavOpen = ref(false);
const passwordForm = reactive({
  currentPassword: '',
  newPassword: '',
  confirmPassword: '',
});

const pageTitle = computed(() => {
  if (route.path.startsWith('/requests')) return '申请与审批';
  if (route.path.startsWith('/notifications')) return '消息中心';
  if (route.path.startsWith('/settings')) return '微信推送设置';
  if (route.path.startsWith('/shifts')) return '班次管理';
  if (route.path.startsWith('/staff')) return '员工管理';
  return '月度排班表';
});

watch(
  () => route.path,
  () => {
    mobileNavOpen.value = false;
  },
);

function resetPasswordForm() {
  passwordForm.currentPassword = '';
  passwordForm.newPassword = '';
  passwordForm.confirmPassword = '';
}

async function onChangePassword() {
  if (!passwordForm.currentPassword || !passwordForm.newPassword) {
    ElMessage.warning('请填写当前密码和新密码');
    return;
  }
  if (passwordForm.newPassword.length < 6) {
    ElMessage.warning('新密码至少 6 位');
    return;
  }
  if (passwordForm.newPassword !== passwordForm.confirmPassword) {
    ElMessage.warning('两次输入的新密码不一致');
    return;
  }

  passwordSaving.value = true;
  try {
    await changePasswordApi({
      currentPassword: passwordForm.currentPassword,
      newPassword: passwordForm.newPassword,
    });
    ElMessage.success('密码已修改，请重新登录');
    passwordVisible.value = false;
    auth.logout();
    router.push('/login');
  } catch (error) {
    ElMessage.error((error as Error).message);
  } finally {
    passwordSaving.value = false;
  }
}

onMounted(async () => {
  try {
    const { data } = await storeApi();
    storeName.value = data?.name || '';
  } catch {
    storeName.value = '';
  }
});

function onLogout() {
  auth.logout();
  router.push('/login');
}
</script>

<style scoped>
.layout {
  --ink: #f4f5ed;
  min-height: 100vh;
  display: grid;
  grid-template-columns: 248px minmax(0, 1fr);
  background: #f1f1eb;
}
.nav-mask { display: none; }
.sider { position: relative; display: flex; flex-direction: column; min-height: 100vh; padding: 28px 18px 22px; overflow: hidden; color: var(--ink); background: #121412; border-right: 1px solid #292c28; }
.sider::before { position: absolute; inset: 0; background-image: linear-gradient(rgba(255,255,255,.035) 1px, transparent 1px), linear-gradient(90deg, rgba(255,255,255,.035) 1px, transparent 1px); background-size: 32px 32px; mask-image: linear-gradient(to bottom, black, transparent 70%); pointer-events: none; content: ''; }
.brand, .nav-label, .menu, .sider-footer { position: relative; z-index: 1; }
.brand { display: flex; align-items: flex-start; gap: 12px; padding: 2px 8px 38px; }
.brand-mark { width: 40px; height: 40px; flex: 0 0 auto; display: grid; place-items: center; color: #121412; background: #d8ff55; border-radius: 50%; box-shadow: 0 0 0 7px rgba(216,255,85,.08); }
.brand-copy { min-width: 0; }
.brand-kicker, .nav-label, .crumb, .live-mark { font-family: "SFMono-Regular", Consolas, monospace; letter-spacing: .13em; }
.brand-kicker { color: #8b9185; font-size: 9px; line-height: 1; }
.brand-title { margin-top: 8px; color: #f4f5ed; font-size: 17px; font-weight: 760; letter-spacing: .08em; line-height: 1; }
.brand-sub { margin-top: 7px; color: #838980; font-size: 12px; }
.nav-label { padding: 0 10px 10px; color: #70766d; font-size: 9px; }
.nav-label-manager { margin-top: 26px; }
.menu { border-right: 0; background: transparent; }
.menu :deep(.el-menu-item) { display: flex; align-items: center; gap: 12px; height: 48px; margin: 3px 0; padding: 0 11px !important; color: #a9aea3 !important; border: 1px solid transparent; border-radius: 8px; font-size: 13px; font-weight: 560; transition: .25s ease; }
.menu :deep(.el-menu-item svg) { flex: 0 0 auto; }
.menu :deep(.el-menu-item .nav-arrow) { margin-left: auto; opacity: 0; transform: translateX(-5px); transition: .25s ease; }
.menu :deep(.el-menu-item:hover) { color: #f4f5ed !important; background: rgba(255,255,255,.055) !important; }
.menu :deep(.el-menu-item:hover .nav-arrow) { opacity: .6; transform: translateX(0); }
.store-line { margin-top: 6px; color: #7b8378; font-size: 11px; letter-spacing: .02em; }
.store-line span { padding: 0 4px; color: #a7ada3; }
.menu :deep(.el-menu-item.is-active) { color: #d8ff55 !important; background: rgba(216,255,85,.1) !important; border-color: rgba(216,255,85,.18); }
.menu :deep(.el-menu-item.is-active)::before { width: 3px; height: 18px; margin-left: -12px; background: #d8ff55; border-radius: 2px; content: ''; }
.menu :deep(.el-menu-item.is-active .nav-arrow) { opacity: 1; transform: translateX(0); }
.sider-footer { margin-top: auto; padding: 16px 10px 0; border-top: 1px solid #2a2d29; }
.live-mark { display: flex; align-items: center; gap: 8px; color: #d8ff55; font-size: 9px; }
.live-dot { width: 6px; height: 6px; display: inline-block; flex: 0 0 auto; border-radius: 50%; background: #d8ff55; box-shadow: 0 0 0 4px rgba(216,255,85,.1); animation: breathe 2.4s ease-in-out infinite; }
.sider-footnote { margin-top: 10px; color: #697068; font-size: 11px; }
.main { display: flex; flex-direction: column; min-width: 0; }
.header { display: flex; align-items: center; justify-content: space-between; gap: 18px; min-height: 90px; padding: 18px 42px; background: rgba(241,241,235,.84); border-bottom: 1px solid #d9dbd3; position: sticky; top: 0; z-index: 10; backdrop-filter: blur(18px); }
.header-left { display: flex; align-items: center; gap: 16px; min-width: 0; }
.heading-stack { min-width: 0; }
.crumb { color: #8c9287; font-size: 9px; white-space: nowrap; }
.crumb span { padding: 0 7px; color: #c1c4bb; }
.title { margin-top: 8px; color: #191c19; font-family: "Arial Narrow", "PingFang SC", sans-serif; font-size: clamp(21px, 2vw, 28px); font-weight: 750; letter-spacing: -.04em; }
.menu-btn { display: none; }
.header-right { display: flex; align-items: center; gap: 12px; flex-wrap: wrap; }
.sync-state { display: inline-flex; align-items: center; gap: 8px; padding-right: 14px; color: #656c62; font-family: "SFMono-Regular", Consolas, monospace; font-size: 10px; letter-spacing: .04em; border-right: 1px solid #d5d7cf; }
.role-tag { display: inline-flex; align-items: center; gap: 5px; border: 0; border-radius: 999px; }
.user-name { color: #30352f; font-size: 13px; font-weight: 650; }
.header-action { min-height: 34px; padding: 0 12px; color: #596058; background: transparent; border-color: #d4d7ce; }
.header-action:hover { color: #191c19; border-color: #9ca398; background: #fff; }
.logout-action { color: #9a5b50; }
.content { position: relative; min-height: calc(100vh - 90px); padding: 34px 42px 48px; overflow: hidden; }
.content::before { position: absolute; width: 480px; height: 480px; top: -260px; right: -180px; border: 1px solid rgba(42,48,40,.08); border-radius: 50%; box-shadow: 0 0 0 40px rgba(42,48,40,.025), 0 0 0 80px rgba(42,48,40,.018); pointer-events: none; content: ''; }
.content > :deep(*) { position: relative; z-index: 1; }
@keyframes breathe { 0%,100% { opacity: .55; transform: scale(.9); } 50% { opacity: 1; transform: scale(1.15); } }
@media (max-width: 1100px) { .header { padding-inline: 26px; } .content { padding-inline: 26px; } .sync-state { display: none; } }
@media (max-width: 900px) {
  .layout { grid-template-columns: 1fr; }
  .nav-mask { display: block; position: fixed; inset: 0; z-index: 40; background: rgba(10,12,10,.55); backdrop-filter: blur(4px); }
  .sider { position: fixed; inset: 0 auto 0 0; z-index: 50; width: min(300px, 88vw); transform: translateX(-105%); transition: transform .35s cubic-bezier(.22,1,.36,1); box-shadow: 20px 0 60px rgba(0,0,0,.24); overflow-y: auto; }
  .layout.nav-open .sider { transform: translateX(0); }
  .menu-btn { display: inline-flex; width: 38px; height: 38px; padding: 0; align-items: center; justify-content: center; border-color: #d4d7ce; color: #30352f; background: transparent; }
  .header { min-height: 76px; padding: 14px 18px; }
  .title { margin-top: 6px; font-size: 22px; }
  .content { min-height: calc(100vh - 76px); padding: 22px 18px 34px; }
}
@media (max-width: 640px) {
  .header { align-items: flex-start; flex-direction: column; gap: 14px; }
  .header-left, .header-right { width: 100%; }
  .header-right { gap: 8px; }
  .header-action { flex: 1 1 auto; justify-content: center; }
  .user-name { max-width: 34%; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .content { padding: 18px 12px 28px; }
}
</style>
