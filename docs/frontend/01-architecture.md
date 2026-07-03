# Arquitectura del Frontend (Client)

Este documento detalla la estructura, las tecnologías y la arquitectura general de la interfaz de usuario (frontend) de Aether Panel.

## Visión General del Espacio de Trabajo (Monorepo)

El directorio `client/` utiliza un enfoque de **Monorepo** basado en Yarn Workspaces. Esta estructura divide el código del cliente en dos paquetes principales que se intercomunican:

1.  **`api`**: Un envoltorio (Wrapper/SDK) encargado exclusivamente de la comunicación HTTP con el backend en Go.
2.  **`frontend`**: La aplicación web real encargada de la interfaz gráfica y la experiencia de usuario.

---

## 1. Paquete `api` (SDK del Cliente)

El directorio `client/api/` contiene una librería construida con Vite y empaquetada como módulos ESM y CJS. Su propósito es abstraer las llamadas HTTP hacia la API RESTful de Aether Panel.

### Características y Tecnologías:
*   **Axios**: Cliente HTTP utilizado para gestionar las peticiones, intercepciones y manejo de errores estandarizado.
*   **js-cookie**: Para la lectura y escritura de cookies (probablemente relacionado a la gestión de sesiones y tokens).
*   **Estructura**:
    *   Cada archivo en `src/` representa un dominio de la API del panel: `servers.js`, `users.js`, `nodes.js`, `auth.js`, `templates.js`, etc.
    *   Exporta todas sus funciones centralizadas desde `index.js`.

---

## 2. Paquete `frontend` (Aplicación Web)

La interfaz de usuario principal que interactúa con el usuario, ubicada en `client/frontend/`.

### Stack Tecnológico:
*   **Astro (`astro`)**: Framework web utilizado por su arquitectura de "Islas" (Islands) que envía el mínimo JavaScript necesario, mejorando los tiempos de carga y el SEO (si fuera necesario). Además, se encarga del enrutamiento estático basado en archivos (`src/pages`).
*   **React (`react`, `@astrojs/react`)**: Utilizado para construir los componentes interactivos de la aplicación.
*   **Tailwind CSS**: Framework de CSS utilitario, usado en conjunto con `tailwind-merge` y `clsx` para gestionar las clases CSS condicionales.
*   **Shadcn UI / Radix UI**: Colección de primitivas de UI sin estilo (`@radix-ui/react-*`) que, junto con Tailwind, construyen componentes accesibles y responsivos. Se usa para diálogos, modales, barras de progreso, menús, etc.
*   **Formularios**: Gestionados de forma robusta con `react-hook-form` y esquemas de validación tipados con `zod`.
*   **Mónaco Editor (`@monaco-editor/react`)**: Para la edición de código, configuración y plantillas directamente en el navegador.
*   **Genkit AI (`@genkit-ai/google-genai`)**: Integración con Inteligencia Artificial, que posiblemente asiste en la creación de servidores, sugerencias de configuración o análisis.
*   **Gráficos**: Uso de `recharts` para visualizar métricas (CPU, RAM) de los servidores de juego.

### Estructura de Directorios (`frontend/src/`)

El patrón de carpetas sigue las mejores prácticas modernas para aplicaciones React y Astro:

*   **`ai/`**: Lógica de integración, prompts y configuración para Genkit y el asistente de IA.
*   **`components/`**: Bloques de construcción visuales de la aplicación. Suele dividirse en componentes "UI" (Shadcn/Genéricos) y componentes específicos del panel.
*   **`contexts/`**: Estado global de la aplicación en React (por ejemplo: AuthContext, ThemeContext).
*   **`features/`**: Lógica agrupada por funcionalidad o dominio comercial (ej. `servers`, `billing`, `users`). Cada "feature" puede tener sus propios componentes, hooks y estado.
*   **`hooks/`**: Custom hooks de React para reusabilidad lógica (ej. `useServerStatus`, `useWebsocket`).
*   **`layouts/`**: Plantillas maestras de Astro/React que definen los esqueletos principales (Sidebar, Header, Layout base de dashboard).
*   **`lib/`**: Archivos de utilería (`utils.ts`), configuraciones globales e instanciación del cliente de API (`api/`).
*   **`pages/`**: Enrutador de Astro. Cada archivo `.astro` o `.tsx` aquí se convierte en una ruta web directa de la aplicación.
*   **`styles/`**: Variables CSS globales, estilos base y configuración de Tailwind.

---

## Flujo de Trabajo y Compilación

Al ser un workspace de Yarn, ejecutar `yarn build` desde la raíz del directorio `client/` desencadenará:
1.  La construcción del SDK en `client/api/dist/`.
2.  La generación del build optimizado de Astro en `client/frontend/dist/`.

Esto asegura que el frontend siempre consuma los esquemas de API más actualizados desarrollados internamente.
