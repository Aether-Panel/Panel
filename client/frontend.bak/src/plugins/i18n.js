import { ref } from 'vue'
import { createI18n } from 'vue-i18n'

let i18n = null
const ltr = ref(true)
const fallback = 'en_US'

const getLocale = () => {
  const stored = localStorage.getItem('locale')
  if (stored && localeList.indexOf(stored) !== -1) return stored
  const userLang = (navigator.language || navigator.userLanguage).replace('-', '_').toLowerCase()
  const test = lang => elem => elem.toLowerCase().indexOf(lang) !== -1
  const fromBrowser =
    localeList.filter(test(userLang))[0] ||
    localeList.filter(test(userLang.split('_')[0]))[0]
  if (fromBrowser) return fromBrowser
  return fallback
}

export default async () => {
  const locale = getLocale()
  i18n = createI18n({
    legacy: false,
    locale,
    fallbackLocale: fallback,
    missingWarn: true,
    fallbackWarn: true,
    silentTranslationWarn: false,
    warnHtmlMessage: false,
    allowComposition: true
  })
  await updateLocale(locale, false)

  const i18nInstall = i18n.install
  i18n.install = (app) => {
    app.provide('ltr', ltr)
    i18nInstall(app)
  }
  return i18n
}

const rtl = ['ar_SA', 'he_IL']
const files = ['common', 'env', 'errors', 'files', 'hotkeys', 'nodes', 'oauth', 'operators', 'scopes', 'servers', 'settings', 'templates', 'users', 'backup', 'plugins', 'uptime', 'admin', 'roles']
export async function updateLocale(locale, save = true) {
  if (save) {
    try {
      localStorage.setItem('locale', locale)
    } catch (e) {
      console.warn('No se pudo guardar el idioma en localStorage:', e)
    }
  }
  const messages = {}
  for (let i = 0; i < files.length; i++) {
    const file = files[i]
    try {
      const module = await import(`../lang/${locale}/${file}.json`)
      const fileMessages = module.default || module
      // Asegurar que los mensajes se carguen correctamente
      if (fileMessages && typeof fileMessages === 'object') {
        messages[file] = fileMessages
      } else {
        console.warn(`⚠️ Mensajes inválidos para ${file} en ${locale}`)
        messages[file] = {}
      }
    } catch (_) {
      try {
        const module = await import(`../lang/${fallback}/${file}.json`)
        const fileMessages = module.default || module
        if (fileMessages && typeof fileMessages === 'object') {
          messages[file] = fileMessages
        } else {
          messages[file] = {}
        }
      } catch (e) {
        console.error(`Error cargando traducción ${file} para ${locale}:`, e)
        messages[file] = {}
      }
    }
  }

  // Asegurar que los mensajes se establezcan correctamente
  if (i18n && i18n.global) {
    // Cargar mensajes del fallback primero si no existen
    if (!i18n.global.getLocaleMessage(fallback)) {
      const fallbackMessages = {}
      for (let i = 0; i < files.length; i++) {
        const file = files[i]
        try {
          const module = await import(`../lang/${fallback}/${file}.json`)
          const fileMessages = module.default || module
          if (fileMessages && typeof fileMessages === 'object') {
            fallbackMessages[file] = fileMessages
          } else {
            fallbackMessages[file] = {}
          }
        } catch (e) {
          fallbackMessages[file] = {}
        }
      }
      i18n.global.setLocaleMessage(fallback, fallbackMessages)
    }
    
    // Establecer los mensajes del locale actual directamente
    // vue-i18n debería resolver las claves como 'nodes.LocalNode' buscando en messages.nodes.LocalNode
    i18n.global.setLocaleMessage(locale, messages)
    i18n.global.locale.value = locale
  }

  if (document && document.querySelector) {
    const html = document.querySelector('html')
    if (html) {
      html.setAttribute('lang', locale)
      html.setAttribute('dir', rtl.indexOf(locale) === -1 ? 'ltr' : 'rtl')
    }
  }
  ltr.value = rtl.indexOf(locale) === -1 ? true : false
}

export const locales = localeList
  .filter(locale => {
    // Filtrar solo locales válidos (formato xx_XX)
    return /^[a-z]{2}_[A-Z]{2}$/.test(locale)
  })
  .map(locale => {
    try {
      let [lang, region] = locale.split('_')

      if (locale === 'sr_SP') {
        // crodwin uses the wrong country code for serbia, so we need to manually fix it
        region = 'RS'
      }

      const f = new Intl.DisplayNames(lang, { type: 'language', languageDisplay: 'standard' })
      return { value: locale, label: f.of(`${lang}-${region}`) }
    } catch (e) {
      console.warn(`⚠️ Locale inválido ignorado: ${locale}`, e)
      return null
    }
  })
  .filter(locale => locale !== null)
