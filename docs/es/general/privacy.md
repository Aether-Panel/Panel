# Política de Privacidad

## Introducción

Aether Panel es un panel open source publicado bajo licencia MIT. Todo el código fuente está disponible públicamente en GitHub para su revisión, auditoría y contribución. Esta política de privacidad explica cómo manejamos los datos en nuestra comunidad y en el sitio web.

Dado que Aether Panel se ejecuta en tu propio servidor (self-hosted), **nosotros no tenemos acceso a tu instalación ni a los datos que contiene**. Toda la información de tus servidores, usuarios y configuración permanece local en tu infraestructura. Esta política cubre principalmente tu interacción con nuestra comunidad (Discord, GitHub, página web).

Creemos en la transparencia total: como proyecto open source, cualquiera puede auditar el código para verificar que no hay recopilación oculta de datos, telemetría, ni puertas traseras.

## Nuestra Filosofía

Aether Panel fue creado con una filosofía diferente:

Esta filosofía nos diferencia de otros paneles que, siendo forks o proyectos similares, optan por modelos de negocio que comprometen la privacidad o la apertura del software.

### Open Source Real, Sin Trucos

El núcleo del panel está en GitHub bajo licencia MIT. Sin versiones enterprise, sin funciones premium ocultas, sin telemetría encubierta. Complementos para integraciones con software de código cerrado (WHMCS) pueden tener costo para cubrir licencias y mantenimiento, pero el panel en sí mismo es y será siempre gratuito.

### Privacidad por Diseño

El panel se ejecuta en tu servidor. Nosotros no recopilamos nada. No hay recolección de estadísticas de uso, no hay telemetría, no hay rastreadores. Lo que pasa en tu servidor, se queda en tu servidor.

### Comunidad, No Clientes

No eres un cliente porque no vendemos nada. Eres parte de una comunidad. Las decisiones se toman basadas en lo que la comunidad necesita, no en lo que genera ingresos.

### Transparencia Total

Al ser open source (MIT), cualquiera puede auditar el código. No hay binarios cerrados, no hay servicios en la nube obligatorios, no hay sorpresas. El panel principal es completamente transparente.


## Datos que Recopilamos

**En tu instalación del panel:** No recopilamos absolutamente nada. El panel no tiene telemetría, no reporta estadísticas de uso, no se conecta a servidores externos (excepto los que tú configures, como bases de datos MySQL).

**En nuestra comunidad (Discord, GitHub, página web):** Si decides interactuar con nosotros, recopilamos voluntariamente:

**Importante:** Si instalas el panel, tú eres el único responsable de los datos de tus usuarios. Nosotros no tenemos acceso, no recopilamos métricas, y no podemos ver tu instalación.

### Datos de Discord (Solo si te unes)

Al unirte a nuestro servidor de Discord:

Esto es estándar en cualquier servidor de Discord. No almacenamos esta información fuera de Discord.

- Nombre de usuario, avatar e ID de Discord (información pública de tu perfil)
- Mensajes que envías en los canales públicos
- Roles que te asignamos (miembro, colaborador, etc.)

### Datos de la Página Web (Sugerencias/Votaciones)

Si participas en sugerencias o votaciones en la página web:

No requerimos registro para la mayoría de las interacciones. Puedes participar de forma anónima.

- Nombre que proporcionas al enviar una sugerencia
- Votos en propuestas de características (almacenados en Firestore)
- País aproximado (determinado por IP, solo con fines estadísticos)

### Datos de GitHub (Solo si contribuyes)

Si contribuyes código al proyecto:

Toda esta información es pública por naturaleza en GitHub.

- Nombre de usuario de GitHub
- Código y mensajes de commit que envías
- Issues y pull requests que creas


## Cómo Usamos Tus Datos

Los datos limitados que recopilamos (solo de la comunidad) se usan exclusivamente para:

**Lo que NO hacemos (y jamás haremos):**

El panel principal es y será siempre completamente gratuito y open source. Los únicos complementos que podrían tener costo son integraciones con software de código cerrado de terceros (como WHMCS), cuyo desarrollo y mantenimiento requiere cubrir licencias e infraestructura. Estos se venderán al precio más bajo posible para solo cubrir costos.

### Mejora del Proyecto

Las sugerencias y reportes de errores nos ayudan a priorizar el desarrollo del fork.

### Comunicación Comunitaria

Discord es nuestro canal principal de comunicación. Tus interacciones allí nos ayudan a construir comunidad.

### Transparencia en Decisiones

Las votaciones nos permiten saber qué características priorizar. Los resultados son públicos.


- No vendemos ni compartimos datos personales con terceros
- No implementamos telemetría en el panel
- No recolectamos estadísticas de uso del panel
- No tenemos servicios cloud obligatorios
- No monetizamos datos de usuarios
- El panel principal es 100% gratuito y open source (MIT) — nunca pondremos funciones del panel tras un paywall

## Transparencia del Código Abierto

Al estar publicado bajo licencia MIT, el código fuente completo está disponible para auditoría pública:

- Repositorio público en GitHub con todo el historial de cambios
- Licencia MIT — puedes usar, modificar y distribuir sin restricciones
- Cualquier persona puede auditar el código para verificar que no hay recopilación de datos
- No hay binarios pre-compilados cerrados — todo se construye desde el código fuente
- No hay servicios externos obligatorios — el panel funciona 100% offline si lo deseas
- Las contribuciones de la comunidad son bienvenidas y transparentes

## Autogestión (Self-Hosted)

Aether Panel está diseñado para ser autogestionado. Esto significa que:

- Tú controlas dónde y cómo se ejecuta el panel
- Todos los datos (usuarios, servidores, configuraciones) están en tu infraestructura
- No hay conexiones salientes obligatorias — el panel funciona sin acceso a internet
- Eres responsable de la seguridad y privacidad de los datos en tu instalación
- Puedes auditar el tráfico de red del panel para verificar que no hay comunicación no deseada

## Almacenamiento de Datos

Dónde y cómo se almacenan los datos según el contexto:

### Instalación del Panel (Tu Servidor)

Base de datos local (SQLite, MySQL o PostgreSQL según tu configuración):

- Datos de usuarios (username, email, contraseña hasheada con bcrypt)
- Configuración del panel (config.json en el servidor)
- Archivos de servidores de juegos
- Backups, logs y bases de datos de servidores
- Tokens de sesión y OAuth2 (almacenados como hashes SHA256)

### Comunidad (Firebase/Firestore)

Datos que proporcionas voluntariamente en la página web:

- Sugerencias y votos (Firestore)
- Testimonios (si decides enviar uno)
- Mensajes de contacto (enviados a Discord via webhook)

### Retención de Datos

Política de retención:

- Datos del panel: los conservas hasta que decidas eliminarlos
- Cuentas de Discord: mientras seas miembro del servidor
- Sugerencias y votos: se mantienen como referencia del proyecto
- Código contribuido: permanente (como parte del repositorio open source)
- Puedes solicitar la eliminación de tus datos de nuestra comunidad en cualquier momento

## Tus Derechos

Respetamos tus derechos sobre tus datos personales:

Puedes solicitar una copia de los datos que tenemos sobre ti en nuestra comunidad.

- **right**: Derecho de Acceso
Puedes solicitar que eliminemos tus datos de nuestra comunidad en cualquier momento.

- **right**: Derecho de Eliminación
Puedes exportar tus datos de tu instalación del panel directamente desde la base de datos.

- **right**: Derecho de Portabilidad

### Cómo Ejercer Tus Derechos

Para solicitudes relacionadas con datos en nuestra comunidad:

Para datos en tu instalación local del panel, tienes control total directo. Nosotros no podemos acceder a tu instalación.

- Contáctanos a través de Discord
- Especifica qué datos deseas revisar o eliminar
- Procesaremos tu solicitud en un plazo razonable

## Servicios Externos

El proyecto utiliza servicios externos mínimos, todos con políticas de privacidad independientes:

No compartimos datos con terceros para publicidad, marketing, ni ningún propósito comercial.

Nuestro servidor comunitario. Sujeto a la política de privacidad de Discord. Solo accedemos a información pública del perfil.

- **name**: Discord
Código fuente, issues y pull requests. Todo es público por naturaleza. Sujeto a la política de privacidad de GitHub.

- **name**: GitHub
Usamos Firestore para almacenar sugerencias, votos y testimonios de la página web. No almacenamos datos sensibles. Sujeto a la política de Google.

- **name**: Firebase (Google)
Para donaciones voluntarias. No almacenamos ni procesamos información de pago — todo se maneja a través de PayPal.

- **name**: PayPal

## Menores de Edad

El proyecto no está dirigido a menores de 13 años. No recopilamos intencionalmente información de menores.

## Cambios a Esta Política

Si actualizamos esta política:

- Publicaremos un anuncio en nuestro Discord
- Actualizaremos la fecha en esta página
- Los cambios sustanciales se notificarán en la comunidad

## Contacto

Para preguntas sobre privacidad o para ejercer tus derechos:

Al ser un proyecto open source sin fines de lucro, respondemos en la medida de lo posible. Tu privacidad es importante, y estamos comprometidos a protegerla.

- Discord: Únete a nuestro servidor y habla con el equipo
- GitHub: Abre un issue en el repositorio
- El proyecto es mantenido por voluntarios — no hay soporte comercial

## En Resumen

Aether Panel es un panel open source de gestión de servidores de videojuegos. Esto significa que:

No confíes en nosotros, confía en el código. Todo está en GitHub para que lo verifiques.

- El código del panel es 100% público y auditable (licencia MIT)
- El panel se ejecuta en TU servidor — nosotros no vemos nada
- No hay telemetría, rastreadores ni recolección de datos en el panel
- El panel principal es gratuito — sin funciones premium de pago
- Solo recopilamos datos si interactúas voluntariamente con nuestra comunidad
- Somos open source — construimos en abierto y contribuimos de vuelta a la comunidad
- Integraciones con software de código cerrado (WHMCS) pueden tener costo para cubrir licencias y mantenimiento, pero el panel base es y será siempre libre

## Módulos de Pago

El panel principal de Aether Panel es y será siempre 100% gratuito y open source (MIT). Sin embargo, pueden existir módulos de integración con software de terceros que tengan un costo asociado.

Actualmente, estamos desarrollando un módulo de integración con WHMCS (plataforma de facturación y automatización). WHMCS es un software de código cerrado que requiere licencia paga. El módulo WHMCS para Aether Panel tendrá un costo porque:

El módulo WHMCS es un complemento opcional. No necesitas comprarlo para usar el panel. El panel principal, todas sus funciones y el código fuente del panel permanecen 100% gratuitos y open source. Nunca pondremos funciones del panel tras un paywall.

- WHMCS es software propietario — requiere licencia paga para desarrollo y pruebas
- El módulo requiere mantenimiento continuo para mantener compatibilidad
- Cubrir costos de infraestructura para desarrollo y pruebas
- No tiene fines de lucro — se venderá al precio más bajo posible

