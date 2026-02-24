import { createRouter, createWebHistory } from 'vue-router'
import makeRoutes from './routes'
import defaultRoute from './defaultRoute'

export default function(api) {
  const router = createRouter({
    history: createWebHistory(),
    linkExactActiveClass: 'active',
    routes: makeRoutes(api),
    scrollBehavior (to, from, savedPosition) {
      if (savedPosition) {
        return savedPosition
      }
      if (to.hash) {
        return { selector: to.hash }
      }
      return { left: 0, top: 0 }
    }
  })

  router.beforeEach((to) => {
    if (to.meta.noAuth && api.auth.isLoggedIn()) {
        return defaultRoute(api)
    }

    if (!to.meta.noAuth && !api.auth.isLoggedIn()) {
      sessionStorage.setItem('returnTo', JSON.stringify({
        name: to.name,
        params: to.params,
        hash: to.hash,
        query: to.query
      }))
      return { name: 'Login' }
    }

    // Verificar permisos si la ruta requiere un permiso específico
    // Solo verificar si el permiso NO es true (true significa acceso libre para autenticados)
    if (to.meta && to.meta.permission && to.meta.permission !== true) {
      if (api.auth.isLoggedIn()) {
        // Verificar si tiene el permiso específico o es admin
        const hasPermission = api.auth.hasScope(to.meta.permission)
        const isAdmin = api.auth.hasScope('admin')
        if (!hasPermission && !isAdmin) {
          // Si no tiene permiso ni es admin, redirigir a la ruta por defecto
          return defaultRoute(api)
        }
      } else {
        // No está autenticado y la ruta requiere permiso
        return defaultRoute(api)
      }
    }

    return true
  })

  return router
}
