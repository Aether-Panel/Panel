const path = require('path')
const { defineConfig } = require('vite')

module.exports = defineConfig({
  build: {
    lib: {
      entry: path.resolve(__dirname, 'src/index.js'),
      name: 'SkyPanel',
      fileName: (format) => {
        if (format === 'es') {
          return 'SkyPanel.mjs'
        } else {
          return 'SkyPanel.cjs'
        }
      }
    },
    minify: false,
    rollupOptions: {
      external: ['axios'],
      output: {
        globals: {
          axios: 'axios'
        }
      }
    }
  }
})
