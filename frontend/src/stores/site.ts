import { reactive } from 'vue'

// 站点公开信息（T10.1/T11.2）：App 首屏消费 /v1/site；设置保存后即时更新标题。
const state = reactive({ title: '', loaded: false })

function setTitle(title: string) {
  if (title) {
    state.title = title
    state.loaded = true
  }
}

export const siteStore = { state, setTitle }
export default siteStore
