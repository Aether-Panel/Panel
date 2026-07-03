# Componentes e Interfaz Gráfica (`client/frontend`)

El paquete de `client/frontend` contiene la interfaz visual con la que los usuarios interactúan (el Dashboard, las páginas de login, la consola web, etc.). Utiliza un enfoque híbrido combinando Astro y React.

## Tecnologías de UI Core

### 1. Astro (Enrutamiento y SSR Estático)
*   Las rutas de la aplicación están determinadas por los archivos en la carpeta `src/pages`.
*   Astro permite que las páginas carguen sumamente rápido y actúa como contenedor, "hidratando" componentes React solo cuando es necesario (mediante la directiva `client:load` o `client:only`).

### 2. React + Tailwind CSS
*   Los componentes interactivos (formularios, consolas en vivo, modales) están construidos en **React 18**.
*   El estilizado es 100% utilitario usando **Tailwind CSS**. Esto permite tener componentes muy consistentes sin tener que crear múltiples archivos `.css`.

## Componentes UI Base (Shadcn UI / Radix)
En el directorio `src/components/`, la mayoría de las primitivas de interfaz están construidas con la filosofía de Shadcn UI (basado en Radix UI). 
Algunos de los componentes base incluyen:
*   **Dialog / AlertDialog**: Para confirmaciones (ej. "¿Estás seguro de que deseas eliminar este servidor?").
*   **Form / Label**: Utilizados en conjunto con `react-hook-form` y `zod` para validación de datos en tiempo real (ej. creación de usuarios).
*   **Select / DropdownMenu**: Componentes altamente accesibles para configuración.
*   **Tabs**: Para navegar por el detalle de un servidor (ej. Pestaña "Consola", Pestaña "Archivos", Pestaña "Backups").

## Features y Dominios (Carpeta `src/features/`)
Para mantener el código de React limpio y mantenible, las funciones están encapsuladas por dominio de negocio:
*   **Console / Terminal**: Componentes especializados que parsean la salida de la consola (que llega por WebSocket) y aplican colores ANSI.
*   **File Manager**: Una interfaz similar a un explorador de archivos nativo, que aprovecha las peticiones al SDK para descargar, subir y extraer `.zip`. Integrado con el editor web `@monaco-editor/react`.

## Estado Global
El estado global está manejado por contextos simples de React (`src/contexts/`). Por lo general:
*   **AuthContext / SessionContext**: Almacena si el usuario está autenticado, su token y sus scopes/roles.
*   **ServerContext**: Al entrar al detalle de un servidor de juego, este contexto guarda la información del Websocket en tiempo real para que todos los sub-componentes (gráficos, status badges) se actualicen mágicamente.
