import path from 'path'
import { fileURLToPath } from 'url'
import { defineConfig } from "vite"
import vue from "@vitejs/plugin-vue"
import eslint from "vite-plugin-eslint"
import vueI18n from '@intlify/unplugin-vue-i18n/vite'
import fs from 'fs'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)

export default defineConfig(({ mode }) => ({
  build: {
    sourcemap: mode === 'development' ? false : true,
    chunkSizeWarningLimit: 550,
    rollupOptions: {
      output: {
        entryFileNames: 'js/[name]-[hash].js',
        chunkFileNames: 'js/[name]-[hash].js',
        assetFileNames: '[ext]/[name]-[hash][extname]'
      }
    }
  },
  resolve: {
    alias: [
      {find: "@", replacement: path.resolve(__dirname, 'src')},
      {find: "SkyPanel", replacement: path.resolve(__dirname, '../api')}
    ]
  },
  define: {
    localeList: fs.readdirSync('src/lang', { withFileTypes: true })
      .filter(dirent => dirent.isDirectory() && /^[a-z]{2}_[A-Z]{2}$/.test(dirent.name))
      .map(dirent => dirent.name)
  },
  plugins: [
    vue(),
    vueI18n({
      runtimeOnly: false,
      include: path.resolve(__dirname, '@/lang/**')
    }),
    eslint()
  ]
}))
