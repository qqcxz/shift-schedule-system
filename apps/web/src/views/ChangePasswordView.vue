<template>
  <div class="password-page">
    <div class="password-card page-card">
      <div class="brand">
        <div class="logo">排</div>
        <div>
          <h1>首次登录需修改密码</h1>
          <p class="muted">请设置一个 6 位以上的新密码</p>
        </div>
      </div>

      <el-form :model="form" @submit.prevent>
        <el-form-item label="当前密码">
          <el-input v-model="form.currentPassword" type="password" show-password autocomplete="current-password" />
        </el-form-item>
        <el-form-item label="新密码">
          <el-input v-model="form.newPassword" type="password" show-password autocomplete="new-password" />
        </el-form-item>
        <el-form-item label="确认新密码">
          <el-input
            v-model="form.confirmPassword"
            type="password"
            show-password
            autocomplete="new-password"
            @keyup.enter="onSubmit"
          />
        </el-form-item>
        <el-button type="primary" size="large" style="width: 100%" :loading="loading" @click="onSubmit">
          保存并继续
        </el-button>
      </el-form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref } from 'vue';
import { useRouter } from 'vue-router';
import { ElMessage } from 'element-plus';
import { changePasswordApi } from '../api';
import { useAuthStore } from '../stores/auth';

const auth = useAuthStore();
const router = useRouter();
const loading = ref(false);
const form = reactive({
  currentPassword: '',
  newPassword: '',
  confirmPassword: '',
});

async function onSubmit() {
  if (!form.currentPassword || !form.newPassword) {
    ElMessage.warning('请填写当前密码和新密码');
    return;
  }
  if (form.newPassword.length < 6) {
    ElMessage.warning('新密码至少 6 位');
    return;
  }
  if (form.newPassword !== form.confirmPassword) {
    ElMessage.warning('两次输入的新密码不一致');
    return;
  }

  loading.value = true;
  try {
    await changePasswordApi({
      currentPassword: form.currentPassword,
      newPassword: form.newPassword,
    });
    ElMessage.success('密码已修改，请重新登录');
    auth.logout();
    router.push('/login');
  } catch (error) {
    ElMessage.error((error as Error).message);
  } finally {
    loading.value = false;
  }
}
</script>

<style scoped>
.password-page {
  min-height: 100vh;
  display: grid;
  place-items: center;
  padding: 24px;
  background:
    radial-gradient(circle at top left, rgba(64, 158, 255, 0.18), transparent 28%),
    radial-gradient(circle at bottom right, rgba(103, 194, 58, 0.16), transparent 30%),
    #eef2ff;
}

.password-card {
  width: min(440px, 100%);
}

.brand {
  display: flex;
  gap: 14px;
  align-items: center;
  margin-bottom: 24px;
}

.brand h1 {
  margin: 0 0 4px;
  font-size: 22px;
}

.brand p {
  margin: 0;
}

.logo {
  width: 56px;
  height: 56px;
  border-radius: 16px;
  display: grid;
  place-items: center;
  color: #fff;
  font-size: 24px;
  font-weight: 700;
  background: linear-gradient(135deg, #409eff, #67c23a);
}
</style>
