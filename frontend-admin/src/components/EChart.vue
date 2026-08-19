<template>
  <div ref="el" class="echart" :style="{ height: typeof height === 'number' ? `${height}px` : height }"></div>
</template>

<script setup lang="ts">
import * as echarts from 'echarts/core'
import { BarChart, PieChart } from 'echarts/charts'
import { GridComponent, LegendComponent, TitleComponent, TooltipComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import type { EChartsOption } from 'echarts'
import { onBeforeUnmount, onMounted, ref, shallowRef, watch } from 'vue'

echarts.use([CanvasRenderer, PieChart, BarChart, TooltipComponent, LegendComponent, GridComponent, TitleComponent])

defineOptions({ name: 'EChart' })

const props = defineProps<{
  option: EChartsOption
  height?: number | string
}>()

const emit = defineEmits<{ (e: 'click', payload: { name: string; dataIndex: number; seriesType?: string }): void }>()

const el = ref<HTMLDivElement | null>(null)
const chart = shallowRef<echarts.ECharts | null>(null)
let observer: ResizeObserver | null = null

function render() {
  if (!chart.value) return
  chart.value.setOption(props.option, true)
}

onMounted(() => {
  if (!el.value) return
  chart.value = echarts.init(el.value)
  chart.value.on('click', (params: { name: string; dataIndex: number; seriesType?: string }) => {
    emit('click', { name: params.name, dataIndex: params.dataIndex, seriesType: params.seriesType })
  })
  render()
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
