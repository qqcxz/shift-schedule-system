<template>
  <div class="page-card">
    <div class="toolbar">
      <div>
        <div class="section-title">员工管理</div>
        <div class="muted">店长可新增员工、修改姓名/账号/密码/角色，停用后不再出现在排班表</div>
      </div>
      <div class="right">
        <el-switch
          v-model="includeInactive"
          active-text="显示已停用"
          @change="loadUsers"
        />
        <el-button @click="loadUsers" :loading="loading">刷新</el-button>
        <el-button type="primary" @click="openCreate">新增员工</el-button>
      </div>
    </div>

    <el-table :data="users" v-loading="loading" stripe>
      <el-table-column prop="displayName" label="姓名" min-width="120" />
      <el-table-column prop="username" label="登录账号" min-width="120" />
      <el-table-column label="角色" width="100">
        <template #default="{ row }">
          <el-tag :type="row.role === 'manager' ? 'warning' : 'success'">
            {{ row.role === 'manager' ? '店长' : '店员' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="100">
        <template #default="{ row }">
          <el-tag :type="row.isActive === false ? 'info' : 'success'">
            {{ row.isActive === false ? '已停用' : '在职' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="260" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
          <el-button
            v-if="row.isActive !== false"
            link
            type="warning"
            @click="onToggleActive(row, false)"
          >
            停用
          </el-button>
          <el-button
            v-else
            link
            type="success"
            @click="onToggleActive(row, true)"
          >
            启用
          </el-button>
          <el-button link type="danger" @click="onDelete(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="dialogVisible" :title="editingId ? '编辑员工' : '新增员工'" width="520px">
      <el-form label-width="96px">
        <el-form-item label="姓名">
          <el-input v-model="form.displayName" placeholder="例如：店员小王" maxlength="32" />
        </el-form-item>
        <el-form-item label="登录账号">
          <el-input v-model="form.username" placeholder="例如：staff4" maxlength="32" />
        </el-form-item>
        <el-form-item :label="editingId ? '新密码' : '密码'">
          <el-input
            v-model="form.password"
            type="password"
            show-password
            :placeholder="editingId ? '不修改请留空' : '至少 6 位'"
            maxlength="64"
          />
        </el-form-item>
        <el-form-item label="角色">
          <el-radio-group v-model="form.role">
            <el-radio-button value="staff">店员</el-radio-button>
            <el-radio-button value="manager">店长</el-radio-button>
          </el-radio-group>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="onSave">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted, reactive, ref } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import {
  createUserApi,
  deleteUserApi,
  updateUserApi,
  usersApi,
  type User,
} from '../api';
import { onRealtime } from '../realtime';

const loading = ref(false);
const saving = ref(false);
const dialogVisible = ref(false);
const includeInactive = ref(true);
const editingId = ref('');
const users = ref<User[]>([]);
const form = reactive({
  displayName: '',
  username: '',
  password: '',
  role: 'staff' as 'manager' | 'staff',
});

let offRealtime: Array<() => void> = [];

function resetForm() {
  form.displayName = '';
  form.username = '';
  form.password = '';
  form.role = 'staff';
}

async function loadUsers() {
  loading.value = true;
  try {
    const { data } = await usersApi(includeInactive.value);
    users.value = data || [];
  } catch (error) {
    ElMessage.error((error as Error).message);
  } finally {
    loading.value = false;
  }
}

function openCreate() {
  editingId.value = '';
  resetForm();
  dialogVisible.value = true;
}

function openEdit(row: User) {
  editingId.value = row.id;
  form.displayName = row.displayName;
  form.username = row.username;
  form.password = '';
  form.role = row.role;
  dialogVisible.value = true;
}

async function onSave() {
  if (!form.displayName.trim() || !form.username.trim()) {
    ElMessage.warning('请填写姓名和登录账号');
    return;
  }
  if (!editingId.value && form.password.length < 6) {
    ElMessage.warning('密码至少 6 位');
    return;
  }
  if (editingId.value && form.password && form.password.length < 6) {
    ElMessage.warning('新密码至少 6 位');
    return;
  }

  saving.value = true;
  try {
    if (editingId.value) {
      await updateUserApi(editingId.value, {
        displayName: form.displayName.trim(),
        username: form.username.trim(),
        role: form.role,
        password: form.password || undefined,
      });
      ElMessage.success('员工信息已更新');
    } else {
      await createUserApi({
        displayName: form.displayName.trim(),
        username: form.username.trim(),
        password: form.password,
        role: form.role,
      });
      ElMessage.success('员工已创建');
    }
    dialogVisible.value = false;
    await loadUsers();
  } catch (error) {
    ElMessage.error((error as Error).message);
  } finally {
    saving.value = false;
  }
}

async function onToggleActive(row: User, isActive: boolean) {
  try {
    await updateUserApi(row.id, { isActive });
    ElMessage.success(isActive ? '已启用' : '已停用');
    await loadUsers();
  } catch (error) {
    ElMessage.error((error as Error).message);
  }
}

async function onDelete(row: User) {
  try {
    await ElMessageBox.confirm(
      `确认删除员工「${row.displayName}」？删除后将停用该账号，历史排班仍会保留。`,
      '删除员工',
      { type: 'warning' },
    );
    await deleteUserApi(row.id);
    ElMessage.success('员工已删除（已停用）');
    await loadUsers();
  } catch (error) {
    if ((error as Error).message && (error as Error).message !== 'cancel') {
      ElMessage.error((error as Error).message);
    }
  }
}

onMounted(async () => {
  await loadUsers();
  offRealtime = [
    onRealtime('users.updated', () => {
      loadUsers();
    }),
  ];
});

onUnmounted(() => {
  offRealtime.forEach((off) => off());
});
</script>

<style scoped>
.section-title {
  font-size: 18px;
  font-weight: 700;
  margin-bottom: 4px;
}

.right {
  display: flex;
  gap: 12px;
  align-items: center;
  flex-wrap: wrap;
}
</style>
