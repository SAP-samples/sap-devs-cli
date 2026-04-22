import DefaultTheme from 'vitepress/theme'
import './style.css'
import FioriShellbar from './components/FioriShellbar.vue'
import FioriHome from './components/FioriHome.vue'
import { h } from 'vue'

export default {
  extends: DefaultTheme,
  Layout() {
    return h(DefaultTheme.Layout, null, {
      'layout-top': () => h(FioriShellbar),
    })
  },
  enhanceApp({ app }: { app: any }) {
    app.component('FioriHome', FioriHome)
  },
}
