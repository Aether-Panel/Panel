# 🚀 Inicio Rápido - Aether Panel en Docker con Sudo

> **Nota**: Aether Panel es el nombre oficial del proyecto. **SkyPanel** es el nombre en clave (codename) utilizado en contenedores Docker e imágenes. Estamos en la **versión 3** del proyecto.

## ⚠️ Problema Detectado

Tu usuario necesita **sudo** para ejecutar Docker. Hay dos soluciones:

---

## ✅ Solución 1: Agregar Usuario al Grupo Docker (Recomendado)

Esto te permitirá usar Docker sin sudo:

```bash
# 1. Agregar tu usuario al grupo docker
sudo usermod -aG docker $USER

# 2. Aplicar cambios (elige una opción)
# Opción A: Reiniciar sesión (cerrar y volver a entrar)
# Opción B: Ejecutar esto (temporal)
newgrp docker

# 3. Verificar que funciona
docker ps

# 4. Ejecutar quickstart
./quickstart-docker.sh
```

---

## ✅ Solución 2: Usar con Sudo (Rápido)

Si no quieres cambiar permisos, ejecuta con sudo:

```bash
# El script detectará automáticamente que necesitas sudo
# y te preguntará si quieres continuar
./quickstart-docker.sh
```

Cuando veas:
```
⚠  Docker requiere sudo
   Para usar sin sudo, ejecuta:
   sudo usermod -aG docker esteban
   newgrp docker

¿Continuar con sudo? (y/N):
```

Presiona **`y`** y Enter para continuar.

---

## 🎯 Comandos Rápidos

### Con Permisos de Docker (después de Solución 1)
```bash
./quickstart-docker.sh          # Inicio automático
./docker-test.sh build          # Construir imagen
./docker-test.sh start          # Iniciar contenedor
./docker-test.sh admin          # Crear usuario admin
```

### Con Sudo (Solución 2)
```bash
# Los scripts detectarán automáticamente que necesitas sudo
./quickstart-docker.sh          # Te preguntará si continuar con sudo

# O ejecuta comandos directamente con sudo
sudo docker build -t skypanel:latest .
sudo docker-compose -f docker-compose.dev.yml up -d
```

---

## 📋 Verificar Estado de Docker

```bash
# Ver si Docker está corriendo
sudo systemctl status docker

# Iniciar Docker si no está corriendo
sudo systemctl start docker

# Habilitar Docker al inicio
sudo systemctl enable docker

# Verificar que puedes usar Docker
docker ps                    # Sin sudo (después de Solución 1)
sudo docker ps              # Con sudo (Solución 2)
```

---

## 🎮 Después de Iniciar

Una vez que el contenedor esté corriendo:

1. **Crear usuario admin**:
   ```bash
   ./docker-test.sh admin
   # O con sudo:
   sudo docker exec -it skypanel-dev /SkyPanel/bin/SkyPanel user add \
     --email admin@example.com \
     --password tu-contraseña \
     --admin
   ```

2. **Acceder al panel**:
   - Panel Web: http://localhost:8080
   - Gatus: http://localhost:8081
   - SFTP: localhost:5657

3. **Ver logs**:
   ```bash
   ./docker-test.sh logs
   # O:
   sudo docker logs -f skypanel-dev
   ```

---

## 🐛 Solución de Problemas

### Docker no está corriendo
```bash
sudo systemctl start docker
sudo systemctl status docker
```

### No puedo ejecutar docker sin sudo
```bash
# Agregar usuario al grupo
sudo usermod -aG docker $USER

# Aplicar cambios
newgrp docker

# O reinicia tu sesión
```

### El puerto 8080 está en uso
```bash
# Ver qué está usando el puerto
sudo netstat -tulpn | grep 8080

# O cambiar el puerto en docker-compose.dev.yml
# Edita la línea: "8080:8080" por "9000:8080"
```

---

## 💡 Recomendación

**Para desarrollo**: Usa la **Solución 1** (agregar usuario al grupo docker)
- Más cómodo
- No necesitas sudo cada vez
- Es la forma estándar

**Para prueba rápida**: Usa la **Solución 2** (con sudo)
- Más rápido
- No cambia permisos del sistema
- Bueno para pruebas temporales

---

## 🚀 Siguiente Paso

Ejecuta uno de estos comandos según tu elección:

```bash
# Solución 1 (recomendado)
sudo usermod -aG docker $USER && newgrp docker
./quickstart-docker.sh

# Solución 2 (rápido)
./quickstart-docker.sh
# (presiona 'y' cuando pregunte)
```

¡Listo! 🎉
