<template>
  <div class="referral-page">
    <ReferralNav />

    <div class="referral-header">
      <h2 class="referral-title">推广素材</h2>
    </div>

    <section class="referral-panel">
      <div class="panel-head">
        <span class="section-title">推广链接与邀请码</span>
      </div>
      <div class="promo">
        <div class="promo__row">
          <span class="promo__label">邀请码</span>
          <span class="promo__value promo__value--code">{{ profile?.invite_code || '—' }}</span>
          <t-button size="small" variant="outline" :disabled="!profile?.invite_code" @click="copy(inviteCode, '邀请码已复制')">
            复制
          </t-button>
        </div>
        <div class="promo__row">
          <span class="promo__label">推广链接</span>
          <span class="promo__value">{{ inviteUrl }}</span>
          <t-button size="small" variant="outline" :disabled="!profile?.invite_code" @click="copy(inviteUrl, '推广链接已复制')">
            复制
          </t-button>
        </div>
        <p class="promo__tip">链接已内置你的邀请码，好友注册后自动归属为你的邀请关系。</p>
      </div>
    </section>

    <section class="referral-panel">
      <div class="panel-head">
        <span class="section-title">推广文案模板</span>
      </div>
      <div class="templates">
        <div v-for="tpl in templates" :key="tpl.label" class="template">
          <div class="template__head">
            <span class="template__label">{{ tpl.label }}</span>
            <t-button size="small" variant="text" theme="primary" @click="copy(tpl.render(inviteUrl, inviteCode), '文案已复制')">
              复制文案
            </t-button>
          </div>
          <p class="template__body">{{ tpl.render(inviteUrl, inviteCode) }}</p>
        </div>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'

import { getReferralProfile, type ReferralProfile } from '@/api/referral'
import ReferralNav from '@/components/referral-nav/index.vue'
import { useBrandStore } from '@/store/modules/brand'

defineOptions({ name: 'ReferralMaterials' })

const brandStore = useBrandStore()
const profile = ref<ReferralProfile | null>(null)

const inviteCode = computed(() => profile.value?.invite_code || '')
const inviteUrl = computed(() => {
  if (!inviteCode.value) return '—'
  return `${window.location.origin}/register?invite_code=${inviteCode.value}`
})

/*
 * 文案模板目前写在前端（管理侧尚无策展页，见 doc88 清点表 B3）。
 * 品牌名从站点配置取，避免模板里再写死一份品牌。
 *
 * 措辞边界：返现是**发给邀请人**的，被邀请人拿不到折扣或赠金
 *（`AccrueForOrder` 只给 inviter 记账，`coupons` 表为空且下单链路不读券）。
 * 因此模板里不能写「注册享优惠」「专属折扣」——推广者把这话发给朋友，
 * 朋友注册后发现没有任何优惠，受损的是推广者本人的信用。
 * 这里只承诺邀请关系确实会建立（好友注册后归属到你名下并产生返现）。
 */
const templates = computed(() => {
  const brand = brandStore.name
  return [
    {
      label: '社群短文案',
      render: (url: string) => `我在用${brand}，这是它的注册链接：${url}`,
    },
    {
      label: '邀请码文案',
      render: (_url: string, code?: string) => `注册${brand}时填邀请码 ${code || '—'}。`,
    },
    {
      label: '长文案',
      render: (url: string) =>
        `${brand}是一个云主机订购与运维平台，从选规格到开通都在同一个控制台里完成。用这个链接注册，或在注册时填我的邀请码即可：${url}`,
    },
  ]
})

async function copy(value: string, successMessage: string) {
  if (!value || value === '—') {
    MessagePlugin.warning('暂无可复制内容')
    return
  }
  try {
    await navigator.clipboard.writeText(value)
    MessagePlugin.success(successMessage)
  } catch {
    MessagePlugin.warning('复制失败，请手动选择内容')
  }
}

async function load() {
  try {
    const { data } = await getReferralProfile()
    profile.value = data
  } catch (e: any) {
    MessagePlugin.error(e?.message || '加载推广信息失败')
  }
}

onMounted(load)
</script>

<style scoped>
.referral-page { padding: 16px 24px; display: flex; flex-direction: column; gap: 16px; }
.referral-header { display: flex; align-items: center; justify-content: space-between; }
.referral-title { font-size: 20px; font-weight: 700; margin: 0; }
.referral-panel {
  background: #fff; border: 1px solid var(--td-border-level-1-color, #f0f0f0);
  border-radius: 12px; padding: 16px 20px;
}
.panel-head { margin-bottom: 12px; }
.section-title { font-size: 15px; font-weight: 700; }
.promo { display: flex; flex-direction: column; gap: 10px; }
.promo__row { display: flex; align-items: center; gap: 12px; font-size: 14px; }
.promo__label { width: 80px; color: #888; }
.promo__value { color: #333; word-break: break-all; }
.promo__value--code { font-family: monospace; font-weight: 700; color: #b76a00; }
.promo__tip { color: #999; font-size: 12px; margin: 0; }
.templates { display: flex; flex-direction: column; gap: 12px; }
.template {
  border: 1px dashed var(--td-border-level-1-color, #e6e6e6);
  border-radius: 10px; padding: 12px 14px;
}
.template__head { display: flex; align-items: center; justify-content: space-between; margin-bottom: 6px; }
.template__label { font-size: 13px; font-weight: 600; color: #444; }
.template__body { margin: 0; color: #666; font-size: 13px; line-height: 1.7; word-break: break-all; }
</style>
