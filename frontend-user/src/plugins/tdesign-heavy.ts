// 页面级「重组件」出口：Table / DatePicker（首屏性能专项）。
//
// 为什么单独放这个文件、而不并入 plugins/tdesign.ts 的按需注册清单：
// 这两个组件源码合计约 600KB（Table 393KB + DatePicker 225KB），只有列表页/账单页用得到。
// 放进入口的全局注册会让控制台、登录页这类首屏白白多下载约 350KB JS；
// 而本文件只被用到它们的页面 import，代码会落到对应页面的异步 chunk 里。
//
// 为什么这里把类型放宽成 Component：
// 1) 入口全局注册时（改动前），<t-table> 走的是 TDesign 的 GlobalComponents 声明，
//    本工程并未加载该声明，也就是说这些组件的 props 从来就不参与类型检查；
// 2) 若按严格类型引入，TDesign 表格的列类型 PrimaryTableCol<T> 对行类型是反向协变的
//    （cell 回调吃 PrimaryTableCellParams<T>），页面按具体行类型声明的列
//    （PrimaryTableCol<OrderInfo>[]）无法赋给组件内默认的 TableRowData 版本，
//    会一次性炸出 20+ 处类型报错。
// 所以这里保持与改动前一致的宽松类型，页面模板写法不变（仍是 <t-table> / <t-date-picker>）。
import type { Component } from 'vue'
import { DatePicker, Table } from 'tdesign-vue-next'

export const TTable = Table as unknown as Component
export const TDatePicker = DatePicker as unknown as Component