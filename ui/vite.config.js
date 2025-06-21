import vue from '@vitejs/plugin-vue'
import { visualizer } from "rollup-plugin-visualizer"
import viteCompression from 'vite-plugin-compression'
import { defineConfig } from 'vite'

export default defineConfig({
  plugins: [
    vue(),
    viteCompression(),
    visualizer(),
  ],
  resolve: {
    dedupe: ["vue"],
    alias: {
      'Quill': './src/mocks/quill.js'
    }
  },
  build: {
    commonjsOptions: {
      transformMixedEsModules: true,
    }
  },
})
