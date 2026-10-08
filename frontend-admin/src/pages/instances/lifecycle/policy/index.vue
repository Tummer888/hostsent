<template>
  <div class="page-body lifecycle-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <SettingIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">生命周期策略</h2>
        </div>
      </div>
    </header>

    <t-loading :loading="loading" show-overlay>
      <section class="policy-card surface-card">
        <div class="policy-grid">
          <div class="policy-item">
            <div class="policy-item__head">
              <h4>到期提醒天数</h4>
              <t-tag variant="light" size="small" shape="round">提醒策略</t-tag>
            </div>
            <p class="policy-item__desc">实例到期前提前 N 天发送提醒，多个天数用英文逗号分隔（如 7,3,1）。</p>
            <t-input v-model="form.remind_days" placeholder="7,3,1" />
          </div>
          <div class="policy-item">
            <div class="policy-item__head">
              <h4>自动续费默认开启</h4>
              <t-tag variant="light" size="small" shape="round">新实例默认值</t-tag>
            </div>
            <p class="policy-item__desc">新实例创建时自动续费开关的默认状态，用户可自行修改。</p>
            <t-switch v-model="form.auto_renew_default" />
          </div>
          <div class="policy-item">
            <div class="policy-item__head">
              <h4>宽限期（天）</h4>
              <t-tag variant="light" size="small" shape="round">到期后</t-tag>
            </div>
            <p class="policy-item__desc">实例到期后进入宽限期，仍可续费恢复，超过后暂停服务。</p>
            <t-input-number v-model="form.grace_days" :min="0" :max="90" theme="column" style="width: 160px" />
          </div>
          <div class="policy-item">
            <div class="policy-item__head">
              <h4>销毁保留期（天）</h4>
              <t-tag variant="light" size="small" shape="round">暂停后</t-tag>
            </div>
            <p class="policy-item__desc">实例暂停后保留数据的天数，超过后系统自动销毁且不可恢复。</p>
            <t-input-number v-model="form.destroy_keep_days" :min="1" :max="365" theme="column" style="width: 160px" />
          </div>
        </div>

        <div class="policy-flow">
          <span class="policy-flow__node">运行中</span>
          <t-icon name="arrow-right" size="16" />
          <span class="policy-flow__node policy-flow__node--warn">宽限期 {{ form.grace_days }} 天</span>
          <t-icon name="arrow-right" size="16" />
          <span class="policy-flow__node policy-flow__node--danger">暂停（保留 {{ form.destroy_keep_days }} 天）</span>
          <t-icon name="arrow-right" size="16" />
          <span class="policy-flow__node policy-flow__node--dark">销毁</span>
        </div>

        <div class="policy-actions">
          <t-button theme="primary" :loading="saving" @click="handleSave">
            <template #icon><SaveIcon aria-hidden="true" /></template>
            保存策略
          </t-button>
          <t-button variant="outline" :disabled="saving" @click="loadPolicy">重置</t-button>
        </div>
      </section>
    </t-loading>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { SaveIcon, SettingIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin } from 'tdesign-vue-next'

import { getLifecyclePolicy, updateLifecyclePolicy } from '@/api/lifecycle'

defineOptions({ name: 'LifecyclePolicy' })

const loading = ref(false)
const saving = ref(false)
const form = reactive({
  remind_days: '7,3,1',
  auto_renew_default: false,
  grace_days: 7,
  destroy_keep_days: 30,
})

async function loadPolicy() {
  loading.value = true
  try {
    const policy = await getLifecyclePolicy()
    form.remind_days = policy.remind_days || '7,3,1'
    form.auto_renew_default = !!policy.auto_renew_default
    form.grace_days = policy.grace_days ?? 7
    form.destroy_keep_days = policy.destroy_keep_days ?? 30
  } catch (e) {
    MessagePlugin.error((e as Error).message || '加载策略失败')
  } finally {
    loading.value = false
  }
}

function validateRemindDays(raw: string): boolean {
  const trimmed = raw.trim()
  if (!trimmed) return false
  return trimmed.split(',').every((part) => /^\d+$/.test(part.trim()) && Number(part.trim()) >= 0)
}

async function handleSave() {
  if (!validateRemindDays(form.remind_days)) {
    MessagePlugin.warning('提醒天数格式不正确，示例：7,3,1')
    return
  }
  saving.value = true
  try {
    const msg = await updateLifecyclePolicy({
      remind_days: form.remind_days.trim(),
      auto_renew_default: form.auto_renew_default,
      grace_days: form.grace_days,
      destroy_keep_days: form.destroy_keep_days,
    })
    MessagePlugin.success(msg || '策略已更新')
  } catch (e) {
    MessagePlugin.error((e as Error).message || '保存策略失败')
  } finally {
    saving.value = false
  }
}

onMounted(loadPolicy)
</script>

<style lang="css">
@import '../shared.css';
</style>

<style scoped>
.policy-card {
  padding: 28px 32px;
}
.policy-grid {
  display: grid;
  gap: 28px;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
}
.policy-item {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.policy-item__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.policy-item__head h4 {
  margin: 0;
  font-size: 15px;
  font-weight: 700;
  color: var(--color-foreground, #1e293b);
}
.policy-item__desc {
  margin: 0;
  font-size: 12px;
  line-height: 1.7;
  color: var(--color-muted-foreground, #94a3b8);
}
.policy-flow {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  margin-top: 28px;
  padding: 16px 20px;
  border-radius: 10px;
  background: var(--hs-surface-2, #f4f6fa);
}
.policy-flow__node {
  padding: 6px 16px;
  border-radius: 999px;
  font-size: 13px;
  font-weight: 600;
  background: #fff;
  color: #334155;
  border: 1px solid #e2e8f0;
}
.policy-flow__node--warn {
  background: #fffbeb;
  border-color: #fcd34d;
  color: #92400e;
}
.policy-flow__node--danger {
  background: #fef2f2;
  border-color: #fca5a5;
  color: #b91c1c;
}
.policy-flow__node--dark {
  background: #1e293b;
  border-color: #1e293b;
  color: #f8fafc;
}
.policy-actions {
  display: flex;
  gap: 12px;
  margin-top: 24px;
}
</style>
