package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	dbpkg "github.com/23301427-jpg/Diego_go2.0/internal/db"
	mw "github.com/23301427-jpg/Diego_go2.0/internal/middleware"
	"github.com/23301427-jpg/Diego_go2.0/internal/services"
)

var recaptchaSecret = getEnvOr("RECAPTCHA_SECRET", "6Lf-68QsAAAAAOCpEeLlDefj4X7d8nbNHUIpMg3R")
var recaptchaSite = getEnvOr("RECAPTCHA_SITE", "6Lf-68QsAAAAAHt7_MxjjVVbEO8SPFgi-Xk6QmVw")

func GetLogin(tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		errMsg := r.URL.Query().Get("error")
		tmpl.ExecuteTemplate(w, "login.html", map[string]interface{}{
			"Error":   errMsg,
			"SiteKey": recaptchaSite,
		})
	}
}

func PostLogin(tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Redirect(w, r, "/?error=Error al procesar formulario", http.StatusFound)
			return
		}
		usuario := r.FormValue("usuario")
		password := r.FormValue("password")
		captcha := r.FormValue("g-recaptcha-response")

		if captcha == "" {
			http.Redirect(w, r, "/?error=Verifica el reCAPTCHA", http.StatusFound)
			return
		}
		if !verifyCaptcha(captcha) {
			http.Redirect(w, r, "/?error=Captcha inválido", http.StatusFound)
			return
		}

		row := dbpkg.DB.QueryRow(`
			SELECT TOP 1 u.id, u.strNombreUsuario, u.strPwd, u.idPerfil, u.idEstadoUsuario,
				u.strCorreo, p.bitAdministrador, eu.strNombreEstado
			FROM Usuario u
			INNER JOIN Perfil p ON p.id = u.idPerfil
			INNER JOIN EstadoUsuario eu ON eu.id = u.idEstadoUsuario
			WHERE u.strNombreUsuario = @p1 OR u.strCorreo = @p1
		`, sql.Named("p1", usuario))

		var (
			idUsuario, idPerfil, idEstado              int
			strNombre, strPwd, strCorreo, nombreEstado string
			admin                                      bool
		)
		if err := row.Scan(&idUsuario, &strNombre, &strPwd, &idPerfil, &idEstado, &strCorreo, &admin, &nombreEstado); err != nil {
			http.Redirect(w, r, "/?error=Usuario o contraseña incorrectos", http.StatusFound)
			return
		}
		if strPwd != password {
			http.Redirect(w, r, "/?error=Usuario o contraseña incorrectos", http.StatusFound)
			return
		}
		if idEstado != 1 {
			http.Redirect(w, r, "/?error=El usuario no existe o está inactivo", http.StatusFound)
			return
		}

		token, err := mw.SignToken(idUsuario, idPerfil, strNombre, admin)
		if err != nil {
			http.Redirect(w, r, "/?error=Error interno al iniciar sesión", http.StatusFound)
			return
		}
		http.SetCookie(w, &http.Cookie{
			Name:     "token",
			Value:    token,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			Path:     "/",
		})
		http.Redirect(w, r, "/dashboard", http.StatusFound)
	}
}

func GetDashboard(tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := mw.UserFromCtx(r)
		perms := mw.PermsFromCtx(r)
		payload, err := services.GetDashboardPayloadWithAdmin(user.IDUsuario, user.IDPerfil, perms, user.Administrador)
		if err != nil {
			http.Error(w, "Error cargando dashboard: "+err.Error(), http.StatusInternalServerError)
			return
		}
		tmpl.ExecuteTemplate(w, "dashboard.html", map[string]interface{}{
			"User":          payload.User,
			"Menus":         payload.Menus,
			"Permissions":   permsToMapAdmin(perms, user.Administrador),
			"InitialModule": nil,
			"IsAdmin":       user.Administrador,
		})
	}
}

func GetStaticModule(tmpl *template.Template, menuName, moduleKey, moduleName string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := mw.UserFromCtx(r)
		perms := mw.PermsFromCtx(r)
		payload, err := services.GetDashboardPayloadWithAdmin(user.IDUsuario, user.IDPerfil, perms, user.Administrador)
		if err != nil {
			http.Error(w, "Error cargando módulo: "+err.Error(), http.StatusInternalServerError)
			return
		}
		tmpl.ExecuteTemplate(w, "dashboard.html", map[string]interface{}{
			"User":        payload.User,
			"Menus":       payload.Menus,
			"Permissions": permsToMapAdmin(perms, user.Administrador),
			"IsAdmin":     user.Administrador,
			"InitialModule": map[string]string{
				"menuName":   menuName,
				"moduleKey":  moduleKey,
				"moduleName": moduleName,
			},
		})
	}
}

func GetSeguridadView(tmpl *template.Template, viewName, menuName, moduleKey, moduleName string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := mw.UserFromCtx(r)
		perms := mw.PermsFromCtx(r)
		payload, err := services.GetDashboardPayloadWithAdmin(user.IDUsuario, user.IDPerfil, perms, user.Administrador)
		if err != nil {
			http.Error(w, "Error cargando vista: "+err.Error(), http.StatusInternalServerError)
			return
		}
		tmpl.ExecuteTemplate(w, "dashboard.html", map[string]interface{}{
			"User":        payload.User,
			"Menus":       payload.Menus,
			"Permissions": permsToMapAdmin(perms, user.Administrador),
			"IsAdmin":     user.Administrador,
			"InitialModule": map[string]string{
				"menuName":   menuName,
				"moduleKey":  moduleKey,
				"moduleName": moduleName,
			},
		})
	}
}

func GetLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:   "token",
		Value:  "",
		MaxAge: -1,
		Path:   "/",
	})
	http.Redirect(w, r, "/", http.StatusFound)
}

// ─── Helpers ──────────────────────────────────────────────────────────────

func verifyCaptcha(response string) bool {
	resp, err := http.PostForm("https://www.google.com/recaptcha/api/siteverify", url.Values{
		"secret":   {recaptchaSecret},
		"response": {response},
	})
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Success bool `json:"success"`
	}
	json.Unmarshal(body, &result)
	return result.Success
}

func permsToMap(perms map[string]*mw.Permission) map[string]interface{} {
	return permsToMapAdmin(perms, false)
}

func permsToMapAdmin(perms map[string]*mw.Permission, isAdmin bool) map[string]interface{} {
	out := make(map[string]interface{})
	for k, p := range perms {
		if isAdmin {
			// Admin tiene todos los permisos habilitados
			out[k] = map[string]interface{}{
				"idModulo": p.IDModulo,
				"nombre":   p.Nombre,
				"ruta":     p.Ruta,
				"agregar":  true,
				"editar":   true,
				"consulta": true,
				"eliminar": true,
				"detalle":  true,
				"any":      true,
			}
		} else {
			out[k] = map[string]interface{}{
				"idModulo": p.IDModulo,
				"nombre":   p.Nombre,
				"ruta":     p.Ruta,
				"agregar":  p.Agregar,
				"editar":   p.Editar,
				"consulta": p.Consulta,
				"eliminar": p.Eliminar,
				"detalle":  p.Detalle,
				"any":      p.Any,
			}
		}
	}
	return out
}

func getEnvOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// RenderError renders error page
func RenderError(tmpl *template.Template, w http.ResponseWriter, code int, message string) {
	w.WriteHeader(code)
	tmpl.ExecuteTemplate(w, "error.html", map[string]interface{}{
		"Code":    code,
		"Message": message,
	})
}

func IsAPIPath(r *http.Request) bool {
	return strings.HasPrefix(r.URL.Path, "/api/")
}

func WriteJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func WriteOK(w http.ResponseWriter) {
	WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func WriteErr(w http.ResponseWriter, status int, msg string) {
	WriteJSON(w, status, map[string]interface{}{"ok": false, "message": msg})
}

// PermsJSON converts perms for JSON API responses
func PermsJSON(perms map[string]*mw.Permission) map[string]interface{} {
	return permsToMap(perms)
}

// SafeNull reads sql.NullString as string
func NullStr(n sql.NullString) string {
	if n.Valid {
		return n.String
	}
	return ""
}

func IntPtr(s string) int {
	var v int
	fmt.Sscanf(s, "%d", &v)
	return v
}
