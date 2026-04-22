# Mi Servidor — Go (Migrado desde Node.js/Express)

## Stack tecnológico

| Componente | Node.js (original) | Go (migrado) |
|---|---|---|
| Servidor | Express.js | `net/http` + `gorilla/mux` |
| Base de datos | `mssql` | `go-mssqldb` |
| Autenticación | `jsonwebtoken` | `golang-jwt/jwt` |
| Templates | EJS | `html/template` |
| Uploads | Multer | `mime/multipart` stdlib |
| Imágenes cloud | cloudinary v2 | `cloudinary-go` |

---

## Estructura del proyecto

```
mi-servidor-go/
├── cmd/
│   └── server/
│       └── main.go          ← Punto de entrada, router
├── internal/
│   ├── db/
│   │   └── db.go            ← Conexión SQL Server
│   ├── middleware/
│   │   └── auth.go          ← JWT + permisos
│   ├── handlers/
│   │   ├── auth.go          ← Login, dashboard, logout
│   │   ├── crud.go          ← CRUD APIs (perfiles, módulos, permisos, usuarios)
│   │   └── images.go        ← Cloudinary upload/list
│   └── services/
│       └── dashboard.go     ← Query de menús y datos del usuario
├── templates/
│   ├── login.html
│   ├── dashboard.html       ← Dashboard completo + JS SPA
│   ├── error.html
│   └── seguridad/
│       ├── perfil.html
│       ├── modulo.html
│       ├── permisos-perfil.html
│       └── usuario.html
├── static/
│   └── uploads/users/       ← Imágenes de usuarios
├── .env.example
├── go.mod
└── render.yaml
```

---

## Despliegue en Render

### 1. Prerrequisitos

- Cuenta en [render.com](https://render.com)
- Repositorio en GitHub o GitLab
- Go 1.22+ instalado localmente para pruebas

### 2. Preparar el proyecto localmente

```bash
# Clonar / copiar el proyecto
cd mi-servidor-go

# Copiar variables de entorno
cp .env.example .env
# Editar .env con tus valores reales

# Instalar dependencias
go mod tidy

# Compilar y probar localmente
go run ./cmd/server
```

Si quieres cargar el `.env` automáticamente en desarrollo, instala `godotenv`:
```bash
go get github.com/joho/godotenv
```
Y agrega al inicio de `main()`:
```go
import "github.com/joho/godotenv"
// ...
godotenv.Load()
```

### 3. Subir a GitHub

```bash
git init
git add .
git commit -m "feat: migración a Go"
git remote add origin https://github.com/tu-usuario/mi-servidor-go.git
git push -u origin main
```

### 4. Crear servicio en Render

1. Ve a [render.com](https://render.com) → **New** → **Web Service**
2. Conecta tu repositorio de GitHub
3. Render detectará automáticamente `render.yaml`, o configura manualmente:
   - **Runtime:** `Go`
   - **Build Command:** `go build -o server ./cmd/server`
   - **Start Command:** `./server`
4. En la sección **Environment Variables** agrega todas las variables de `.env.example` con tus valores reales:

| Variable | Descripción |
|---|---|
| `JWT_SECRET` | Clave secreta para firmar tokens JWT |
| `DB_USER` | Usuario de SQL Server |
| `DB_PASSWORD` | Contraseña de SQL Server |
| `DB_SERVER` | Host del servidor SQL Server |
| `DB_NAME` | Nombre de la base de datos |
| `CLOUDINARY_CLOUD_NAME` | Cloud name de tu cuenta Cloudinary |
| `CLOUDINARY_API_KEY` | API Key de Cloudinary |
| `CLOUDINARY_API_SECRET` | API Secret de Cloudinary |
| `RECAPTCHA_SECRET` | Secret key de Google reCAPTCHA |
| `RECAPTCHA_SITE` | Site key de Google reCAPTCHA |

5. Haz clic en **Create Web Service**

Render detecta Go automáticamente y despliega el binario. El puerto se configura vía la variable `PORT` que Render inyecta.

### 5. Notas de producción

- **Uploads de usuarios:** en Render el filesystem es efímero. Si necesitas persistencia de imágenes locales, usa Cloudinary también para los uploads de usuarios (ajusta `CreateUsuario` y `UpdateUsuario` en `handlers/crud.go`).
- **Templates:** los templates `.html` se leen en tiempo de ejecución. Asegúrate que el directorio `templates/` esté incluido en el repositorio.
- **Variables de entorno:** nunca subas el `.env` a git (ya está en `.gitignore`).

---

## Cambios y equivalencias clave

### Middleware de autenticación
El middleware `RequireAuth` + `LoadPermissions` es equivalente al chain `requireAuth` + `loadPermissions` de Node.js. Se aplica vía `gorilla/mux` subrouters.

### Rutas API
Todas las rutas `/api/*` son idénticas a las del servidor original. El frontend JavaScript del dashboard no necesita cambios de endpoints.

### Templates
Los templates `.html` usan la sintaxis `html/template` de Go. Las funciones personalizadas `toJSON` y `safeHTML` están registradas en el `FuncMap`.

### Archivos estáticos / uploads
Los archivos subidos se sirven desde `static/uploads/` con `http.FileServer`. La ruta pública es `/uploads/users/archivo.jpg`.

---

## Comandos útiles

```bash
# Compilar
go build -o server ./cmd/server

# Ejecutar con live reload (requiere air)
go install github.com/cosmtrek/air@latest
air

# Tests
go test ./...

# Ver módulos
go mod graph
```
