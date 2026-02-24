import { createApp } from 'vue'
import makeI18n from '@/plugins/i18n'
import api, {apiClient} from '@/plugins/api'
import clickOutside from '@/plugins/clickOutside'
import conditions from '@/plugins/conditions'
import configPlugin from '@/plugins/config'
import events from '@/plugins/events'
import hotkeys from '@/plugins/hotkeys'
import theme, { BUILTIN_THEME_NAMES } from '@/plugins/theme'
import toast from '@/plugins/toast'
import userSettings from '@/plugins/userSettings'
import validators from '@/plugins/validators'
import makeRouter from '@/router'
import App from "@/App.vue"
import '@/assets/main.css' // Importar Tailwind CSS

const checkEnv = !!import.meta.env.VITE_CHECK_ENV
if (/app\.github\.dev/.test(window.location.host) && checkEnv) {
  const err = document.createElement('div')
  err.style.border = '16px solid red'
  err.style.backgroundColor = 'rgb(30, 41, 59)'
  err.style.color = 'rgb(226, 232, 240)'
  err.style.textAlign = 'center'
  err.style.display = 'flex'
  err.style.flexDirection = 'column'
  err.innerHTML = `
<h1 style="margin:1em">!! IMPORTANT NOTICE !!</h1>
<h2 style="margin:1em">Usage of GitHub Codespaces for hosting is not permitted</h2>
<p style="text-wrap:balance;max-width:75%;align-self:center;line-height:1.5;margin:1em">
  Please read the <a href="https://docs.github.com/en/site-policy/github-terms/github-terms-for-additional-products-and-features#codespaces">GitHub Codespaces Terms of Service</a>.<br/>
  These explicitly state that the usage of GitHub Codespaces for
    <code style="background-color:rgb(51, 65, 85);border:1px solid rgb(148, 163, 184);color:rgb(226, 232, 240);">any other activity unrelated to the development or testing of the software project associated with the repository where GitHub Codespaces is initiated</code>
  is not permitted.
</p>
<blockquote style="text-wrap:balance;max-width:75%;border-left:8px solid rgb(148, 163, 184);align-self:center;background-color:rgb(30, 41, 59);color:rgb(226, 232, 240);margin:1em;padding:1em;border-radius:0.5rem;">
  In order to prevent violations of these limitations and abuse of GitHub Codespaces, GitHub may monitor your use of GitHub Codespaces.
  Misuse of GitHub Codespaces may result in termination of your access to Codespaces, restrictions in your ability to use GitHub Codespaces,
  or the disabling of repositories created to run Codespaces in a way that violates these Terms.
</blockquote>
<h4 style="text-wrap:balance;max-width:75%;align-self:center;margin:1em">
  The SkyPanel team does not tolerate or support the use of SkyPanel in order to facilitate violations against the GitHub Terms of Service
</h4>
`
  document.getElementById('app').appendChild(err)
  document.getElementById('hideApp').remove()
  throw new Error('github codespaces detected')
}

if ('serviceWorker' in navigator) {
  navigator.serviceWorker.register('/sw.js', { scope: '/' })
}

window.SkyPanel = {}

function normalizeConfig(rawConfig = {}) {
  const config = {...rawConfig}
  const themes = {...config.themes}
  themes.active = themes.active === 'SkyPanel' ? 'Default' : (themes.active || 'Default')
  const normalizedAvailable = (themes.available || ['Default']).map(themeName =>
    themeName === 'SkyPanel' ? 'Default' : themeName
  )
  themes.available = Array.from(new Set([...BUILTIN_THEME_NAMES, ...normalizedAvailable]))
  if (!themes.available.includes(themes.active)) {
    themes.available.push(themes.active)
  }
  themes.settings = themes.settings || '{}'
  config.themes = themes
  
  // Asegurar que branding existe
  if (!config.branding) {
    config.branding = {}
  }
  if (!config.branding.name) {
    config.branding.name = 'SkyPanel'
  }
  
  return config
}

async function mountApp(config) {
  const normalizedConfig = normalizeConfig(config)
  createApp(App)
    .use(api)
    .use(events)
    .use(configPlugin(normalizedConfig))
    .use(userSettings(apiClient))
    .use(clickOutside)
    .use(conditions)
    .use(await makeI18n())
    .use(theme(normalizedConfig))
    .use(toast)
    .use(validators)
    .use(makeRouter(apiClient))
    .use(hotkeys)
    .mount('#app')
}

apiClient
  .getConfig()
  .then(config => {
    mountApp(config)
  })
  .catch(error => {
    console.log(error)
    mountApp({
      branding: {
        name: 'SkyPanel'
      },
      themes: {
        active: 'Default',
        available: ['Default'],
        settings: '{}'
      },
      registrationEnabled: true
    })
  })
