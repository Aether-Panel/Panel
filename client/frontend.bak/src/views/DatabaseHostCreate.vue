<script setup>
import { ref, inject, computed, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import Btn from '@/components/ui/Btn.vue'

const api = inject('api')
const router = useRouter()
const route = useRoute()

// Detectar si estamos en modo edición
const isEditMode = computed(() => !!route.params.id)
const hostId = computed(() => route.params.id)

const currentStep = ref(1)
const formData = ref({
  username: '',
  password: '',
  host: '',
  port: 3306,
  name: '',
  max_databases: 0,
  node_id: null
})

const nodes = ref([])
const isLoading = ref(false)
const loadingHost = ref(false)
const originalUsername = ref('') // Guardar el username original para modo edición

// Cargar nodos y datos del host si estamos en modo edición
onMounted(async () => {
  try {
    const response = await api.node.list()
    nodes.value = response || []
  } catch (error) {
    console.error('Error loading nodes:', error)
  }

  // Si estamos en modo edición, cargar los datos del host
  if (isEditMode.value) {
    await loadHostData()
    // En modo edición, empezar directamente en el paso 3
    currentStep.value = 3
  }
})

// Cargar datos del host existente
async function loadHostData() {
  loadingHost.value = true
  try {
    const host = await api.databaseHost.get(hostId.value)
    originalUsername.value = host.username || '' // Guardar el username original
    formData.value = {
      username: host.username || '',
      password: '', // No cargamos la contraseña por seguridad
      host: host.host || '',
      port: host.port || 3306,
      name: host.name || '',
      max_databases: host.max_databases || 0,
      node_id: host.node_id || null
    }
  } catch (error) {
    console.error('Error loading database host:', error)
    alert('Error al cargar el Database Host: ' + (error.msg || error.message))
    router.push({ name: 'Admin.DatabaseHostList' })
  } finally {
    loadingHost.value = false
  }
}

// Generar usuario y contraseña aleatoria para el paso 2
const generatedUser = computed(() => {
  return 'aetheruser'
})

const generatedPassword = computed(() => {
  const chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789!@#$%^&*()_+-=[]{}|;:,.<>?'
  let password = ''
  for (let i = 0; i < 16; i++) {
    password += chars.charAt(Math.floor(Math.random() * chars.length))
  }
  return password
})

const step1Valid = computed(() => {
  return formData.value.username && formData.value.password
})

const step3Valid = computed(() => {
  return formData.value.host && formData.value.port && formData.value.name
})

function nextStep() {
  if (currentStep.value === 1 && !step1Valid.value) {
    alert('Por favor completa todos los campos requeridos')
    return
  }
  if (currentStep.value < 3) {
    currentStep.value++
  }
}

function prevStep() {
  if (currentStep.value > 1) {
    currentStep.value--
  }
}

function copyToClipboard(text) {
  navigator.clipboard.writeText(text).then(() => {
    alert('Copiado al portapapeles')
  }).catch(err => {
    console.error('Error al copiar:', err)
  })
}

async function saveHost() {
  if (!step3Valid.value) {
    alert('Por favor completa todos los campos requeridos')
    return
  }

  isLoading.value = true
  try {
    if (isEditMode.value) {
      // En modo edición, usar el username proporcionado o el original
      const updateData = {
        host: formData.value.host,
        port: formData.value.port,
        name: formData.value.name,
        username: formData.value.username || originalUsername.value, // Usar el nuevo o el original
        max_databases: formData.value.max_databases,
        node_id: formData.value.node_id
      }
      // Solo incluir password si se proporcionó uno nuevo
      if (formData.value.password) {
        updateData.password = formData.value.password
      }
      await api.databaseHost.update(hostId.value, updateData)
      alert('Database Host actualizado exitosamente')
    } else {
      await api.databaseHost.create(formData.value)
      alert('Database Host creado exitosamente')
    }
    router.push({ name: 'Admin.DatabaseHostList' })
  } catch (error) {
    console.error(`Error ${isEditMode.value ? 'updating' : 'creating'} database host:`, error)
    alert(`Error al ${isEditMode.value ? 'actualizar' : 'crear'} el Database Host: ` + (error.msg || error.message))
  } finally {
    isLoading.value = false
  }
}
</script>

<template>
  <div 
    :class="[
      'databasehost-create',
      'w-full max-w-4xl ml-auto mr-0',
      'space-y-6 p-6'
    ]"
  >
    <h1 
      :class="[
        'text-3xl font-bold text-foreground mb-6',
        'pb-3 border-b-2 border-border/50'
      ]"
    >
      {{ isEditMode ? 'Editar Database Host' : 'Crear Database Host' }}
    </h1>

    <loader v-if="loadingHost" />

    <template v-else>
    <!-- Progress Indicator (solo en modo creación) -->
    <div v-if="!isEditMode" :class="['flex items-center justify-between mb-8']">
      <div :class="['flex items-center', currentStep >= 1 ? 'text-primary' : 'text-muted-foreground']">
        <div :class="[
          'w-10 h-10 rounded-full flex items-center justify-center',
          'border-2',
          currentStep >= 1 ? 'border-primary bg-primary/20' : 'border-border'
        ]">
          1
        </div>
        <span :class="['ml-2 font-medium']">Credenciales</span>
      </div>
      <div :class="['flex-1 h-0.5 mx-4', currentStep >= 2 ? 'bg-primary' : 'bg-border']" />
      <div :class="['flex items-center', currentStep >= 2 ? 'text-primary' : 'text-muted-foreground']">
        <div :class="[
          'w-10 h-10 rounded-full flex items-center justify-center',
          'border-2',
          currentStep >= 2 ? 'border-primary bg-primary/20' : 'border-border'
        ]">
          2
        </div>
        <span :class="['ml-2 font-medium']">Instrucciones</span>
      </div>
      <div :class="['flex-1 h-0.5 mx-4', currentStep >= 3 ? 'bg-primary' : 'bg-border']" />
      <div :class="['flex items-center', currentStep >= 3 ? 'text-primary' : 'text-muted-foreground']">
        <div :class="[
          'w-10 h-10 rounded-full flex items-center justify-center',
          'border-2',
          currentStep >= 3 ? 'border-primary bg-primary/20' : 'border-border'
        ]">
          3
        </div>
        <span :class="['ml-2 font-medium']">Configuración</span>
      </div>
    </div>

    <!-- Step 1: Credentials (solo en modo creación) -->
    <div v-if="!isEditMode && currentStep === 1" :class="['space-y-6 bg-card p-6 rounded-xl border-2 border-border/30']">
      <div :class="['bg-info/10 border border-info/30 rounded-lg p-4 mb-4']">
        <p :class="['text-sm text-info font-medium']">
          ℹ️ Actualmente, solo se admiten bases de datos MySQL/MariaDB para database hosts!
        </p>
        <p :class="['text-xs text-muted-foreground mt-2']">
          ¿El panel y la base de datos no están en el mismo servidor? Asegúrate de permitir acceso externo.
        </p>
      </div>

      <input 
        v-model="formData.username"
        label="Username*"
        placeholder="root"
        :class="['w-full px-4 py-2 bg-background border-2 border-border rounded-lg focus:border-primary focus:outline-none']"
      />
      <p :class="['text-sm text-muted-foreground -mt-4']">
        El nombre de usuario de una cuenta que tenga permisos suficientes para crear nuevos usuarios y bases de datos en el sistema.
      </p>

      <input 
        v-model="formData.password"
        label="Password*"
        type="password"
        placeholder="••••••••"
        :class="['w-full px-4 py-2 bg-background border-2 border-border rounded-lg focus:border-primary focus:outline-none']"
      />
      <p :class="['text-sm text-muted-foreground -mt-4']">
        La contraseña para el usuario de la base de datos.
      </p>

      <div :class="['flex justify-end gap-3']">
        <btn @click="router.back()" color="secondary">
          Cancelar
        </btn>
        <btn @click="nextStep()" :disabled="!step1Valid">
          Siguiente
        </btn>
      </div>
    </div>

    <!-- Step 2: Instructions (solo en modo creación) -->
    <div v-if="!isEditMode && currentStep === 2" :class="['space-y-6 bg-card p-6 rounded-xl border-2 border-border/30']">
      <h3 :class="['text-xl font-bold text-foreground mb-4']">
        Database User
      </h3>

      <div :class="['bg-secondary/10 border border-border rounded-lg p-4']">
        <p :class="['text-sm text-muted-foreground mb-2']">
          Usa <code :class="['bg-background px-2 py-1 rounded']">mysql -u root -p</code> para acceder al CLI de mysql.
        </p>
      </div>

      <div :class="['space-y-4']">
        <div>
          <label :class="['block text-sm font-semibold text-foreground mb-2']">
            Comando para crear el usuario
          </label>
          <div :class="['relative']">
            <pre :class="[
              'bg-background border-2 border-border rounded-lg p-4',
              'text-sm font-mono text-foreground overflow-x-auto'
            ]">CREATE USER '{{ generatedUser }}'@'127.0.0.1' IDENTIFIED BY '{{ generatedPassword }}';</pre>
            <button
              @click="copyToClipboard(`CREATE USER '${generatedUser}'@'127.0.0.1' IDENTIFIED BY '${generatedPassword}';`)"
              :class="[
                'absolute top-2 right-2',
                'px-3 py-1 bg-primary/10 border border-primary/30 rounded',
                'text-primary text-xs font-medium',
                'hover:bg-primary/20 transition-all'
              ]"
            >
              Copiar
            </button>
          </div>
        </div>

        <div>
          <label :class="['block text-sm font-semibold text-foreground mb-2']">
            Comando para asignar permisos
          </label>
          <div :class="['relative']">
            <pre :class="[
              'bg-background border-2 border-border rounded-lg p-4',
              'text-sm font-mono text-foreground overflow-x-auto'
            ]">GRANT ALL PRIVILEGES ON *.* TO '{{ generatedUser }}'@'127.0.0.1' WITH GRANT OPTION;</pre>
            <button
              @click="copyToClipboard(`GRANT ALL PRIVILEGES ON *.* TO '${generatedUser}'@'127.0.0.1' WITH GRANT OPTION;`)"
              :class="[
                'absolute top-2 right-2',
                'px-3 py-1 bg-primary/10 border border-primary/30 rounded',
                'text-primary text-xs font-medium',
                'hover:bg-primary/20 transition-all'
              ]"
            >
              Copiar
            </button>
          </div>
        </div>

        <div :class="['bg-secondary/10 border border-border rounded-lg p-4']">
          <p :class="['text-sm text-muted-foreground']">
            Para salir del CLI de mysql ejecuta <code :class="['bg-background px-2 py-1 rounded']">exit</code>.
          </p>
        </div>
      </div>

      <div :class="['bg-warning/10 border border-warning/30 rounded-lg p-4 mt-6']">
        <h4 :class="['font-semibold text-foreground mb-2']">
          Acceso Externo
        </h4>
        <p :class="['text-sm text-muted-foreground mb-2']">
          Es probable que necesites permitir acceso externo a esta instancia de MySQL para que los servidores puedan conectarse.
        </p>
        <ol :class="['text-sm text-muted-foreground space-y-2 ml-4 list-decimal']">
          <li>
            Abre <code :class="['bg-background px-2 py-1 rounded']">my.cnf</code>, que varía su ubicación dependiendo de tu SO y cómo se instaló MySQL. Puedes escribir <code :class="['bg-background px-2 py-1 rounded']">find /etc -iname my.cnf</code> para localizarlo.
          </li>
          <li>
            Abre <code :class="['bg-background px-2 py-1 rounded']">my.cnf</code>, agrega el texto de abajo al final del archivo y guárdalo:
            <pre :class="['bg-background border border-border rounded p-2 mt-2 text-xs font-mono']">[mysqld]
bind-address=0.0.0.0</pre>
          </li>
          <li>
            Reinicia MySQL/MariaDB para aplicar estos cambios. Esto sobrescribirá la configuración predeterminada de MySQL, que por defecto solo aceptará solicitudes de localhost. Actualizar esto permitirá conexiones en todas las interfaces y, por lo tanto, conexiones externas. Asegúrate de permitir el puerto MySQL (por defecto 3306) en tu firewall.
          </li>
        </ol>
      </div>

      <div :class="['flex justify-between gap-3']">
        <btn @click="prevStep()" color="secondary">
          Anterior
        </btn>
        <btn @click="nextStep()">
          Siguiente
        </btn>
      </div>
    </div>

    <!-- Step 3: Final Configuration -->
    <div v-if="currentStep === 3" :class="['space-y-6 bg-card p-6 rounded-xl border-2 border-border/30']">
      <!-- Username y Password (solo en modo edición o en paso 3 de creación) -->
      <template v-if="isEditMode">
        <div :class="['bg-info/10 border border-info/30 rounded-lg p-4 mb-4']">
          <p :class="['text-sm text-info font-medium']">
            ℹ️ Deja los campos de usuario y contraseña en blanco si no deseas actualizarlos.
          </p>
        </div>
        
        <input 
          v-model="formData.username"
          label="Username"
          placeholder="Dejar en blanco para no actualizar"
          :class="['w-full px-4 py-2 bg-background border-2 border-border rounded-lg focus:border-primary focus:outline-none']"
        />
        <p :class="['text-sm text-muted-foreground -mt-4']">
          El nombre de usuario de una cuenta que tenga permisos suficientes para crear nuevos usuarios y bases de datos en el sistema. Deja en blanco para mantener el actual.
        </p>

        <input 
          v-model="formData.password"
          label="Password"
          type="password"
          placeholder="Dejar en blanco para no actualizar"
          :class="['w-full px-4 py-2 bg-background border-2 border-border rounded-lg focus:border-primary focus:outline-none']"
        />
        <p :class="['text-sm text-muted-foreground -mt-4']">
          La contraseña para el usuario de la base de datos. Deja en blanco para mantener la actual.
        </p>
      </template>

      <input 
        v-model="formData.host"
        label="Host*"
        placeholder="192.168.1.100"
        :class="['w-full px-4 py-2 bg-background border-2 border-border rounded-lg focus:border-primary focus:outline-none']"
      />
      <p :class="['text-sm text-muted-foreground -mt-4']">
        La dirección IP o nombre de dominio que debe usarse al intentar conectarse a este host MySQL desde este Panel para crear nuevas bases de datos.
      </p>

      <input 
        v-model.number="formData.port"
        label="Port*"
        type="number"
        placeholder="3306"
        :class="['w-full px-4 py-2 bg-background border-2 border-border rounded-lg focus:border-primary focus:outline-none']"
      />
      <p :class="['text-sm text-muted-foreground -mt-4']">
        El puerto en el que se está ejecutando MySQL para este host.
      </p>

      <input 
        v-model="formData.name"
        label="Display Name*"
        placeholder="Mi Database Host"
        :class="['w-full px-4 py-2 bg-background border-2 border-border rounded-lg focus:border-primary focus:outline-none']"
      />
      <p :class="['text-sm text-muted-foreground -mt-4']">
        La dirección IP o nombre de dominio que debe mostrarse al usuario final.
      </p>

      <input 
        v-model.number="formData.max_databases"
        label="Max Databases"
        type="number"
        placeholder="0 (ilimitado)"
        :class="['w-full px-4 py-2 bg-background border-2 border-border rounded-lg focus:border-primary focus:outline-none']"
      />
      <p :class="['text-sm text-muted-foreground -mt-4']">
        El número máximo de bases de datos que se pueden crear en este host. Si se alcanza el límite, no se podrán crear nuevas bases de datos en este host. Dejar en blanco es ilimitado.
      </p>

      <div>
        <label :class="['block text-sm font-semibold text-foreground mb-2']">
          Linked Nodes
        </label>
        <select
          v-model="formData.node_id"
          :class="['w-full px-4 py-2 bg-background border-2 border-border rounded-lg focus:border-primary focus:outline-none']"
        >
          <option :value="null">Ninguno</option>
          <option v-for="node in nodes" :key="node.id" :value="node.id">
            {{ node.name }}
          </option>
        </select>
        <p :class="['text-sm text-muted-foreground mt-2']">
          Esta configuración solo establece este database host como predeterminado al agregar una base de datos a un servidor en el nodo seleccionado.
        </p>
      </div>

      <div :class="['flex justify-between gap-3']">
        <btn v-if="!isEditMode" @click="prevStep()" color="secondary">
          Anterior
        </btn>
        <btn v-else @click="router.back()" color="secondary">
          Cancelar
        </btn>
        <btn @click="saveHost()" :disabled="!step3Valid || isLoading">
          {{ isLoading ? (isEditMode ? 'Actualizando...' : 'Creando...') : (isEditMode ? 'Actualizar' : 'Crear') }}
        </btn>
      </div>
    </div>
    </template>
  </div>
</template>
