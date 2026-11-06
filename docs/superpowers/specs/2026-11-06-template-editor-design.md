# Editor visual de plantillas

Fecha: 2026-11-06 · Estado: borrador para aprobar

## Por qué

Hoy una plantilla local se crea en un diálogo con tres pestañas:
- **General:** nombre, nombre visible y tipo.
- **Ejecución:** comando de arranque, comando de parada y carpeta de trabajo.
- **JSON.**

Todo lo demás solo se puede escribir a mano en JSON:
- las **variables** que rellena el usuario al crear un servidor;
- los **pasos de instalación** (unas 25 operaciones: `download`, `command`, `writefile`, `steamgamedl`, `mojangdl`, `extract`…, con condiciones `if`);
- el **entorno** (host o Docker, imagen, entornos admitidos);
- los **requisitos**;
- las **opciones de arranque**: stdin, código de parada, autoarranque y variables de entorno.

El usuario quiere que cualquiera pueda crear plantillas sin escribir JSON, sin quitárselo a quien sí quiera usarlo.

## Decisiones del usuario

- **Página completa con secciones**, en lugar del diálogo.
- **Todas las operaciones de instalación tienen formulario.** Si un paso es de un tipo desconocido, se edita su JSON solo para ese paso.
- **Ayudas:** «lo que sea mejor y más profesional». Decisión mía: las cuatro.
  1. Duplicar una plantilla existente, local o de la comunidad.
  2. Plantillas base: Minecraft Java, juego de Steam, app Node.js, app Python y Docker genérico.
  3. Validación en vivo.
  4. Vista previa del formulario «Crear servidor».
- **Buenos componentes**, buscados en la red; ver «Componentes».

## Objetivos

1. Una página `/templates/editor/` para plantillas nuevas y `/templates/editor/?name=<nombre>` para editar una local. Tiene:
   - un **menú lateral de secciones** con contadores y avisos (Información, Variables, Instalación, Ejecución, Entorno, Requisitos, Avanzado);
   - un selector **Formulario | JSON | Dividido** (el modo dividido solo en pantallas anchas);
   - los botones **Guardar**, **Descargar .json** y **Vista previa**.
2. **El formulario y el JSON son dos vistas del mismo objeto.**
   - Lo que se cambia en una aparece en la otra al momento.
   - Un JSON a medio escribir no rompe el formulario: este se queda con el último objeto válido y marca el JSON como inválido.
3. **No se pierde nada (ida y vuelta sin pérdidas).** Al abrir una plantilla y guardarla sin tocar nada, sale el mismo JSON.
   - Los campos que el formulario no conoce se conservan tal cual.
   - El orden de las claves puede cambiar.
   - Esto vale para las 60 plantillas de la comunidad.
4. **Variables:** lista ordenable arrastrando, con estas columnas:
   - clave (con validación: `^[A-Za-z0-9_-]+$`, sin repetidas; el guion se admite porque 4 plantillas de la comunidad lo usan, pero se avisa de que esas claves no sirven en condiciones);
   - nombre visible y descripción;
   - tipo: texto, número, sí/no u opción;
   - valor por defecto;
   - obligatoria, editable por el usuario e interna;
   - opciones (para el tipo «opción»).

   Además:
   - Un aviso marca cada variable usada en un comando o paso (`${clave}`) que no está definida, y cada variable definida que nadie usa.
   - Al escribir comandos, un botón inserta `${variable}`.
5. **Instalación:** lista de pasos ordenable.
   - «Añadir paso» abre un buscador con las operaciones agrupadas por categoría: Archivos, Descargas, Juegos/Minecraft, Steam, Java/Node y Sistema.
   - Cada tipo tiene su formulario, generado a partir de un esquema por operación, con los campos, la ayuda y el tipo de control.
   - Cada paso tiene una **condición opcional `if`**, con atajos para «solo en host» y «solo en Docker» y edición libre.
   - Un paso de tipo desconocido muestra «Editar JSON de este paso».
   - Los mismos controles sirven para `uninstall` y para los `pre`/`post` de Ejecución.
6. **Ejecución:**
   - comando o comandos (uno o varios, con condición);
   - parada por comando o por código de señal;
   - carpeta de trabajo;
   - variables de entorno (editor clave-valor);
   - stdin (tipo y opciones);
   - autoarranque, recuperación tras caída y reinicio tras parada.
7. **Entorno:**
   - entorno por defecto: host o Docker, con su imagen;
   - entornos admitidos (lista);
   - aviso si se usa una imagen de terceros: el proyecto solo quiere `ghcr.io/aether-panel/*` o imágenes oficiales.
8. **Requisitos:** sistema operativo, arquitectura y binarios.
   **Avanzado:** `stats`, `query`, `keepAlive` y `groups`, con formulario donde sea sencillo y JSON por sección en el resto.
9. **Validación en vivo**, con el mismo esquema en el formulario y en el JSON:
   - errores por campo;
   - resumen por sección en el menú lateral;
   - en el modo JSON, Monaco valida contra el esquema JSON generado (subraya errores y autocompleta claves y tipos de operación);
   - Guardar se desactiva si hay errores bloqueantes, como un nombre vacío o inválido o un JSON roto;
   - los avisos no bloquean.

   **Al guardar**, el servidor sigue validando como hoy. Si rechaza la plantilla, su error se muestra.
10. **Vista previa:** un panel lateral (Sheet) que renderiza el formulario «Crear servidor» con las variables de la plantilla, usando los mismos componentes que el asistente de creación real.
11. **Empezar:** al pulsar «Nueva plantilla» se ofrece:
    - en blanco;
    - una plantilla base;
    - duplicar una existente (buscador sobre las locales y las de la comunidad);
    - importar un .json.

    En la lista, cada plantilla tiene la acción «Duplicar», y las locales también «Editar».
12. **Responsive:**
    - en móvil, el menú lateral pasa a ser un selector de sección arriba y no hay modo dividido;
    - las listas se reordenan también con botones ↑ ↓, además de arrastrar;
    - sin desplazamiento horizontal a 360 px.
13. **Accesibilidad:** cada control tiene su etiqueta, se puede reordenar con el teclado (dnd-kit lo trae) y los errores se anuncian.
14. **Textos** en `es.json` y `en.json`. **Documentación** en es y en: `features/templates` o la página que exista.

## No objetivos

- No cambia el formato de las plantillas, ni el backend, salvo lo mínimo si algo hiciera falta.
- No se editan directamente las plantillas de la comunidad. Se duplican a local.
- No hay plantillas compartidas en tiempo real ni historial de versiones.

## Anti-objetivos

- Perder campos al guardar: el formulario siempre trabaja sobre el objeto completo.
- Que el formulario impida usar JSON: el modo JSON es igual de completo que hoy.
- Una página lenta: Monaco y la vista previa se cargan solo cuando se abren.

## Componentes (investigados)

- **Ordenar arrastrando:** `@dnd-kit/core` + `@dnd-kit/sortable` + `@dnd-kit/utilities`, la opción por defecto recomendada para listas ordenables en React, accesible por teclado. Hay que comprobar la compatibilidad de dependencias con React 19 al instalar; si da problemas, `@dnd-kit/react`.
- **Buscador o selector de operaciones e iconos:** `cmdk` con el patrón Command/Combobox de shadcn, más `@radix-ui/react-popover`.
- **Formularios:** `react-hook-form` + `zod` (ya están en el proyecto), con un esquema zod por operación y para la plantilla.
- **Esquema para Monaco:** se genera con `z.toJSONSchema()` (zod v4, ya instalado) y se pasa a `monaco.languages.json.jsonDefaults.setDiagnosticsOptions` para validar y autocompletar.
- **Editor JSON:** Monaco, ya presente (`@monaco-editor/react` y `monaco-setup.ts`).
- **UI base:** los componentes shadcn/Radix del proyecto: Tabs, Select, Switch, Sheet, Dialog, Tooltip y ScrollArea.

## Fuente de los esquemas de operaciones

- Se escribe un registro en el frontend (`client/frontend/src/features/templates/schema/`): por cada operación, su clave, categoría, nombre, ayuda, esquema zod de sus argumentos e iconos.
- Los argumentos se sacan del código Go de cada operación (`internal/operations/*`).
- Un test en Go (o un script de CI) comprueba que **todas** las operaciones registradas en el backend tienen entrada en el registro del frontend, para que una operación nueva no se quede sin formulario.

## Lo que queda al constructor

- La estructura exacta de los componentes y archivos dentro de `features/templates/`.
- Los textos de ayuda de cada campo de operación, sacados del código Go y de las plantillas de la comunidad.
- El detalle de las plantillas base, que deben ser válidas y usar solo imágenes `ghcr.io/aether-panel/*` u oficiales.
