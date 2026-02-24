<script setup>
import { inject } from 'vue'
import { useRouter } from 'vue-router'
import Icon from './Icon.vue'

const router = useRouter()
const api = inject('api')

const currentYear = new Date().getFullYear()

const isLoggedIn = api?.auth?.isLoggedIn()
const hasAdminAccess = isLoggedIn && api?.auth?.hasScope('admin')

function navigateTo(routeName) {
  router.push({ name: routeName })
}
</script>

<template>
  <footer 
    :class="[
      'w-full',
      'bg-background border-t-2 border-border/50',
      'mt-auto'
    ]"
  >
    <!-- Contenido principal del footer -->
    <div 
      :class="[
        'max-w-7xl mx-auto',
        'px-6 py-12'
      ]"
    >
      <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-8 mb-10">
        <!-- Sección: Sobre Aether Panel -->
        <div class="space-y-4">
          <div class="flex items-center space-x-3 mb-3">
            <img 
              src="/img/resources/image.png" 
              alt="Aether Panel Logo"
              class="w-24 h-24 object-contain"
            />
            <h3 class="text-lg font-bold text-foreground">Aether Panel</h3>
          </div>
          <p class="text-sm text-muted-foreground leading-relaxed">
            Plataforma de gestión de servidores de nueva generación. Potente, moderna y fácil de usar.
          </p>
          <div class="flex space-x-3 pt-2">
            <a 
              href="https://github.com" 
              target="_blank"
              rel="noopener noreferrer"
              class="text-muted-foreground hover:text-primary transition-colors"
              title="GitHub"
            >
              <icon name="bi-github" class="text-2xl" />
            </a>
            <a 
              href="https://discord.gg/cNW5UFthRX" 
              target="_blank"
              rel="noopener noreferrer"
              class="text-muted-foreground hover:text-primary transition-colors"
              title="Discord"
            >
              <icon name="bi-discord" class="text-2xl" />
            </a>
            <a 
              href="https://twitter.com" 
              target="_blank"
              rel="noopener noreferrer"
              class="text-muted-foreground hover:text-primary transition-colors"
              title="Twitter"
            >
              <icon name="bi-twitter" class="text-2xl" />
            </a>
          </div>
        </div>

        <!-- Sección: Navegación -->
        <div class="space-y-4">
          <h4 class="text-base font-semibold text-foreground mb-3">
            Navegación
          </h4>
          <ul class="space-y-2.5">
            <li v-if="isLoggedIn">
              <button 
                class="text-sm text-muted-foreground hover:text-primary hover:underline transition-colors"
                @click="navigateTo('ServerList')"
              >
                Servidores
              </button>
            </li>
            <li v-if="isLoggedIn">
              <button 
                class="text-sm text-muted-foreground hover:text-primary hover:underline transition-colors"
                @click="navigateTo('NodeList')"
              >
                Nodos
              </button>
            </li>
            <li v-if="hasAdminAccess">
              <button 
                class="text-sm text-muted-foreground hover:text-primary hover:underline transition-colors"
                @click="navigateTo('Admin.UserList')"
              >
                Usuarios
              </button>
            </li>
            <li v-if="hasAdminAccess">
              <button 
                class="text-sm text-muted-foreground hover:text-primary hover:underline transition-colors"
                @click="navigateTo('Admin')"
              >
                Panel de Administración
              </button>
            </li>
          </ul>
        </div>

        <!-- Sección: Recursos -->
        <div class="space-y-4">
          <h4 class="text-base font-semibold text-foreground mb-3">
            Recursos
          </h4>
          <ul class="space-y-2.5">
            <li>
              <a 
                href="https://docs.example.com" 
                target="_blank"
                rel="noopener noreferrer"
                class="text-sm text-muted-foreground hover:text-primary hover:underline transition-colors"
              >
                Documentación
              </a>
            </li>
            <li>
              <a 
                href="https://api.example.com" 
                target="_blank"
                rel="noopener noreferrer"
                class="text-sm text-muted-foreground hover:text-primary hover:underline transition-colors"
              >
                API
              </a>
            </li>
            <li>
              <a 
                href="https://community.example.com" 
                target="_blank"
                rel="noopener noreferrer"
                class="text-sm text-muted-foreground hover:text-primary hover:underline transition-colors"
              >
                Comunidad
              </a>
            </li>
            <li>
              <a 
                href="https://status.example.com" 
                target="_blank"
                rel="noopener noreferrer"
                class="text-sm text-muted-foreground hover:text-primary hover:underline transition-colors"
              >
                Estado del Sistema
              </a>
            </li>
          </ul>
        </div>

        <!-- Sección: Soporte -->
        <div class="space-y-4">
          <h4 class="text-base font-semibold text-foreground mb-3">
            Soporte
          </h4>
          <ul class="space-y-2.5">
            <li v-if="isLoggedIn">
              <button 
                class="text-sm text-muted-foreground hover:text-primary hover:underline transition-colors"
                @click="navigateTo('Self')"
              >
                Mi Cuenta
              </button>
            </li>
            <li v-if="hasAdminAccess">
              <button 
                class="text-sm text-muted-foreground hover:text-primary hover:underline transition-colors"
                @click="navigateTo('Admin.Settings')"
              >
                Configuración
              </button>
            </li>
            <li>
              <a 
                href="mailto:support@example.com"
                class="text-sm text-muted-foreground hover:text-primary hover:underline transition-colors"
              >
                Contacto
              </a>
            </li>
            <li>
              <a 
                href="https://help.example.com" 
                target="_blank"
                rel="noopener noreferrer"
                class="text-sm text-muted-foreground hover:text-primary hover:underline transition-colors"
              >
                Centro de Ayuda
              </a>
            </li>
          </ul>
        </div>
      </div>

      <!-- Divisor y Copyright -->
      <div class="pt-8 border-t-2 border-border/50">
        <div class="flex flex-col md:flex-row justify-between items-center gap-4 text-sm text-muted-foreground">
          <!-- Copyright -->
          <div class="text-center md:text-left">
            <p>
              © {{ currentYear }} <span class="font-semibold text-primary">Aether Panel</span>. Todos los derechos reservados.
            </p>
          </div>

          <!-- Enlaces legales -->
          <div class="flex items-center gap-1">
            <a 
              href="/privacy" 
              class="hover:text-primary hover:underline transition-colors px-2"
            >
              Privacidad
            </a>
            <span class="text-border">•</span>
            <a 
              href="/terms" 
              class="hover:text-primary hover:underline transition-colors px-2"
            >
              Términos
            </a>
            <span class="text-border">•</span>
            <a 
              href="/cookies" 
              class="hover:text-primary hover:underline transition-colors px-2"
            >
              Cookies
            </a>
          </div>

          <!-- Versión -->
          <div class="flex items-center gap-2">
            <span>Powered by</span>
            <span class="font-semibold text-primary">SkyPanel v1.0</span>
          </div>
        </div>
      </div>
    </div>
  </footer>
</template>

<style scoped>
footer {
  flex-shrink: 0;
}

button {
  background: none;
  border: none;
  padding: 0;
  cursor: pointer;
  text-align: left;
  width: 100%;
}
</style>

