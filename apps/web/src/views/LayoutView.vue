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
          <el-button @click="onLogout">退出</el-button>
        </div>
      </header>
      <section class="content">
        <router-view />
      </section>
    </main>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { storeApi } from '../api';
import { useAuthStore } from '../stores/auth';

const auth = useAuthStore();
const route = useRoute();
const router = useRouter();
const storeName = ref('');

const pageTitle = computed(() => {
  if (route.path.startsWith('/requests')) return '申请与审批';
  if (route.path.startsWith('/notifications')) return '消息中心';
  if (route.path.startsWith('/shifts')) return '班次管理';
  if (route.path.startsWith('/staff')) return '员工管理';
  return '月度排班表';
});

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
