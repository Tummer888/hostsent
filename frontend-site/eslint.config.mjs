import withNuxt from './.nuxt/eslint.config.mjs'

export default withNuxt({
  rules: {
    // 与两 Vite 前端同一口径：命名/风格由 Prettier 承担，这里只管代码正确性。
    'vue/multi-word-component-names': 'off',
    // Nuxt 预设要求 void 元素不加自闭合斜杠，而 Prettier 3 恰好相反（自动补 />），
    // 两者不可调和；两个 Vite 前端（vue/recommended）本就不启用此规则，这里对齐关闭。
    'vue/html-self-closing': 'off',
  },
})
