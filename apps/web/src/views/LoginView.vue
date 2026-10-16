<template>
  <div class="login-page">
    <div class="login-grid" aria-hidden="true" />
    <div class="login-orbit orbit-one" aria-hidden="true" />
    <div class="login-orbit orbit-two" aria-hidden="true" />
    <div class="login-shell">
      <section class="login-intro">
        <div class="login-eyebrow"><span class="intro-line" /> SHIFT / 01 — WORKSPACE</div>
        <h1>MAKE TIME<br /><i>VISIBLE.</i></h1>
        <p class="intro-copy">把每一次班次变更，变成团队都能看见的节奏。</p>
        <div class="intro-footer">
          <div class="intro-signal"><Activity :size="17" /><span>REAL-TIME<br />OPERATIONS</span></div>
          <ArrowUpRight :size="22" />
        </div>
      </section>

      <section class="login-card page-card">
        <div class="brand-lockup">
          <div class="login-logo" aria-hidden="true"><CalendarDays :size="22" :stroke-width="1.7" /></div>
          <div>
            <div class="login-brand-name">SHIFTBOARD</div>
            <div class="login-brand-sub">门店排班系统</div>
          </div>
        </div>
        <div class="login-heading">
          <div class="login-kicker">SECURE ACCESS</div>
          <h2>欢迎回来</h2>
          <p>登录以继续管理你的工作台</p>
        </div>

        <el-form :model="form" @submit.prevent>
          <el-form-item label="用户名">
            <el-input v-model="form.username" placeholder="请输入用户名" size="large" autocomplete="username" />
          </el-form-item>
          <el-form-item label="密码">
            <el-input
              v-model="form.password"
              type="password"
              show-password
              size="large"
              placeholder="请输入密码"
              autocomplete="current-password"
              @keyup.enter="onSubmit"
            />
          </el-form-item>
          <el-button class="login-submit" type="primary" size="large" :loading="loading" @click="onSubmit">
            <span>进入工作台</span><ArrowUpRight :size="18" />
          </el-button>
        </el-form>
        <div class="login-note"><LockKeyhole :size="13" /> 你的数据仅对授权成员可见</div>
      </section>
    </div>
    <div class="login-coordinates">31°13' N &nbsp;&nbsp; 121°28' E <span>SHIFTBOARD / 2025</span></div>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { ElMessage } from 'element-plus';
import { Activity, ArrowUpRight, CalendarDays, LockKeyhole } from '@lucide/vue';
import { useAuthStore } from '../stores/auth';
import { getLoginRedirect } from '../router/login-redirect';
import { showApiError } from '../api/feedback';

const auth = useAuthStore();
const router = useRouter();
const route = useRoute();
const loading = ref(false);
const form = reactive({ username: '', password: '' });

async function onSubmit() {
  if (!form.username.trim() || !form.password) {
    ElMessage.warning('请输入用户名和密码');
    return;
  }

  loading.value = true;
  try {
    await auth.login(form.username.trim(), form.password);
    ElMessage.success('登录成功');
    await router.replace(auth.mustChangePassword ? '/change-password' : getLoginRedirect(route.query.redirect));
  } catch (error) {
    showApiError(error);
  } finally {
    loading.value = false;
  }
}
</script>

<style scoped>
.login-page { position: relative; min-height: 100vh; display: grid; place-items: center; overflow: hidden; color: #f3f5ea; background: #171a17; }
.login-grid { position: absolute; inset: 0; opacity: .16; background-image: linear-gradient(rgba(216,255,85,.24) 1px, transparent 1px), linear-gradient(90deg, rgba(216,255,85,.24) 1px, transparent 1px); background-size: 72px 72px; mask-image: radial-gradient(ellipse at center, black 20%, transparent 76%); }
.login-orbit { position: absolute; border: 1px solid rgba(216,255,85,.18); border-radius: 50%; pointer-events: none; }
.orbit-one { width: min(70vw, 920px); height: min(70vw, 920px); left: -22vw; bottom: -42vw; }
.orbit-two { width: min(55vw, 720px); height: min(55vw, 720px); right: -25vw; top: -35vw; border-color: rgba(255,255,255,.1); }
.login-shell { position: relative; z-index: 1; display: grid; grid-template-columns: minmax(260px, .9fr) minmax(360px, 460px); gap: clamp(42px, 8vw, 132px); align-items: center; width: min(1100px, calc(100% - 64px)); }
.login-intro { padding-bottom: 30px; }
.login-eyebrow, .login-kicker, .login-coordinates { font-family: "SFMono-Regular", Consolas, monospace; letter-spacing: .14em; }
.login-eyebrow { display: flex; align-items: center; gap: 12px; color: #d8ff55; font-size: 10px; }
.intro-line { width: 34px; height: 1px; background: #d8ff55; }
.login-intro h1 { margin: 30px 0 22px; color: #f3f5ea; font-family: "Arial Narrow", Impact, sans-serif; font-size: clamp(70px, 10vw, 136px); font-weight: 800; letter-spacing: -.08em; line-height: .78; }
.login-intro h1 i { color: #d8ff55; font-style: normal; }
.intro-copy { max-width: 270px; margin: 0; color: #a3a99d; font-size: 14px; line-height: 1.8; }
.intro-footer { display: flex; align-items: center; justify-content: space-between; width: min(280px, 100%); margin-top: 70px; padding-top: 16px; color: #899186; border-top: 1px solid #383e36; }
.intro-signal { display: flex; align-items: center; gap: 10px; color: #d8ff55; }
.intro-signal span { font-family: "SFMono-Regular", Consolas, monospace; font-size: 9px; letter-spacing: .12em; line-height: 1.5; }
.login-card { padding: 34px; color: #1a1d19; background: rgba(250,250,246,.96); border: 0; border-radius: 3px; box-shadow: 24px 26px 0 rgba(216,255,85,.12), 0 25px 90px rgba(0,0,0,.3); }
.login-card::after { width: 110px; height: 5px; }
.brand-lockup { display: flex; align-items: center; gap: 12px; }
.login-logo { width: 43px; height: 43px; display: grid; place-items: center; color: #171a17; background: #d8ff55; border-radius: 50%; }
.login-brand-name { color: #1a1d19; font-size: 14px; font-weight: 800; letter-spacing: .08em; }
.login-brand-sub { margin-top: 4px; color: #858c80; font-size: 11px; }
.login-heading { margin: 56px 0 28px; }
.login-kicker { color: #8d9688; font-size: 9px; }
.login-heading h2 { margin: 11px 0 8px; font-family: "Arial Narrow", "PingFang SC", sans-serif; font-size: 32px; letter-spacing: -.05em; }
.login-heading p { margin: 0; color: #7b8377; font-size: 13px; }
.login-card :deep(.el-form-item) { margin-bottom: 19px; }
.login-card :deep(.el-input__wrapper) { height: 48px; }
.login-submit { width: 100%; justify-content: space-between; margin-top: 10px; padding: 0 16px 0 18px; }
.login-note { display: flex; align-items: center; gap: 6px; margin-top: 25px; color: #92998e; font-family: "SFMono-Regular", Consolas, monospace; font-size: 9px; letter-spacing: .03em; }
.login-coordinates { position: absolute; right: 32px; bottom: 24px; z-index: 1; color: #697167; font-size: 9px; }
.login-coordinates span { margin-left: 28px; color: #aab09f; }
@media (max-width: 760px) {
  .login-shell { grid-template-columns: 1fr; gap: 32px; width: min(460px, calc(100% - 32px)); padding: 38px 0 60px; }
  .login-intro { padding: 0; }
  .login-intro h1 { margin: 22px 0 14px; font-size: clamp(60px, 18vw, 100px); }
  .intro-footer { display: none; }
  .login-card { padding: 26px 22px; box-shadow: 12px 14px 0 rgba(216,255,85,.12), 0 25px 70px rgba(0,0,0,.28); }
  .login-heading { margin-top: 38px; }
  .login-coordinates { right: 16px; bottom: 14px; font-size: 8px; }
  .login-coordinates span { display: none; }
}
</style>
