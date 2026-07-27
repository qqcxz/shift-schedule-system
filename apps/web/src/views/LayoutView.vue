<template>
  <div class="layout" :class="{ 'nav-open': mobileNavOpen }">
    <div v-if="mobileNavOpen" class="nav-mask" @click="mobileNavOpen = false" />

    <aside class="sider">
      <div class="brand">
        <div class="brand-mark">班</div>
        <div>
          <div class="brand-title">门店排班</div>
          <div class="brand-sub">Shift Board</div>
        </div>
      </div>
      <el-menu :default-active="route.path" router class="menu" @select="mobileNavOpen = false">
        <el-menu-item index="/schedule">排班表</el-menu-item>
        <el-menu-item index="/requests">申请审批</el-menu-item>
        <el-menu-item index="/notifications">消息中心</el-menu-item>
        <el-menu-item v-if="auth.isManager" index="/shifts">班次管理</el-menu-item>
        <el-menu-item v-if="auth.isManager" index="/staff">员工管理</el-menu-item>
      </el-menu>
    </aside>

    <main class="main">
      <header class="header">
        <div class="header-left">
          <el-button class="menu-btn" @click="mobileNavOpen = !mobileNavOpen">菜单</el-button>
          <div>
            <div class="title">{{ pageTitle }}</div>
            <div class="muted store-line">{{ storeName || '当前门店' }} · 数据实时互通</div>
          </div>
        </div>
        <div class="header-right">
          <el-tag :type="auth.isManager ? 'warning' : 'success'">
            {{ auth.isManager ? '店长' : '店员' }}
          </el-tag>
          <span class="user-name">{{ auth.user?.displayName }}</span>
          <el-button @click="passwordVisible = true">修改密码</el-button>
          <el-button @click="onLogout">退出</el-button>
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
  min-height: 100vh;
  display: grid;
  grid-template-columns: 232px 1fr;
  background: #f5f7fb;
}

.nav-mask {
  display: none;
}

.sider {
  background: linear-gradient(180deg, #f8fbff 0%, #eef4ff 100%);
  border-right: 1px solid #dbe4f3;
  color: #1f2937;
  padding: 20px 14px;
}

.brand {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 8px 10px 22px;
}

.brand-mark {
  width: 40px;
  height: 40px;
  border-radius: 12px;
  display: grid;
  place-items: center;
  background: linear-gradient(135deg, #3b82f6, #2563eb);
  color: #fff;
  font-weight: 700;
  box-shadow: 0 8px 18px rgba(37, 99, 235, 0.28);
}

.brand-title {
  font-size: 18px;
  font-weight: 700;
  color: #111827;
  line-height: 1.2;
}

.brand-sub {
  margin-top: 2px;
  font-size: 12px;
  color: #6b7280;
}

.menu {
  border-right: none;
  background: transparent;
}

.menu :deep(.el-menu-item) {
  height: 44px;
  margin-bottom: 6px;
  border-radius: 12px;
  color: #334155 !important;
  font-weight: 600;
}

.menu :deep(.el-menu-item:hover) {
  background: rgba(37, 99, 235, 0.08) !important;
  color: #1d4ed8 !important;
}

.menu :deep(.el-menu-item.is-active) {
  background: #2563eb !important;
  color: #fff !important;
  box-shadow: 0 10px 20px rgba(37, 99, 235, 0.22);
}

.main {
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.header {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  align-items: center;
  padding: 18px 24px;
  background: rgba(255, 255, 255, 0.92);
  border-bottom: 1px solid #e5e7eb;
  position: sticky;
  top: 0;
  z-index: 10;
  backdrop-filter: blur(8px);
}

.header-left {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
}

.menu-btn {
  display: none;
}

.title {
  font-size: 20px;
  font-weight: 700;
}

.header-right {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.content {
  padding: 20px 24px 28px;
}

@media (max-width: 900px) {
  .layout {
    grid-template-columns: 1fr;
  }

  .nav-mask {
    display: block;
    position: fixed;
    inset: 0;
    background: rgba(15, 23, 42, 0.35);
    z-index: 40;
  }

  .sider {
    position: fixed;
    top: 0;
    left: 0;
    bottom: 0;
    width: min(280px, 86vw);
    z-index: 50;
    transform: translateX(-105%);
    transition: transform 0.2s ease;
    box-shadow: 12px 0 30px rgba(15, 23, 42, 0.12);
    overflow-y: auto;
  }

  .layout.nav-open .sider {
    transform: translateX(0);
  }

  .menu-btn {
    display: inline-flex;
  }

  .header {
    padding: 12px 14px;
    align-items: flex-start;
    flex-direction: column;
  }

  .header-left,
  .header-right {
    width: 100%;
  }

  .title {
    font-size: 18px;
  }

  .store-line {
    font-size: 12px;
  }

  .content {
    padding: 14px 12px 24px;
  }
}

@media (max-width: 640px) {
  .user-name {
    max-width: 100%;
  }

  .header-right {
    gap: 8px;
  }

  .header-right :deep(.el-button) {
    flex: 1 1 calc(50% - 8px);
  }
}
</style>
