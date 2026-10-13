<template>
  <div ref="el" class="echart" :style="{ height: containerHeight }"></div>
</template>

<script setup lang="ts">
// ECharts 壳组件（与管理端 frontend-admin/src/components/EChart.vue 同源）。
//
// 为什么按需 import：全量 echarts 约 1MB，用户端只有详情页监控用到图表，
// 这里只注册用得到的 Line/Bar 与 Grid/Tooltip/Legend，代码落到详情页的异步 chunk。
import * as echarts from 'echarts/core'
import { BarChart, LineChart } from 'echarts/charts'
import { GridComponent, LegendComponent, TooltipComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import type { EChartsOption } from 'echarts'
import { computed, onBeforeUnmount, onMounted, ref, shallowRef, watch } from 'vue'

echarts.use([CanvasRenderer, LineChart, BarChart, TooltipComponent, LegendComponent, GridComponent])

defineOptions({ name: 'UserEChart' })

const props = defineProps<{
  option: EChartsOption
  height?: number | string
}>()

const el = ref<HTMLDivElement | null>(null)
const chart = shallowRef<echarts.ECharts | null>(null)
let observer: ResizeObserver | null = null
let rafId = 0
let resizeAttempts = 0

/**
 * 容器高度：数字与「纯数字字符串」都按 px 处理。
 *
 * 必须归一化的原因：prop 类型是 `number | string` 时 Vue 不做数字转换，
 * 模板里写 height="300" 到组件里是字符串 "300"，当 CSS 长度用会被整条丢弃
 * （无单位）→ 容器高度 0 → echarts 在 0 高度画布上绘制，页面一片空白。
 */
const containerHeight = computed(() => {
  const value = props.height
  if (value === undefined || value === null || value === '') return '260px'
  if (typeof value === 'number') return `${value}px`
  const text = String(value).trim()
  return /^\d+(\.\d+)?$/.test(text) ? `${text}px` : text
})

function render() {
  if (!chart.value) return
  chart.value.setOption(props.option, true)
}

/** 首帧兜底：容器尚在折叠/过渡中时 clientHeight 为 0，逐帧重试 resize（最多 30 帧）。 */
function ensureSized() {
  const node = el.value
  const instance = chart.value
  if (!node || !instance) return
  if (node.clientWidth > 0 && node.clientHeight > 0) {
    instance.resize()
    return
  }
  if (resizeAttempts >= 30) return
  resizeAttempts += 1
  rafId = requestAnimationFrame(ensureSized)
}

onMounted(() => {
  if (!el.value) return
  chart.value = echarts.init(el.value)
  render()
  ensureSized()
  if (typeof ResizeObserver !== 'undefined') {
    observer = new ResizeObserver(() => chart.value?.resize())
    observer.observe(el.value)
  }
})

watch(() => props.option, render, { deep: true })

onBeforeUnmount(() => {
  cancelAnimationFrame(rafId)
  observer?.disconnect()
  chart.value?.dispose()
  chart.value = null
})
</script>

<style scoped>
.echart {
  width: 100%;
}
</style>
