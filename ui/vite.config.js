import vue from '@vitejs/plugin-vue'
import { visualizer } from "rollup-plugin-visualizer"
import { defineConfig } from 'vite'

export default defineConfig({
  plugins: [vue(), visualizer()],
  resolve: {
    dedupe: ["vue"],
  },
  build: {
    commonjsOptions: {
      transformMixedEsModules: true,
    },
  },
})
