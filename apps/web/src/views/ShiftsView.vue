<template>
  <div class="page-card">
    <div class="toolbar">
      <div>
        <div class="section-title">班次管理</div>
        <div class="muted">店长可自定义班次名称、时间、颜色；修改后排班表会同步展示新时间</div>
      </div>
      <div class="right">
        <el-button @click="loadShifts" :loading="loading">刷新</el-button>
        <el-button type="primary" @click="openCreate">新增班次</el-button>
      </div>
    </div>

    <el-table :data="shifts" v-loading="loading" stripe>
      <el-table-column label="颜色" width="80">
        <template #default="{ row }">
          <span class="color-dot" :style="{ background: row.color }" />
        </template>
      </el-table-column>
      <el-table-column prop="name" label="班次名称" min-width="120" />
      <el-table-column prop="code" label="编码" min-width="120" />
      <el-table-column label="时间" min-width="160">
        <template #default="{ row }">
          <span v-if="row.isOff">休息</span>
          <span v-else>{{ row.startTime }} - {{ row.endTime }}</span>
        </template>
      </el-table-column>
      <el-table-column prop="sortOrder" label="排序" width="90" />
      <el-table-column label="类型" width="100">
        <template #default="{ row }">
          <el-tag :type="row.isOff ? 'info' : 'success'">{{ row.isOff ? '休息' : '上班' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="180" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
          <el-button link type="danger" @click="onDelete(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="dialogVisible" :title="editingId ? '编辑班次' : '新增班次'" width="520px">
      <el-form label-width="96px">
        <el-form-item label="班次名称">
          <el-input v-model="form.name" placeholder="例如：早班" maxlength="32" />
        </el-form-item>
        <el-form-item label="编码">
          <el-input v-model="form.code" placeholder="可选，默认按名称生成" maxlength="32" />
        </el-form-item>
        <el-form-item label="是否休息">
          <el-switch v-model="form.isOff" />
        </el-form-item>
        <el-form-item v-if="!form.isOff" label="开始时间">
          <el-time-select
            v-model="form.startTime"
            start="00:00"
            step="00:30"
            end="23:30"
            placeholder="开始时间"
            style="width: 100%"
          />
        </el-form-item>
        <el-form-item v-if="!form.isOff" label="结束时间">
          <el-time-select
            v-model="form.endTime"
            start="00:00"
            step="00:30"
            end="23:30"
            placeholder="结束时间"
            style="width: 100%"
          />
        </el-form-item>
        <el-form-item label="颜色">
          <div class="color-row">
            <el-color-picker v-model="form.color" />
            <el-input v-model="form.color" style="width: 140px" />
          </div>
        </el-form-item>
        <el-form-item label="排序">
          <el-input-number v-model="form.sortOrder" :min="0" :max="999" />
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
  createShiftApi,
  deleteShiftApi,
  shiftsApi,
  updateShiftApi,
  type ShiftTemplate,
} from '../api';
import { onRealtime } from '../realtime';

const loading = ref(false);
const saving = ref(false);
const dialogVisible = ref(false);
const editingId = ref('');
const shifts = ref<ShiftTemplate[]>([]);
const form = reactive({
  name: '',
  code: '',
  startTime: '09:00',
  endTime: '13:00',
  color: '#409EFF',
  sortOrder: 1,
  isOff: false,
});

let offRealtime: Array<() => void> = [];

function resetForm() {
  form.name = '';
  form.code = '';
  form.startTime = '09:00';
  form.endTime = '13:00';
  form.color = '#409EFF';
  form.sortOrder = (shifts.value[shifts.value.length - 1]?.sortOrder || 0) + 1;
  form.isOff = false;
}

async function loadShifts() {
  loading.value = true;
  try {
    const { data } = await shiftsApi();
    shifts.value = data || [];
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

function openEdit(row: ShiftTemplate) {
  editingId.value = row.id;
  form.name = row.name;
  form.code = row.code;
  form.startTime = row.startTime;
  form.endTime = row.endTime;
  form.color = row.color;
  form.sortOrder = row.sortOrder || 0;
  form.isOff = row.isOff;
  dialogVisible.value = true;
}

async function onSave() {
  if (!form.name.trim()) {
    ElMessage.warning('请填写班次名称');
    return;
  }
  if (!form.isOff && (!form.startTime || !form.endTime)) {
    ElMessage.warning('请填写班次时间');
    return;
  }

  saving.value = true;
  try {
    const payload = {
      name: form.name.trim(),
      code: form.code.trim() || undefined,
      startTime: form.isOff ? '00:00' : form.startTime,
      endTime: form.isOff ? '00:00' : form.endTime,
      color: form.color || '#409EFF',
      sortOrder: form.sortOrder,
      isOff: form.isOff,
    };
    if (editingId.value) {
      await updateShiftApi(editingId.value, payload);
      ElMessage.success('班次已更新');
    } else {
      await createShiftApi(payload);
      ElMessage.success('班次已创建');
    }
    dialogVisible.value = false;
    await loadShifts();
  } catch (error) {
    ElMessage.error((error as Error).message);
  } finally {
    saving.value = false;
  }
}

async function onDelete(row: ShiftTemplate) {
  try {
    await ElMessageBox.confirm(`确认删除班次「${row.name}」？若已被排班使用将无法删除。`, '删除班次', {
      type: 'warning',
    });
    await deleteShiftApi(row.id);
    ElMessage.success('班次已删除');
    await loadShifts();
  } catch (error) {
    if ((error as Error).message && (error as Error).message !== 'cancel') {
      ElMessage.error((error as Error).message);
    }
  }
}

onMounted(async () => {
  await loadShifts();
  offRealtime = [
    onRealtime('shifts.updated', () => {
      loadShifts();
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
}

.color-dot {
  display: inline-block;
  width: 18px;
  height: 18px;
  border-radius: 50%;
  vertical-align: middle;
}

.color-row {
  display: flex;
  align-items: center;
  gap: 12px;
}
</style>
