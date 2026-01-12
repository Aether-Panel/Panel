# 📚 Documentación de SkyPanel

Bienvenido a la documentación oficial de **SkyPanel**, el panel de gestión de servidores de juegos de código abierto más completo y moderno.

## 🚀 Inicio Rápido

¿Primera vez usando SkyPanel? Comienza aquí:

1. 📦 [**Instalación**](./01-installation.md) - Instala SkyPanel en tu servidor
2. ⚙️ [**Configuración**](./02-configuration.md) - Configura el panel según tus necesidades
3. 🎮 [**Crear tu Primer Servidor**](./03-creating-servers.md) - Guía paso a paso

---

## 📖 Tabla de Contenidos

### 🏁 Primeros Pasos

- [**01. Instalación**](./01-installation.md)
  - Requisitos del sistema
  - Instalación automática
  - Instalación manual
  - Instalación con Docker
  - Verificación y configuración inicial
  - Solución de problemas

- [**02. Configuración**](./02-configuration.md)
  - Archivo de configuración
  - Base de datos
  - Email y notificaciones
  - SFTP
  - Gatus (monitoreo)
  - Seguridad

- [**03. Creación de Servidores**](./03-creating-servers.md)
  - Crear servidor desde plantilla
  - Configuración de servidor
  - Gestión de archivos
  - Consola web
  - Backups

### 👥 Gestión

- [**04. Gestión de Usuarios**](./04-user-management.md)
  - Crear y eliminar usuarios
  - Roles y permisos
  - Autenticación 2FA
  - OAuth2 clients

- [**05. Gestión de Nodos**](./05-node-management.md)
  - Configurar nodos remotos
  - Balanceo de carga
  - Monitoreo de nodos

- [**06. Gestión de Servidores**](./06-server-management.md)
  - Operaciones de servidor
  - Recursos y límites
  - Variables de entorno
  - Plantillas personalizadas

### 🔧 Características Avanzadas

- [**07. Sistema de Plantillas**](./07-templates.md)
  - Plantillas predefinidas
  - Crear plantillas personalizadas
  - Operaciones disponibles
  - Variables y sustitución

- [**08. Monitoreo con Gatus**](./08-monitoring.md)
  - Configurar monitoreo
  - Health checks
  - Alertas
  - Dashboard de estado

- [**09. Integración Discord**](./09-discord-integration.md)
  - Configurar webhooks
  - Tipos de notificaciones
  - Bot de Discord

- [**10. Sistema de Backups**](./10-backups.md)
  - Backups automáticos
  - Restauración
  - Almacenamiento remoto

### 🔌 API y Desarrollo

- [**11. Referencia de API**](./11-api-reference.md)
  - Autenticación OAuth2
  - Endpoints de servidores
  - Endpoints de usuarios
  - Endpoints de nodos
  - WebSocket API

- [**12. Integración con Python**](./12-python-integration.md)
  - Ejecutar scripts Python
  - API REST con Python
  - gRPC
  - Ejemplos prácticos

- [**13. Desarrollo de Plugins**](./13-plugin-development.md)
  - Crear operaciones personalizadas
  - Extender funcionalidad
  - Mejores prácticas

### 🛠️ Administración

- [**14. Comandos CLI**](./14-cli-commands.md)
  - Gestión de usuarios
  - Gestión de base de datos
  - Mantenimiento
  - Debugging

- [**15. Seguridad**](./15-security.md)
  - Mejores prácticas
  - Firewall
  - SSL/TLS
  - Hardening

- [**16. Actualización**](./16-upgrading.md)
  - Actualizar SkyPanel
  - Migración de datos
  - Rollback

- [**17. Troubleshooting**](./17-troubleshooting.md)
  - Problemas comunes
  - Logs y debugging
  - Recuperación de desastres

### 🎮 Juegos Soportados

- [**18. Minecraft**](./games/18-minecraft.md)
  - Vanilla, Forge, Paper, Fabric
  - Plugins y mods
  - Configuración avanzada

- [**19. Terraria**](./games/19-terraria.md)
- [**20. Valheim**](./games/20-valheim.md)
- [**21. ARK: Survival Evolved**](./games/21-ark.md)
- [**22. Rust**](./games/22-rust.md)
- [**23. CS:GO**](./games/23-csgo.md)
- [**24. Otros Juegos**](./games/24-other-games.md)

### 📚 Recursos

- [**25. FAQ**](./25-faq.md)
- [**26. Glosario**](./26-glossary.md)
- [**27. Contribuir**](./27-contributing.md)
- [**28. Changelog**](./28-changelog.md)

---

## 🎯 Guías Rápidas

### Instalación Rápida

```bash
# Instalación con un solo comando
bash <(curl -s https://install.skypanel.com/install.sh)
```

### Crear Primer Usuario

```bash
sudo -u skypanel skypanel user add --username admin --email admin@example.com --admin
```

### Crear Servidor de Minecraft

1. Accede al panel: `http://TU_IP:8080`
2. Click en "Crear Servidor"
3. Selecciona plantilla "Minecraft Java Edition"
4. Configura nombre, puerto y recursos
5. Click en "Crear"

---

## 🔗 Enlaces Útiles

| Recurso | Enlace |
|---------|--------|
| 🌐 **Sitio Web** | [skypanel.com](https://skypanel.com) |
| 📖 **Documentación** | [docs.skypanel.com](https://docs.skypanel.com) |
| 💬 **Discord** | [discord.gg/skypanel](https://discord.gg/skypanel) |
| 🐛 **Issues** | [github.com/SkyPanel/SkyPanel/issues](https://github.com/SkyPanel/SkyPanel/issues) |
| 📦 **Releases** | [github.com/SkyPanel/SkyPanel/releases](https://github.com/SkyPanel/SkyPanel/releases) |
| 🎥 **YouTube** | [youtube.com/@skypanel](https://youtube.com/@skypanel) |

---

## 💡 Ejemplos de Uso

### Caso 1: Hosting de Minecraft para Amigos

```bash
# 1. Instalar SkyPanel
bash <(curl -s https://install.skypanel.com/install.sh)

# 2. Crear usuario
sudo -u skypanel skypanel user add --username admin --admin

# 3. Acceder al panel y crear servidor Minecraft
# 4. Compartir IP:PUERTO con amigos
```

### Caso 2: Proveedor de Hosting de Juegos

- ✅ Gestión multi-servidor
- ✅ Sistema de permisos por usuario
- ✅ Monitoreo automático con Gatus
- ✅ Backups programados
- ✅ API para integración con sistema de facturación

### Caso 3: Comunidad Gaming

- ✅ Múltiples servidores (Minecraft, Terraria, Valheim)
- ✅ Roles de administrador/moderador
- ✅ Notificaciones Discord
- ✅ Consola web para gestión remota

---

## 🆘 Obtener Ayuda

### Antes de Preguntar

1. 🔍 Busca en la [documentación](./README.md)
2. 📖 Revisa las [FAQ](./25-faq.md)
3. 🐛 Busca en [issues existentes](https://github.com/SkyPanel/SkyPanel/issues)

### Canales de Soporte

- 💬 **Discord**: Para preguntas rápidas y ayuda de la comunidad
- 🐛 **GitHub Issues**: Para reportar bugs o solicitar features
- 📧 **Email**: support@skypanel.com (soporte empresarial)

### Reportar un Bug

Incluye la siguiente información:

```
**Versión de SkyPanel**: 3.0.0
**Sistema Operativo**: Ubuntu 22.04
**Método de Instalación**: Script automático / Manual / Docker

**Descripción del Problema**:
[Describe el problema aquí]

**Pasos para Reproducir**:
1. [Paso 1]
2. [Paso 2]
3. [Paso 3]

**Comportamiento Esperado**:
[Qué esperabas que sucediera]

**Logs**:
```
[Pega logs relevantes aquí]
```
```

---

## 🤝 Contribuir

¿Quieres contribuir a SkyPanel? ¡Genial!

1. 🍴 Fork el repositorio
2. 🌿 Crea una rama para tu feature (`git checkout -b feature/AmazingFeature`)
3. 💾 Commit tus cambios (`git commit -m 'Add some AmazingFeature'`)
4. 📤 Push a la rama (`git push origin feature/AmazingFeature`)
5. 🔃 Abre un Pull Request

Lee la [Guía de Contribución](./27-contributing.md) para más detalles.

---

## 📄 Licencia

SkyPanel está licenciado bajo la [Apache License 2.0](../LICENSE).

---

## 🌟 Agradecimientos

SkyPanel es posible gracias a:

- El equipo de desarrollo de SkyPanel
- Todos los [contribuidores](https://github.com/SkyPanel/SkyPanel/graphs/contributors)
- La comunidad de código abierto
- Proyectos como Go, Vue.js, Next.js, Gatus, y muchos más

---

## 📊 Estado del Proyecto

![GitHub release](https://img.shields.io/github/v/release/SkyPanel/SkyPanel)
![GitHub stars](https://img.shields.io/github/stars/SkyPanel/SkyPanel)
![GitHub issues](https://img.shields.io/github/issues/SkyPanel/SkyPanel)
![GitHub license](https://img.shields.io/github/license/SkyPanel/SkyPanel)
![Discord](https://img.shields.io/discord/123456789)

---

**¡Feliz hosting de servidores! 🎮🚀**
