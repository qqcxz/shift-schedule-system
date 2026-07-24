<template>
  <div class="layout">
    <aside class="sider">
      <div class="brand">门店排班</div>
      <el-menu :default-active="route.path" router class="menu">
        <el-menu-item index="/schedule">排班表</el-menu-item>
        <el-menu-item index="/requests">申请审批</el-menu-item>
        <el-menu-item index="/notifications">消息中心</el-menu-item>
        <el-menu-item v-if="auth.isManager" index="/shifts">班次管理</el-menu-item>
        <el-menu-item v-if="auth.isManager" index="/staff">员工管理</el-menu-item>
      </el-menu>
    </aside>

    <main class="main">
      <header class="header">
        <div>
          <div class="title">{{ pageTitle }}</div>
          <div class="muted">{{ storeName || '当前门店' }} · 数据实时互通</div>
        </div>
        <div class="header-right">
          <el-tag :type="auth.isManager ? 'warning' : 'success'">
            {{ auth.isManager ? '店长' : '店员' }}
          </el-tag>
          <span>{{ auth.user?.displayName }}</span>
          <el-button @click="passwordVisible = true">修改密码</el-button>
          <el-button @click="onLogout">退出</el-button>
        </div>
      </header>
      <section class="content">
        <router-view />
      </section>
    </main>

    <el-dialog v-model="passwordVisible" title="修改密码" width="420px" @closed="resetPasswordForm">
      <el-form label-width="96px">
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
import { computed, onMounted, reactive, ref } from 'vue';
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
  grid-template-columns: 220px 1fr;
}

.sider {
  background: #111827;
  color: #fff;
  padding: 20px 12px;
}

.brand {
  font-size: 20px;
  font-weight: 700;
  padding: 8px 12px 20px;
}

.menu {
  border-right: none;
  background: transparent;
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
  background: rgba(255, 255, 255, 0.9);
  border-bottom: 1px solid #e5e7eb;
  position: sticky;
  top: 0;
  z-index: 10;
  backdrop-filter: blur(8px);
}

.title {
  font-size: 20px;
  font-weight: 700;
}

.header-right {
  display: flex;
  align-items: center;
  gap: 12px;
}

.content {
  padding: 20px 24px 32px;
}

@media (max-width: 900px) {
  .layout {
    grid-template-columns: 1fr;
  }

  .sider {
    display: none;
  }
}
</style>
