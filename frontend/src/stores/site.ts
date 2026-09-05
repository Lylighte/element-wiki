import { reactive } from 'vue'

// 站点公开信息（T10.1/T11.2）：App 首屏消费 /v1/site；设置保存后即时更新标题。
// commentsEnabled 为首载快照：null 表示站点信息未就绪，消费方自行兜底。
const state = reactive({ title: '', loaded: false, commentsEnabled: null as boolean | null })

function setTitle(title: string) {
  if (title) {
    state.title = title
    state.loaded = true
  }
}

function setCommentsEnabled(enabled: boolean) {
  state.commentsEnabled = enabled
}

export const siteStore = { state, setTitle, setCommentsEnabled }
export default siteStore
