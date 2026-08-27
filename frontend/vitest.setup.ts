// 全局测试装配点（jsdom 环境已启用）
import { config } from '@vue/test-utils'

// 未挂载真实 router 的组件测试统一用锚点 stub 渲染 RouterLink，避免解析警告。
config.global.stubs = {
  ...config.global.stubs,
  RouterLink: {
    name: 'RouterLink',
    props: { to: [String, Object] },
    template: '<a :href="String(to)"><slot /></a>',
  },
}
