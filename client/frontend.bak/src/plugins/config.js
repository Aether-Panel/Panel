import { reactive, watch } from 'vue'

export default (rawConfig) => {
  const config = reactive(rawConfig)
  
  return {
    install: (app) => {
      app.config.globalProperties.$config = config
      app.provide('config', config)

      document.title = config.branding.name
      
      // Actualizar el título del documento cuando cambie el nombre
      if (config.branding) {
        watch(() => config.branding.name, (newName) => {
          document.title = newName
        })
      }
    }
  }
}
