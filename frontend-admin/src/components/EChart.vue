<template>
  <div ref="el" class="echart" :style="{ height: containerHeight }"></div>
</template>

<script setup lang="ts">
import * as echarts from 'echarts/core'
import { BarChart, LineChart, PieChart } from 'echarts/charts'
import { GridComponent, LegendComponent, TitleComponent, TooltipComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import type { EChartsOption } from 'echarts'
import { computed, onBeforeUnmount, onMounted, ref, shallowRef, watch } from 'vue'

echarts.use([
  CanvasRenderer,
  PieChart,
  BarChart,
  LineChart,
  TooltipComponent,
  LegendComponent,
  GridComponent,
  TitleComponent,
])

defineOptions({ name: 'EChart' })

const props = defineProps<{
  option: EChartsOption
  height?: number | string
}>()

const emit = defineEmits<{ (e: 'click', payload: { name: string; dataIndex: number; seriesType?: string }): void }>()

/**
 * 容器高度：数字与「纯数字字符串」都按 px 处理。
 *
 * 必须归一化的原因（曾导致财务总览/财务报表「收支趋势」整块空白）：
 * prop 类型是 `number | string` 时 Vue **不做**数字转换（只有含 Boolean 的类型才置 shouldCast），
 * 所以模板里的 `height="300"` 到组件里是字符串 `"300"`；直接当 CSS 长度会得到无单位值，
 * 被浏览器整条丢弃 → 容器高度 0 → echarts 在 0 高度画布上绘制，页面表现为一片空白。
 * 写法上仍推荐 `:height="300"`，此处兜底是为了不再依赖写法正确。
 */
const containerHeight = computed(() => {
  const value = props.height
  if (value === undefined || value === null || value === '') return '300px'
  if (typeof value === 'number') return `${value}px`
  const text = String(value).trim()
  return /^\d+(\.\d+)?$/.test(text) ? `${text}px` : text
})

const el = ref<HTMLDivElement | null>(null)
const chart = shallowRef<echarts.ECharts | null>(null)
let observer: ResizeObserver | null = null
let rafId = 0
let resizeAttempts = 0

function render() {
  if (!chart.value) return
  chart.value.setOption(props.option, true)
}

/**
 * 首帧兜底：容器尚未完成布局（处于折叠/过渡/懒加载容器中）时 clientHeight 为 0，
 * echarts 会按 0 尺寸初始化并保持空白。此时逐帧重试 resize，最多 30 帧。
 */
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
  chart.value.on('click', (params: { name: string; dataIndex: number; seriesType?: string }) => {
    emit('click', { name: params.name, dataIndex: params.dataIndex, seriesType: params.seriesType })
  })
  render()
  ensureSized()
  if (typeof ResizeObserver !== 'undefined') {
    observer = new ResizeObserver(() => chart.value?.resize())
    observer.observe(el.value)
  }
})

watch(
  () => props.option,
  () => render(),
  { deep: true },
)

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
