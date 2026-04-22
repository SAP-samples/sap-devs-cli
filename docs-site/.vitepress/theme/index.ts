import DefaultTheme from 'vitepress/theme'
import './style.css'
import FioriShellbar from './components/FioriShellbar.vue'
import FioriHome from './components/FioriHome.vue'
import { h } from 'vue'
import { useData } from 'vitepress'

export default {
  extends: DefaultTheme,
  Layout() {
    const { frontmatter } = useData()
    if (frontmatter.value.layout === 'FioriHome') {
      return h('div', [
        h(FioriShellbar),
        h(FioriHome),
      ])
    }
    return h(DefaultTheme.Layout, null, {
      'layout-top': () => h(FioriShellbar),
    })
  },
}
