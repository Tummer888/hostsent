<template>
  <div class="page-body product-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip"><FileIcon size="22" aria-hidden="true" /></span>
        <div class="page-header__text">
          <h2 class="page-header__title">自定义规格</h2>
        </div>
      </div>
    </header>

    <section class="form-card surface-card">
      <h3 class="card-title">规格定义</h3>
      <t-form label-align="top" :data="form" @submit.prevent="save">
        <div class="form-grid">
          <t-form-item label="规格名称" name="name"><t-input v-model="form.name" placeholder="如：高并发-16核32G" /></t-form-item>
          <t-form-item label="规格族" name="spec_family"><t-select v-model="form.spec_family" :options="specFamilyOptions" /></t-form-item>
          <t-form-item label="CPU（核）" name="cpu"><t-input-number v-model="form.cpu" :min="1" theme="column" /></t-form-item>
          <t-form-item label="内存（GB）" name="memory"><t-input-number v-model="form.memory" :min="1" :precision="1" theme="column" /></t-form-item>
          <t-form-item label="系统盘（GB）" name="disk"><t-input-number v-model="form.disk" :min="1" theme="column" /></t-form-item>
          <t-form-item label="带宽（Mbps）" name="bandwidth"><t-input-number v-model="form.bandwidth" :min="0" theme="column" /></t-form-item>
          <t-form-item label="磁盘类型" name="disk_type"><t-select v-model="form.disk_type" :options="diskTypeOptions" /></t-form-item>
          <t-form-item label="参考售价（元）" name="price"><t-input-number v-model="form.price" :min="0" :precision="2" theme="column" /></t-form-item>
          <t-form-item label="排序" name="sort_order"><t-input-number v-model="form.sort_order" theme="column" placeholder="数值越小越靠前" /></t-form-item>
          <t-form-item label="状态" name="status"><t-select v-model="form.status" :options="statusOptions" /></t-form-item>
        </div>
        <t-form-item label="操作系统" name="os"><t-input v-model="form.os" placeholder="如：Debian 12" /></t-form-item>
        <t-form-item label="适用场景" name="description"><t-textarea v-model="form.description" :autosize="{ minRows: 2, maxRows: 4 }" /></t-form-item>
        <div class="form-footer">
          <t-button variant="outline" @click="reset">重置</t-button>
          <t-button theme="primary" type="submit" :loading="saving">保存规格</t-button>
        </div>
      </t-form>
    </section>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { FileIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin } from 'tdesign-vue-next'

import { createSpecTemplate } from '@/api/product'

defineOptions({ name: 'ProductSpecCustom' })

const router = useRouter()

const specFamilyOptions = [
  { label: '通用型', value: 'general' },
  { label: '计算型', value: 'compute' },
  { label: '内存型', value: 'memory' },
  { label: '存储型', value: 'storage' },
  { label: 'GPU型', value: 'gpu' },
]
const diskTypeOptions = [
  { label: 'SSD', value: 'ssd' },
  { label: 'HDD', value: 'hdd' },
]
const statusOptions = [
  { label: '启用', value: 1 },
  { label: '停用', value: 0 },
]

const saving = ref(false)
const form = reactive({ name: '', spec_family: 'general', cpu: 2, memory: 4, disk: 50, bandwidth: 5, disk_type: 'ssd', os: '', price: 0, sort_order: 0, status: 1, description: '' })

function reset() {
  Object.assign(form, { name: '', spec_family: 'general', cpu: 2, memory: 4, disk: 50, bandwidth: 5, disk_type: 'ssd', os: '', price: 0, sort_order: 0, status: 1, description: '' })
}

async function save() {
  if (!form.name) {
    MessagePlugin.warning('请填写规格名称')
    return
  }
  saving.value = true
  try {
    await createSpecTemplate({ ...form })
    MessagePlugin.success('自定义规格已保存')
    reset()
    router.push('/product/spec/templates')
  } catch (error) {
    MessagePlugin.error((error as Error).message || '保存失败')
  } finally {
    saving.value = false
  }
}
</script>

<style lang="css">
@import '../../shared.css';
</style>
