# dev2.0-extransfer (LAN Transfers)

## ⚠️ Advertencia de Seguridad Importante

**ESTA RAMA NO ES APTA PARA PRODUCCIÓN.**

Esta rama especial (`dev2.0-extransfer`) contiene una modificación en el código del backend (`internal/utils/utils.go`) que **desactiva parcialmente la prevención de Server-Side Request Forgery (SSRF)**. 

Específicamente, se ha eliminado la comprobación `isPrivateIP(ip)` en la validación de URLs externas.

### ¿Para qué sirve esta rama?
Esta versión fue creada con el único propósito de permitir **Transferencias de Servidores (External Transfers) entre máquinas dentro de una misma red local (LAN)**, utilizando IPs privadas (ej. `192.168.x.x` o `10.x.x.x`). 

Es ideal para:
- Pruebas locales y entornos de desarrollo (Dev).
- Migraciones internas en un laboratorio doméstico o intranet aislada donde confías en todos los usuarios.

### ¿Por qué no es segura para producción (Hosting Comercial)?
Al permitir que el panel se conecte a direcciones IP privadas, se introduce una vulnerabilidad de **SSRF (Server-Side Request Forgery)**. 

Si este panel se expone al público de forma comercial:
1. Un cliente malintencionado que tenga permisos de Administrador en un servidor podría usar la herramienta de "Importar Servidor" (External Transfer).
2. Podría colocar como URL de origen la IP privada del router del datacenter, una base de datos interna, o dispositivos IoT en la misma red.
3. El servidor Backend de Aether Panel haría peticiones HTTP hacia esas IPs internas en nombre del atacante, permitiéndole escanear la red o extraer información de servicios que no deberían estar expuestos a Internet.

*(Nota: La validación de `loopback` y `localhost` sigue activa, por lo que el servidor no puede atacarse a sí mismo, pero la red LAN queda expuesta).*

---
**Conclusión:** Úsala bajo tu propio riesgo únicamente en entornos privados. Para producción pública, utiliza la rama `dev2.0` principal, la cual bloquea por defecto las conexiones a redes privadas.
