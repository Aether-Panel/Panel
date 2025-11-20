import { ApiClient, ServerCookieSessionStore } from 'SkyPanel'

export const apiClient = new ApiClient(
  location.origin,
  new ServerCookieSessionStore()
)

export default {
  install: (app) => {
    app.config.globalProperties.$api = apiClient
    app.provide('api', apiClient)
    window.SkyPanel.api = apiClient
  }
}
