<template>
  <div class="login-page">
    <div class="login-card page-card">
      <div class="brand">
        <div class="logo">排</div>
        <div>
          <h1>门店排班系统</h1>
          <p class="muted">店长排班 · 店员申请 · 实时同步</p>
        </div>
      </div>

      <el-form :model="form" @submit.prevent>
        <el-form-item label="用户名">
          <el-input v-model="form.username" placeholder="manager / staff1" size="large" />
        </el-form-item>
        <el-form-item label="密码">
          <el-input v-model="form.password" type="password" show-password size="large" @keyup.enter="onSubmit" />
        </el-form-item>
        <el-button type="primary" size="large" style="width: 100%" :loading="loading" @click="onSubmit">
          登录
        </el-button>
      </el-form>

      <div class="tips muted">
        演示账号：manager / staff1 / staff2 / staff3，密码均为 123456
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref } from 'vue';
import { useRouter } from 'vue-router';
import { ElMessage } from 'element-plus';
import { useAuthStore } from '../stores/auth';

const auth = useAuthStore();
const router = useRouter();
const loading = ref(false);
const form = reactive({
  username: 'manager',
  password: '123456',
});

async function onSubmit() {
  loading.value = true;
  try {
    await auth.login(form.username, form.password);
    ElMessage.success('登录成功');
    router.push('/schedule');
  } catch (error) {
    ElMessage.error((error as Error).message);
  } finally {
    loading.value = false;
  }
}
</script>

<style scoped>
.login-page {
  min-height: 100vh;
  display: grid;
  place-items: center;
  padding: 24px;
  background:
    radial-gradient(circle at top left, rgba(64, 158, 255, 0.18), transparent 28%),
    radial-gradient(circle at bottom right, rgba(103, 194, 58, 0.16), transparent 30%),
    #eef2ff;
}

.login-card {
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
  font-size: 24px;
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

.tips {
  margin-top: 18px;
  font-size: 13px;
  line-height: 1.6;
}
</style>
