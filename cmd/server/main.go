package main

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"

	dbpkg "github.com/23301427-jpg/Diego_go2.0/internal/db"
	"github.com/23301427-jpg/Diego_go2.0/internal/handlers"
	mw "github.com/23301427-jpg/Diego_go2.0/internal/middleware"
	"github.com/gorilla/mux"
)

func main() {
	dbpkg.Connect()

	tmpl := loadTemplates()

	uploadDir := filepath.Join("static", "uploads", "users")
	os.MkdirAll(uploadDir, 0755)

	r := mux.NewRouter()

	// Static files
	r.PathPrefix("/uploads/").Handler(
		http.StripPrefix("/uploads/", http.FileServer(http.Dir("static/uploads/"))),
	)

	// Public
	r.HandleFunc("/", handlers.GetLogin(tmpl)).Methods("GET")
	r.HandleFunc("/login", handlers.PostLogin(tmpl)).Methods("POST")
	r.HandleFunc("/logout", handlers.GetLogout).Methods("GET")
	r.HandleFunc("/images", handlers.GetImages).Methods("GET")
	r.HandleFunc("/upload-image", handlers.UploadImage).Methods("POST")

	// Authenticated
	auth := r.NewRoute().Subrouter()
	auth.Use(mw.RequireAuth)
	auth.Use(mw.LoadPermissions)

	auth.HandleFunc("/dashboard", handlers.GetDashboard(tmpl)).Methods("GET")

	// Static modules
	staticMods := []struct{ path, menu, key, name string }{
		{"/principal-1-1", "Principal 1", "principal_1_1", "Principal 1.1"},
		{"/principal-1-2", "Principal 1", "principal_1_2", "Principal 1.2"},
		{"/principal-2-1", "Principal 2", "principal_2_1", "Principal 2.1"},
		{"/principal-2-2", "Principal 2", "principal_2_2", "Principal 2.2"},
		{"/galeria", "Galería", "galeria", "Galería"},
	}
	for _, sm := range staticMods {
		path, menu, key, name := sm.path, sm.menu, sm.key, sm.name
		auth.Handle(path, mw.RequireModuleAccess(key, "consulta")(
			http.HandlerFunc(handlers.GetStaticModule(tmpl, menu, key, name)),
		)).Methods("GET")
	}

	// Seguridad pages
	segMods := []struct{ path, view, menu, key, name string }{
		{"/seguridad/perfil", "perfil", "Seguridad", "perfil", "Perfil"},
		{"/seguridad/modulo", "modulo", "Seguridad", "modulo", "Módulo"},
		{"/seguridad/permisos-perfil", "permisos-perfil", "Seguridad", "permisos_perfil", "Permisos Perfil"},
		{"/seguridad/usuario", "usuario", "Seguridad", "usuario", "Usuario"},
	}
	for _, sm := range segMods {
		path, view, menu, key, name := sm.path, sm.view, sm.menu, sm.key, sm.name
		auth.Handle(path, mw.RequireModuleAccess(key, "consulta")(
			http.HandlerFunc(handlers.GetSeguridadView(tmpl, view, menu, key, name)),
		)).Methods("GET")
	}

	// API Catalogos
	auth.HandleFunc("/api/catalogos/base", handlers.GetCatalogosBase).Methods("GET")
	auth.HandleFunc("/api/static/{moduleKey}", handlers.GetStaticPermissions).Methods("GET")
	auth.HandleFunc("/api/me/permissions", handlers.GetMyPermissions).Methods("GET")

	// API Perfiles
	auth.Handle("/api/perfiles", wrapAccess("perfil", "consulta", handlers.GetPerfiles)).Methods("GET")
	auth.Handle("/api/perfiles/{id}", wrapAccess("perfil", "detalle", handlers.GetPerfilByID)).Methods("GET")
	auth.Handle("/api/perfiles", wrapAccess("perfil", "agregar", handlers.CreatePerfil)).Methods("POST")
	auth.Handle("/api/perfiles/{id}", wrapAccess("perfil", "editar", handlers.UpdatePerfil)).Methods("PUT")
	auth.Handle("/api/perfiles/{id}", wrapAccess("perfil", "eliminar", handlers.DeletePerfil)).Methods("DELETE")

	// API Modulos
	auth.Handle("/api/modulos", wrapAccess("modulo", "consulta", handlers.GetModulos)).Methods("GET")
	auth.Handle("/api/modulos/{id}", wrapAccess("modulo", "detalle", handlers.GetModuloByID)).Methods("GET")
	auth.Handle("/api/modulos", wrapAccess("modulo", "agregar", handlers.CreateModulo)).Methods("POST")
	auth.Handle("/api/modulos/{id}", wrapAccess("modulo", "editar", handlers.UpdateModulo)).Methods("PUT")
	auth.Handle("/api/modulos/{id}", wrapAccess("modulo", "eliminar", handlers.DeleteModulo)).Methods("DELETE")

	// API Permisos Perfil — solo requiere autenticacion; permisos se controlan en la pagina
	auth.HandleFunc("/api/permisos-perfil", handlers.GetPermisosPerfil).Methods("GET")
	auth.HandleFunc("/api/permisos-perfil", handlers.UpsertPermisosPerfil).Methods("POST")
	auth.HandleFunc("/api/permisos-perfil/{id}", handlers.UpsertPermisosPerfil).Methods("PUT")
	auth.HandleFunc("/api/permisos-perfil/{id}", handlers.DeletePermisosPerfil).Methods("DELETE")

	// API Usuarios
	auth.Handle("/api/usuarios", wrapAccess("usuario", "consulta", handlers.GetUsuarios)).Methods("GET")
	auth.Handle("/api/usuarios/{id}", wrapAccess("usuario", "detalle", handlers.GetUsuarioByID)).Methods("GET")
	auth.Handle("/api/usuarios", wrapAccess("usuario", "agregar", handlers.CreateUsuario(uploadDir))).Methods("POST")
	auth.Handle("/api/usuarios/{id}", wrapAccess("usuario", "editar", handlers.UpdateUsuario(uploadDir))).Methods("PUT")
	auth.Handle("/api/usuarios/{id}", wrapAccess("usuario", "eliminar", handlers.DeleteUsuario)).Methods("DELETE")

	// 404
	r.NotFoundHandler = http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		handlers.RenderError(tmpl, w, 404, "La página solicitada no existe.")
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}
	log.Printf("Servidor corriendo en http://localhost:%s", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}

func wrapAccess(module, action string, h http.HandlerFunc) http.Handler {
	return mw.RequireModuleAccess(module, action)(h)
}

func loadTemplates() *template.Template {
	funcMap := template.FuncMap{
		"toJSON": func(v interface{}) template.JS {
			b, _ := json.Marshal(v)
			return template.JS(b)
		},
		"safeHTML": func(s string) template.HTML { return template.HTML(s) },
		"safeJS":   func(s string) template.JS { return template.JS(s) },
		"firstChar": func(s string) string {
			if len(s) > 0 {
				return string([]rune(s)[:1])
			}
			return "?"
		},
	}
	tmpl, err := template.New("").Funcs(funcMap).ParseGlob("templates/*.html")
	if err != nil {
		log.Fatalf("Error cargando templates: %v", err)
	}
	sub, err := tmpl.ParseGlob("templates/seguridad/*.html")
	if err == nil {
		tmpl = sub
	}
	return tmpl
}
