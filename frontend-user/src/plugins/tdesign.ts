// TDesign 用户端按需注册（首屏性能专项）。
//
// 之前入口用的是 app.use(TDesign)：TDesign 的全量 install 会把全部约 70 个组件
// （Cascader / Upload / ColorPicker / Transfer / TreeSelect 等一大批本端根本没用到的
// 组件）连同各自的组件样式一起打进入口 chunk，首屏要多下载约 1.2MB JS 与 200KB CSS，
// 登录页这种只用得上几个组件的页面也一样要等这一坨。
//
// 这里只注册模板里真实用到的组件。模板写法不变（仍是 <t-xxx> 全局组件，页面无需改动），
// 只是把「谁在入口里」收敛到这份清单。新增组件用法时在下面补一行即可，漏掉的话
// 控制台会报「Failed to resolve component: t-xxx」。
//
// 注意：Loading 已内置注册 v-loading 指令（tdesign withInstall 的第三个参数），
// 所以页面里的 v-loading 不需要单独引入 LoadingPlugin。
//
// 另外两个「重组件」故意不在这里注册：Table 与 DatePicker（两者源码合计约 600KB，
// 且只有列表页/账单页用得到）。它们由页面自行按需引入：
//   import { Table as TTable } from 'tdesign-vue-next'   // 模板里仍写 <t-table>
// 这样它们的代码只进对应页面的异步 chunk，不会拖累控制台/登录页这类首屏。
import type { App } from 'vue'
import {
  Alert,
  Avatar,
  Button,
  Card,
  Checkbox,
  Descriptions,
  DescriptionsItem,
  Dialog,
  Drawer,
  Dropdown,
  Empty,
  Form,
  FormItem,
  Icon,
  Input,
  InputNumber,
  Link,
  Loading,
  Option,
  OptionGroup,
  Pagination,
  Popconfirm,
  Popup,
  Radio,
  RadioButton,
  RadioGroup,
  Select,
  Slider,
  Space,
  StepItem,
  Steps,
  Switch,
  TabPanel,
  Tabs,
  Tag,
  Textarea,
  Tooltip,
} from 'tdesign-vue-next'

const components = [
  Alert,
  Avatar,
  Button,
  Card,
  Checkbox,
  Descriptions,
  DescriptionsItem,
  Dialog,
  Drawer,
  Dropdown,
  Empty,
  Form,
  FormItem,
  Icon,
  Input,
  InputNumber,
  Link,
  Loading,
  Option,
  OptionGroup,
  Pagination,
  Popconfirm,
  Popup,
  Radio,
  RadioButton,
  RadioGroup,
  Select,
  Slider,
  Space,
  StepItem,
  Steps,
  Switch,
  TabPanel,
  Tabs,
  Tag,
  Textarea,
  Tooltip,
]

export function setupTDesign(app: App) {
  components.forEach((component) => app.use(component))
}