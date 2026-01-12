# 🛠️ Referencia de Comandos CLI y Gestión

SkyPanel incluye una potente herramienta de línea de comandos (CLI) que te permite gestionar el panel, crear usuarios, realizar tareas de mantenimiento y ejecutar el servidor.

Esta guía cubre los comandos esenciales para instalar, configurar y ejecutar SkyPanel.

---

## 🏗️ 1. Compilación e Instalación

Antes de poder usar los comandos CLI, necesitas compilar el binario del proyecto.

### Requisitos Previos
Asegúrate de estar en el directorio raíz del proyecto y tener Go instalado (versión 1.21+).

### Comando de Compilación
Para generar el ejecutable `./skypanel`, ejecuta el siguiente comando:

```bash
go build -o skypanel ./cmd
```

> **Nota**: Esto creará un archivo binario llamado `skypanel` en tu directorio actual. Si estás en Windows, se creará `skypanel.exe`.

---

## 👥 2. Crear Cuenta de Administrador

Una vez compilado el binario, el paso más importante es crear tu primera cuenta de usuario con privilegios de administrador. Esto es necesario para poder iniciar sesión en el panel web.

### Comando para Crear Admin

Usa el siguiente comando para crear un usuario administrador instantáneamente:

```bash
./skypanel user add --name "ADMIN" --email "admin@admin.com" --admin --password "admin1234"
```

### Desglose de Parámetros

| Parámetro | Descripción | Ejemplo |
|-----------|-------------|---------|
| `--name` | Nombre de usuario para iniciar sesión | `"ADMIN"` |
| `--email` | Dirección de correo electrónico del usuario | `"admin@admin.com"` |
| `--admin` | Bandera que otorga permisos de superusuario | *(sin valor)* |
| `--password` | Contraseña para la cuenta | `"admin1234"` |

> **⚠️ Importante**: Si no proporcionas la contraseña mediante la bandera `--password`, el sistema te la pedirá de forma interactiva por seguridad.

---

## 🚀 3. Inicialización y Ejecución

Para iniciar el servidor web y todos los servicios de SkyPanel (incluyendo SFTP y Gatus), utiliza el comando `run`.

### Iniciar el Panel

```bash
./skypanel run
```

Este comando:
1.  Iniciará el servidor web en el puerto configurado (por defecto `8080`).
2.  Iniciará los servicios internos.
3.  Mostrará logs en la consola en tiempo real.

> **Tip**: Para ejecutarlo en segundo plano o como servicio del sistema, consulta la [Guía de Instalación](./01-installation.md).

---

## 📚 4. Otros Comandos Útiles

Aquí tienes una referencia rápida de otros sub-comandos disponibles en el CLI.

### Gestión de Usuarios

**Listar usuarios:**
```bash
./skypanel user list
```

**Eliminar un usuario:**
```bash
./skypanel user delete --email "usuario@ejemplo.com"
```

**Cambiar contraseña:**
```bash
./skypanel user password --email "admin@admin.com" --password "NuevaClave123"
```

### Gestión de Base de Datos

**Migrar base de datos (Actualizar esquema):**
Útil cuando actualizas SkyPanel a una nueva versión.
```bash
./skypanel db migrate
```

### Versión

**Ver la versión actual:**
```bash
./skypanel version
```

---

## 🆘 Solución de Problemas Comunes

**Error: "permission denied" al ejecutar**
Si recibes un error de permisos, asegúrate de que el binario tiene permisos de ejecución:
```bash
chmod +x skypanel
```

**Error: "address already in use"**
Si al ejecutar `./skypanel run` ves este error, significa que el puerto 8080 ya está ocupado.
- Verifica si ya tienes una instancia de SkyPanel corriendo.
- O edita `config.json` para cambiar el puerto.

