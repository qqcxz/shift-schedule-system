<template>
  <div class="password-page">
    <div class="password-grid" aria-hidden="true" />
    <div class="password-shell">
      <div class="password-aside">
        <div class="aside-kicker"><span /> ACCOUNT / 02</div>
        <div class="aside-number">02</div>
        <h1>SET YOUR<br /><i>RHYTHM.</i></h1>
        <p>第一次进入工作台，请先建立一个只属于你的安全节奏。</p>
      </div>
      <div class="password-card page-card">
        <div class="password-mark" aria-hidden="true"><KeyRound :size="21" /></div>
        <div class="password-kicker">FIRST ACCESS / SECURITY</div>
        <h2>设置新密码</h2>
        <p class="muted">请设置一个 6 位以上的新密码</p>
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
          <el-button class="password-submit" type="primary" size="large" :loading="loading" @click="onSubmit">
            保存并继续 <ArrowUpRight :size="17" />
          </el-button>
        </el-form>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref } from 'vue';
import { useRouter } from 'vue-router';
import { ElMessage } from 'element-plus';
import { ArrowUpRight, KeyRound } from '@lucide/vue';
import { changePasswordApi } from '../api';
import { useAuthStore } from '../stores/auth';

const auth = useAuthStore();
const router = useRouter();
const loading = ref(false);
const form = reactive({ currentPassword: '', newPassword: '', confirmPassword: '' });

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
    await changePasswordApi({ currentPassword: form.currentPassword, newPassword: form.newPassword });
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
.password-page { position: relative; min-height: 100vh; display: grid; place-items: center; overflow: hidden; background: #171a17; }
.password-grid { position: absolute; inset: 0; opacity: .17; background-image: linear-gradient(rgba(216,255,85,.22) 1px, transparent 1px), linear-gradient(90deg, rgba(216,255,85,.22) 1px, transparent 1px); background-size: 72px 72px; mask-image: radial-gradient(ellipse at center, black, transparent 74%); }
.password-shell { position: relative; z-index: 1; display: grid; grid-template-columns: minmax(250px, .8fr) minmax(360px, 470px); gap: clamp(42px, 8vw, 130px); align-items: center; width: min(980px, calc(100% - 64px)); }
.password-aside { color: #f3f5ea; }
.aside-kicker, .password-kicker { color: #d8ff55; font-family: "SFMono-Regular", Consolas, monospace; font-size: 10px; letter-spacing: .13em; }
.aside-kicker { display: flex; align-items: center; gap: 10px; }
.aside-kicker span { width: 28px; height: 1px; background: #d8ff55; }
.aside-number { margin: 28px 0 12px; color: rgba(216,255,85,.15); font-family: "Arial Narrow", Impact, sans-serif; font-size: 120px; font-weight: 800; line-height: .75; letter-spacing: -.1em; }
.password-aside h1 { margin: 0; font-family: "Arial Narrow", Impact, sans-serif; font-size: clamp(52px, 7vw, 94px); font-weight: 800; letter-spacing: -.07em; line-height: .82; }
.password-aside h1 i { color: #d8ff55; font-style: normal; }
.password-aside p { max-width: 260px; margin-top: 26px; color: #9ba399; font-size: 13px; line-height: 1.8; }
.password-card { padding: 34px; background: #fafaf6; border: 0; border-radius: 3px; box-shadow: 24px 26px 0 rgba(216,255,85,.12), 0 25px 90px rgba(0,0,0,.3); }
.password-card::after { width: 110px; height: 5px; }
.password-mark { width: 44px; height: 44px; display: grid; place-items: center; margin-bottom: 28px; color: #171a17; background: #d8ff55; border-radius: 50%; }
.password-kicker { color: #8d9688; font-size: 9px; }
.password-card h2 { margin: 11px 0 7px; color: #1a1d19; font-family: "Arial Narrow", "PingFang SC", sans-serif; font-size: 32px; letter-spacing: -.05em; }
.password-card > .muted { margin: 0 0 30px; font-size: 13px; }
.password-card :deep(.el-input__wrapper) { min-height: 46px; }
.password-submit { display: inline-flex; align-items: center; gap: 8px; margin-top: 8px; }
@media (max-width: 720px) {
  .password-shell { grid-template-columns: 1fr; gap: 32px; width: min(470px, calc(100% - 32px)); padding: 38px 0; }
  .aside-number { display: none; }
  .password-aside h1 { margin-top: 22px; font-size: clamp(54px, 15vw, 84px); }
  .password-aside p { margin-top: 16px; }
  .password-card { padding: 26px 22px; }
}
</style>
