# Política de Privacidad

## Introducción

Aether Panel es un panel open source publicado bajo licencia Apache 2.0. Todo el código fuente está disponible públicamente en GitHub para su revisión, auditoría y contribución. Esta política de privacidad explica cómo manejamos los datos en nuestra comunidad y en el sitio web.

Dado que Aether Panel se ejecuta en tu propio servidor (self-hosted), **nosotros no tenemos acceso a tu instalación ni a los datos que contiene**. Toda la información de tus servidores, usuarios y configuración permanece local en tu infraestructura. Esta política cubre principalmente tu interacción con nuestra comunidad (Discord, GitHub, página web).

Creemos en la transparencia total: como proyecto open source, cualquiera puede auditar el código para verificar que no hay recopilación oculta de datos, telemetría, ni puertas traseras.

## Nuestra Filosofía

Aether Panel fue creado con una filosofía diferente:

Esta filosofía nos diferencia de otros paneles que, siendo forks o proyectos similares, optan por modelos de negocio que comprometen la privacidad o la apertura del software.

### Open Source Real, Sin Trucos

El núcleo del panel está en GitHub bajo licencia Apache 2.0. Sin versiones enterprise, sin funciones premium ocultas, sin telemetría encubierta. Complementos para integraciones con software de código cerrado (WHMCS) pueden tener costo para cubrir licencias y mantenimiento, pero el panel en sí mismo es y será siempre gratuito.

### Privacidad por Diseño

El panel se ejecuta en tu servidor. Nosotros no recopilamos nada. No hay recolección de estadísticas de uso, no hay telemetría, no hay rastreadores. Lo que pasa en tu servidor, se queda en tu servidor.

### Comunidad, No Clientes

No eres un cliente porque no vendemos nada. Eres parte de una comunidad. Las decisiones se toman basadas en lo que la comunidad necesita, no en lo que genera ingresos.

### Transparencia Total

Al ser open source (Apache 2.0), cualquiera puede auditar el código. No hay binarios cerrados, no hay servicios en la nube obligatorios, no hay sorpresas. El panel principal es completamente transparente.


## Datos que Recopilamos

**En tu instalación del panel:** No recopilamos absolutamente nada. El panel no tiene telemetría ni reporta estadísticas de uso. Sí contacta con algunos servicios externos: unos pocos siempre (los avatares de usuario y la comprobación de actualizaciones cuando un administrador abre el panel) y el resto solo cuando se configuran o se usan. La lista está en [Servicios externos que contacta el panel](#servicios-externos-que-contacta-el-panel).

**En nuestra comunidad (Discord, GitHub, página web):** Si decides interactuar con nosotros, recopilamos voluntariamente:

**Importante:** Si instalas el panel, tú eres el único responsable de los datos de tus usuarios. Nosotros no tenemos acceso, no recopilamos métricas, y no podemos ver tu instalación.

### Datos de Discord (Solo si te unes)

Al unirte a nuestro servidor de Discord:

- Nombre de usuario, avatar e ID de Discord (información pública de tu perfil)
- Mensajes que envías en los canales públicos
- Roles que te asignamos (miembro, colaborador, etc.)

Esto es estándar en cualquier servidor de Discord. No almacenamos esta información fuera de Discord.

### Datos de la Página Web (Sugerencias/Votaciones)

Si participas en sugerencias o votaciones en la página web:

- Nombre que proporcionas al enviar una sugerencia
- Votos en propuestas de características (almacenados en Firestore)
- País aproximado (determinado por IP, solo con fines estadísticos)

No requerimos registro para la mayoría de las interacciones. Puedes participar de forma anónima.

### Datos de GitHub (Solo si contribuyes)

Si contribuyes código al proyecto:

- Nombre de usuario de GitHub
- Código y mensajes de commit que envías
- Issues y pull requests que creas

Toda esta información es pública por naturaleza en GitHub.


## Cómo Usamos Tus Datos

Los datos limitados que recopilamos (solo de la comunidad) se usan exclusivamente para:

### Mejora del Proyecto

Las sugerencias y reportes de errores nos ayudan a priorizar el desarrollo del fork.

### Comunicación Comunitaria

Discord es nuestro canal principal de comunicación. Tus interacciones allí nos ayudan a construir comunidad.

### Transparencia en Decisiones

Las votaciones nos permiten saber qué características priorizar. Los resultados son públicos.

**Lo que NO hacemos (y jamás haremos):**

- No vendemos ni compartimos datos personales con terceros
- No implementamos telemetría en el panel
- No recolectamos estadísticas de uso del panel
- No tenemos servicios cloud obligatorios
- No monetizamos datos de usuarios
- El panel principal es 100% gratuito y open source (Apache 2.0) — nunca pondremos funciones del panel tras un paywall

El panel principal es y será siempre completamente gratuito y open source. Los únicos complementos que podrían tener costo son integraciones con software de código cerrado de terceros (como WHMCS), cuyo desarrollo y mantenimiento requiere cubrir licencias e infraestructura. Estos se venderán al precio más bajo posible para solo cubrir costos.

## Transparencia del Código Abierto

Al estar publicado bajo licencia Apache 2.0, el código fuente completo está disponible para auditoría pública:

- Repositorio público en GitHub con todo el historial de cambios
- Licencia Apache 2.0 — puedes usar, modificar y distribuir sin restricciones
- Cualquier persona puede auditar el código para verificar que no hay recopilación de datos
- No hay binarios pre-compilados cerrados — todo se construye desde el código fuente
- No hay servicios externos obligatorios — el panel arranca y funciona sin acceso a internet, pero instalar servidores de juego, descargar imágenes de contenedor y buscar plugins o modpacks lo necesitan (ver [Servicios externos que contacta el panel](#servicios-externos-que-contacta-el-panel))
- Las contribuciones de la comunidad son bienvenidas y transparentes

## Autogestión (Self-Hosted)

Aether Panel está diseñado para ser autogestionado. Esto significa que:

- Tú controlas dónde y cómo se ejecuta el panel
- Todos los datos (usuarios, servidores, configuraciones) están en tu infraestructura
- No hay conexiones salientes obligatorias — el panel funciona sin acceso a internet, salvo las funciones que descargan algo o hablan con un servicio configurado (ver [Servicios externos que contacta el panel](#servicios-externos-que-contacta-el-panel))
- Eres responsable de la seguridad y privacidad de los datos en tu instalación
- Puedes auditar el tráfico de red del panel para verificar que no hay comunicación no deseada

## Almacenamiento de Datos

Dónde y cómo se almacenan los datos según el contexto:

### Instalación del Panel (Tu Servidor)

Base de datos local (SQLite por defecto; MySQL/MariaDB, PostgreSQL o SQL Server según la configuración). Los datos personales que guarda:

- Cuentas: nombre de usuario, correo, contraseña cifrada con bcrypt, rol y permisos, y fechas de creación y modificación. Las cuentas invitadas a un servidor por correo reciben un nombre de usuario aleatorio hasta que la persona lo cambia.
- Autenticación en dos pasos: el secreto TOTP (guardado tal cual, porque el panel lo necesita para comprobar los códigos) y los códigos de recuperación (guardados como hashes).
- Claves SSH añadidas para SFTP: nombre, clave pública, tipo y huella.
- Sesiones: un hash SHA-256 del token, la cuenta y la caducidad. Los tokens para restablecer la contraseña también se guardan como hashes SHA-256 y duran 30 minutos. Los secretos de los clientes OAuth2 se guardan como hashes bcrypt y las claves API de facturación como hashes SHA-256.
- Registro de accesos: inicios de sesión, intentos fallidos, cierres de sesión, registros, restablecimientos de contraseña y enlaces de acceso, cada uno con el nombre de usuario, el correo, la dirección IP, el agente de usuario del navegador y la hora (ver [Registro de actividad y registros de acceso](../features/activity-and-access-log.md)).
- Actividad de los servidores: quién hizo qué en cada servidor (nombre de usuario, acción, un detalle breve y dirección IP).
- Preferencias de la interfaz de cada usuario (ajustes de usuario).

Fuera de la base de datos, en el disco del panel:

- Configuración del panel (`config.json`, `/etc/SkyPanel/config.json` en instalaciones Linux) y algunos ajustes en la tabla `panel_settings`
- Archivos de los servidores de juego, copias de seguridad, historial de archivos e informes de caídas (los informes guardan líneas de consola, que pueden incluir nombres de jugadores)
- Registros del panel: cada petición HTTP se anota con su hora, estado, dirección IP del cliente, método y ruta (los secretos de las rutas se enmascaran)

### Comunidad (Firebase/Firestore)

Datos que proporcionas voluntariamente en la página web:

- Sugerencias y votos (Firestore)
- Testimonios (si decides enviar uno)
- Mensajes de contacto (enviados a Discord via webhook)

### Retención de Datos

Política de retención:

- Datos del panel: se conservan hasta que se decida eliminarlos, salvo lo que el panel borra por su cuenta:
  - Las entradas del registro de accesos de más de 90 días se borran cada vez que se consulta ese registro.
  - Las entradas de actividad de los servidores de más de 90 días se borran (se comprueba como mucho una vez por hora, al anotar nueva actividad).
  - Los archivos rotados de los registros del panel se borran a los `logRetentionDays` días (14 por defecto; `0` los conserva todos).
  - Cada servidor guarda sus 10 informes de caída más recientes.
  - Las sesiones dejan de funcionar tras 1 hora sin actividad; se borran al cerrar sesión, al cambiar la contraseña y al borrar la cuenta. Las caducadas se eliminan de la base de datos como mucho una vez por hora, al crearse una sesión nueva.
  - Borrar una cuenta borra sus permisos, clientes OAuth2, sesiones, códigos de recuperación de 2FA, restablecimientos de contraseña pendientes, claves SSH y preferencias de la interfaz. Sus entradas del registro de accesos y de actividad de los servidores se quedan hasta que la limpieza de 90 días las quita.
- Cuentas de Discord: mientras seas miembro del servidor
- Sugerencias y votos: se mantienen como referencia del proyecto
- Código contribuido: permanente (como parte del repositorio open source)
- Puedes solicitar la eliminación de tus datos de nuestra comunidad en cualquier momento

### Cookies y almacenamiento del navegador

El panel pone estas cookies:

| Cookie | La pone | Qué guarda | Duración |
|--------|---------|------------|----------|
| `skypanel_auth` | El panel, al iniciar sesión | El token de la sesión. `HttpOnly`, `SameSite=Lax` y `Secure` siempre que se accede al panel por HTTPS. | 1 hora, renovada mientras se usa el panel |
| `session` | El panel, durante un inicio de sesión con autenticación en dos pasos | El correo de la cuenta y la hora, entre el paso de la contraseña y el del código. Firmada con la clave de sesión del panel. Sus opciones salen de `panel.web.cookies.*`. | `panel.web.cookies.age` (30 días por defecto) |
| `sidebar_state` | La interfaz web | Si la barra lateral está abierta o plegada | 7 días |

La interfaz web también guarda preferencias en el almacenamiento local y de sesión del navegador (idioma de la interfaz, la última pestaña abierta en cada servidor, avisos descartados, si ya se hizo la comprobación de actualizaciones) y, en el almacenamiento de sesión, la conversación con el asistente del servidor y un cambio pendiente de aprobación, por servidor. Ninguna de estas cookies ni de estos datos se usa para seguimiento ni publicidad.

## Servicios externos que contacta el panel

El panel no envía telemetría. Estos son los servicios externos con los que puede hablar, y cuándo. «Navegador» significa que la petición la hace el navegador del visitante, así que el servicio ve su dirección IP; «panel» o «nodo» significa que la hace el propio servidor.

| Servicio | Cuándo | Quién conecta | Qué se envía |
|----------|--------|---------------|--------------|
| Avatares de Kodama (`api.kodama.sh`) | **Siempre**, donde se muestra el avatar de una cuenta | Navegador | Una semilla obtenida de la cuenta con un hash de un solo sentido (nunca el correo ni el nombre de usuario) y opciones de dibujo. No hay ningún ajuste para desactivarlo; si el servicio no responde, se muestran las iniciales. |
| API de GitHub (`api.github.com`) | Cuando alguien con `settings.edit` abre el panel (una vez por sesión del navegador) y en la pantalla de actualizaciones | Panel | Consultas de los últimos commits del repositorio `Aether-Panel/Panel`. Ningún dato de cuentas. Ver [Actualizaciones del panel](../features/panel-updates.md). |
| Registros de contenedores (GitHub Container Registry, Docker Hub…) | Al aplicar una actualización del panel y al descargar la imagen de un servidor | Panel / nodo | Descargas de imágenes. |
| Descargas de juegos y entornos (Mojang, PaperMC, Fabric, Forge, NeoForge, Quilt, Spigot, Sponge, Magma, Mohist, Adoptium, Node.js, Steam, versiones publicadas en GitHub…) | Al instalar un servidor o cambiar su versión, y al mostrar listas de versiones, según su plantilla | Nodo | Peticiones de descarga. Ver [Versión del juego](../features/game-versions.md). |
| Repositorios de plantillas | Solo si hay repositorios (o `templates.url`) configurados | Panel | Peticiones de los archivos de plantilla. Ver [Plantillas](../features/templates.md). |
| Modrinth, CurseForge | Cuando alguien busca o instala plugins y modpacks | Nodo | Términos de búsqueda e identificadores de proyectos y versiones. Ver [Modpacks y plugins](../features/modpacks-and-plugins.md). |
| Google Gemini (`generativelanguage.googleapis.com`) | **Opcional**: solo con una clave de API de Gemini, y solo cuando alguien usa el asistente o un análisis con IA | Panel | Los mensajes del chat, las líneas de consola elegidas y el contexto del servidor descrito en [Asistente de IA](../features/ai-assistant.md). Las líneas de consola pueden incluir nombres de jugadores. Cuando el asistente puede trabajar con los archivos del servidor (`panel.settings.aiFileTools`, activado por defecto), también el contenido de los archivos que lee, los nombres de las carpetas que lista y las líneas que encuentran sus búsquedas, del servidor en el que se usa; los archivos de configuración pueden contener contraseñas o tokens. Véase [Asistente de IA: trabajar con archivos](../features/ai-agent.md#privacidad). |
| Cloudflare Turnstile (`challenges.cloudflare.com`) | **Opcional**: solo con Turnstile activado y sus claves | Navegador y panel | El navegador carga el widget de Cloudflare; el panel envía el token del desafío y la IP del visitante para verificarlo. La comprobación integrada que se usa por defecto no necesita ningún servicio externo. Ver [Captcha / anti-bots](./17-turnstile.md). |
| API de DNS de Cloudflare (`api.cloudflare.com`) | **Opcional**: solo con un token y una zona de Cloudflare para subdominios | Panel | Nombres y destinos de los registros DNS. Ver [Subdominios automáticos](../features/subdomains.md). |
| Webhooks de Discord | **Opcional**: solo si hay una URL de webhook configurada (para todo el panel o por servidor) | Panel / nodo | Avisos de eventos: nombre del servidor, el evento y, en los eventos del panel, detalles como el nodo o la transferencia. Ver [Notificaciones del servidor](../features/server-notifications.md) y [Discord](./16-discord.md). |
| API de bots de Telegram (`api.telegram.org`) y webhooks JSON propios | **Opcional**: solo si el propietario de un servidor los configura | Nodo | Los mismos avisos de eventos del servidor. |
| Proveedor de correo (servidor SMTP, Mailgun, Mailjet o SendGrid) | **Opcional**: solo si `panel.email.provider` está configurado | Panel | La dirección del destinatario y el contenido del correo (creación de cuenta, invitaciones, restablecimiento de contraseña…). Ver [Correo electrónico](./15-email.md). |
| Almacenamiento compatible con S3 (AWS S3, Cloudflare R2, MinIO…) | **Opcional**: solo con las copias externas activadas | Nodo | Copias de los archivos de backup. Ver [Copias de seguridad](../features/backups.md). |
| Webhooks de extensiones | **Opcional**: solo para extensiones que un administrador instala y activa | Panel | Los eventos a los que se suscribió cada extensión, firmados. Ver [Extensiones](../reference/15-extensions.md#eventos). |
| Otro Aether Panel | **Opcional**: solo durante una transferencia externa que inicia un administrador | Panel | Los archivos y la definición del servidor. Ver [Transferencias de servidores](../features/server-transfers.md). |
| Servidor de licencias (`prueba.skyhostingcloud.com`) | **Opcional**: solo cuando un administrador activa una clave de licencia | Panel | La clave de licencia, el nombre de host de la máquina y su dirección IP. Ver [Sistema de licencias](./14-license.md). |
| Hosts de bases de datos | **Opcional**: solo los que añade un administrador | Panel | Órdenes de gestión de bases de datos y usuarios. Ver [Hosts de bases de datos](../features/database-hosts.md). |

## Tus Derechos

Respetamos tus derechos sobre tus datos personales:

- **Derecho de Acceso**: Puedes solicitar una copia de los datos que tenemos sobre ti en nuestra comunidad.
- **Derecho de Eliminación**: Puedes solicitar que eliminemos tus datos de nuestra comunidad en cualquier momento.
- **Derecho de Portabilidad**: Puedes exportar tus datos de tu instalación del panel directamente desde la base de datos.

### Cómo Ejercer Tus Derechos

Para solicitudes relacionadas con datos en nuestra comunidad:

- Contáctanos a través de Discord
- Especifica qué datos deseas revisar o eliminar
- Procesaremos tu solicitud en un plazo razonable

Para datos en tu instalación local del panel, tienes control total directo. Nosotros no podemos acceder a tu instalación.

## Servicios Externos

El proyecto utiliza servicios externos mínimos, todos con políticas de privacidad independientes:

- **Discord**: Nuestro servidor comunitario. Sujeto a la política de privacidad de Discord. Solo accedemos a información pública del perfil.
- **GitHub**: Código fuente, issues y pull requests. Todo es público por naturaleza. Sujeto a la política de privacidad de GitHub.
- **Firebase (Google)**: Usamos Firestore para almacenar sugerencias, votos y testimonios de la página web. No almacenamos datos sensibles. Sujeto a la política de Google.
- **PayPal**: Para donaciones voluntarias. No almacenamos ni procesamos información de pago — todo se maneja a través de PayPal.

No compartimos datos con terceros para publicidad, marketing, ni ningún propósito comercial.

## Menores de Edad

El proyecto no está dirigido a menores de 13 años. No recopilamos intencionalmente información de menores.

## Cambios a Esta Política

Si actualizamos esta política:

- Publicaremos un anuncio en nuestro Discord
- Actualizaremos la fecha en esta página
- Los cambios sustanciales se notificarán en la comunidad

## Contacto

Para preguntas sobre privacidad o para ejercer tus derechos:

- Discord: Únete a nuestro servidor y habla con el equipo
- GitHub: Abre un issue en el repositorio
- El proyecto es mantenido por voluntarios — no hay soporte comercial

Al ser un proyecto open source sin fines de lucro, respondemos en la medida de lo posible. Tu privacidad es importante, y estamos comprometidos a protegerla.

## En Resumen

Aether Panel es un panel open source de gestión de servidores de videojuegos. Esto significa que:

- El código del panel es 100% público y auditable (licencia Apache 2.0)
- El panel se ejecuta en TU servidor — nosotros no vemos nada
- No hay telemetría, rastreadores ni recolección de datos en el panel
- El panel principal es gratuito — sin funciones premium de pago
- Solo recopilamos datos si interactúas voluntariamente con nuestra comunidad
- Somos open source — construimos en abierto y contribuimos de vuelta a la comunidad
- Integraciones con software de código cerrado (WHMCS) pueden tener costo para cubrir licencias y mantenimiento, pero el panel base es y será siempre libre

No confíes en nosotros, confía en el código. Todo está en GitHub para que lo verifiques.

## Módulos de Pago

El panel principal de Aether Panel es y será siempre 100% gratuito y open source (Apache 2.0). Sin embargo, pueden existir módulos de integración con software de terceros que tengan un costo asociado.

Actualmente, estamos desarrollando un módulo de integración con WHMCS (plataforma de facturación y automatización). WHMCS es un software de código cerrado que requiere licencia paga. El módulo WHMCS para Aether Panel tendrá un costo porque:

- WHMCS es software propietario — requiere licencia paga para desarrollo y pruebas
- El módulo requiere mantenimiento continuo para mantener compatibilidad
- Cubrir costos de infraestructura para desarrollo y pruebas
- No tiene fines de lucro — se venderá al precio más bajo posible

El módulo WHMCS es un complemento opcional. No necesitas comprarlo para usar el panel. El panel principal, todas sus funciones y el código fuente del panel permanecen 100% gratuitos y open source. Nunca pondremos funciones del panel tras un paywall.

