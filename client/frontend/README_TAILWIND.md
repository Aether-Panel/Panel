# Migración a Tailwind CSS

Este proyecto ahora usa Tailwind CSS para el diseño y los temas, reemplazando el sistema anterior basado en SCSS.

## Configuración

### Archivo principal de estilos
- `src/assets/main.css` - Contiene las directivas de Tailwind y las variables CSS para temas

### Configuración de Tailwind
- `tailwind.config.js` - Configuración de Tailwind con soporte para temas personalizados

## Sistema de Temas

Los temas ahora se basan en variables CSS que se actualizan dinámicamente:

### Modos disponibles:
- `mode-auto` - Detección automática del modo del sistema
- `mode-light` - Modo claro
- `mode-dark` - Modo oscuro
- `mode-dark-highcontrast` - Modo oscuro con alto contraste

### Variables CSS personalizables:
- `--color-primary` - Color principal (configurable desde settings)
- `--color-secondary` - Color secundario
- `--color-background` - Color de fondo
- `--color-foreground` - Color de texto
- Y muchas más...

## Uso de Tailwind

### Ejemplos de uso:

```vue
<template>
  <!-- Usar clases de Tailwind directamente -->
  <div class="bg-background text-foreground p-4 rounded-lg">
    <h1 class="text-2xl font-bold text-primary">Título</h1>
    <button class="bg-primary text-primary-foreground px-4 py-2 rounded hover:bg-primary/90">
      Botón
    </button>
  </div>

  <!-- Componentes con modo oscuro -->
  <div class="bg-sidebar text-sidebar-foreground dark:bg-sidebar-dark">
    Sidebar
  </div>
</template>
```

### Clases de utilidad personalizadas:

- `bg-primary` / `text-primary` - Color principal
- `bg-background` / `text-foreground` - Fondo y texto
- `bg-sidebar` / `text-sidebar-foreground` - Sidebar
- `bg-topbar` / `text-topbar-foreground` - Topbar
- `bg-error` / `text-error-foreground` - Errores
- `bg-success` / `text-success-foreground` - Éxito
- `bg-warning` / `text-warning-foreground` - Advertencias

## Migración gradual

El sistema antiguo de temas SCSS sigue funcionando, pero se recomienda migrar gradualmente a Tailwind:

1. **Mantener compatibilidad**: El sistema actual sigue cargando los temas SCSS antiguos
2. **Nuevos componentes**: Usa Tailwind CSS para todos los componentes nuevos
3. **Refactorizar componentes**: Migra componentes existentes cuando sea necesario

## Personalización de temas

Para crear un nuevo tema con Tailwind:

1. Actualiza las variables CSS en `src/assets/main.css`
2. Ajusta `tailwind.config.js` si necesitas nuevas utilidades
3. Usa las clases de Tailwind en tus componentes

## Ventajas de Tailwind CSS

- ✅ Diseño más rápido y fácil
- ✅ Temas personalizables sin escribir CSS personalizado
- ✅ Utilidades consistentes en todo el proyecto
- ✅ Mejor rendimiento (solo se incluyen las clases usadas)
- ✅ Mejor mantenimiento del código

