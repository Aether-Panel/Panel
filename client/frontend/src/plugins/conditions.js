function resolveIf() {
  return true
}

export default {
  install: async (app) => {
    app.config.globalProperties.$conditions = resolveIf
    app.provide('conditions', resolveIf)
    window.SkyPanel.conditions = resolveIf
  }
}
