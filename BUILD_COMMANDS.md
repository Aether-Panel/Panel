# Comandos para compilar y generar dist

## Compilar el Frontend (Vue)

```bash
cd client/frontend
yarn install
yarn build
```

Esto generará los archivos compilados en `client/frontend/dist/`

## Compilar el Backend (Go)

```bash
# Desde la raíz del proyecto
go mod download
go mod verify
go build -tags "" -ldflags "-X 'github.com/SkyPanel/SkyPanel/v3.Hash=unknown' -X 'github.com/SkyPanel/SkyPanel/v3.Version=dev'" -o SkyPanel ./cmd
```

## Compilar todo (Frontend + Backend)

```bash
# 1. Compilar frontend
cd client/frontend
yarn install
yarn build

# 2. Copiar dist al directorio www del backend
cd ../..
cp -r client/frontend/dist/* www/

# 3. Compilar backend
go mod download
go mod verify
go build -tags "" -ldflags "-X 'github.com/SkyPanel/SkyPanel/v3.Hash=unknown' -X 'github.com/SkyPanel/SkyPanel/v3.Version=dev'" -o SkyPanel ./cmd
```

## Compilar con Docker

```bash
docker build -t skypanel .
```

## Comandos de desarrollo

### Frontend en modo desarrollo
```bash
cd client/frontend
yarn dev
```

### Backend en modo desarrollo
```bash
go run ./cmd run
```

## Verificar instalación

```bash
# Verificar que el binario se compiló correctamente
./SkyPanel --version

# Verificar que los archivos del frontend están en www/
ls -la www/
```

## Notas

- Asegúrate de tener Node.js 22+ y Yarn instalados para el frontend
- Asegúrate de tener Go 1.21+ instalado para el backend
- El directorio `www/` debe contener los archivos compilados del frontend
- Después de compilar, reinicia el servicio de SkyPanel:
  ```bash
  sudo systemctl restart SkyPanel
  ```

