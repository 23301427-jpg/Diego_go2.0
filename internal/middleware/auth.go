package middleware

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	dbpkg "github.com/23301427-jpg/Diego_go2.0/internal/db"
	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const (
	UserKey        contextKey = "user"
	PermissionsKey contextKey = "permissions"
)

var jwtSecret = []byte(getEnvOr("JWT_SECRET", "mi_servidor_jwt_secret_2026"))

type UserClaims struct {
	IDUsuario     int    `json:"idUsuario"`
	IDPerfil      int    `json:"idPerfil"`
	Nombre        string `json:"nombre"`
	Administrador bool   `json:"administrador"`
	jwt.RegisteredClaims
}

type Permission struct {
	IDModulo int
	Nombre   string
	Ruta     string
	Agregar  bool
	Editar   bool
	Consulta bool
	Eliminar bool
	Detalle  bool
	Any      bool
}

func SignToken(idUsuario, idPerfil int, nombre string, admin bool) (string, error) {
	claims := UserClaims{
		IDUsuario:     idUsuario,
		IDPerfil:      idPerfil,
		Nombre:        nombre,
		Administrador: admin,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(8 * time.Hour)),
		},
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t.SignedString(jwtSecret)
}

func ParseToken(tokenStr string) (*UserClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &UserClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("método de firma inesperado")
		}
		return jwtSecret, nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(*UserClaims); ok && token.Valid {
		return claims, nil
	}
	return nil, fmt.Errorf("token inválido")
}

func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie("token")
		if err != nil {
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}
		claims, err := ParseToken(c.Value)
		if err != nil {
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}
		ctx := context.WithValue(r.Context(), UserKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func LoadPermissions(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := UserFromCtx(r)
		if user == nil {
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}
		perms, err := LoadPermsFromDB(user.IDPerfil)
		if err != nil {
			http.Error(w, "Error cargando permisos", http.StatusInternalServerError)
			return
		}
		ctx := context.WithValue(r.Context(), PermissionsKey, perms)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func LoadPermsFromDB(idPerfil int) (map[string]*Permission, error) {
	rows, err := dbpkg.DB.Query(`
		SELECT m.id, m.strNombreModulo, m.strClaveModulo, m.strRuta,
			ISNULL(pp.bitAgregar,0), ISNULL(pp.bitEditar,0),
			ISNULL(pp.bitConsulta,0), ISNULL(pp.bitEliminar,0), ISNULL(pp.bitDetalle,0)
		FROM Modulo m
		LEFT JOIN PermisosPerfil pp ON pp.idModulo=m.id AND pp.idPerfil=@p1
	`, sql.Named("p1", idPerfil))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	perms := make(map[string]*Permission)
	for rows.Next() {
		var p Permission
		var clave string
		if err := rows.Scan(&p.IDModulo, &p.Nombre, &clave, &p.Ruta, &p.Agregar, &p.Editar, &p.Consulta, &p.Eliminar, &p.Detalle); err != nil {
			return nil, err
		}
		p.Any = p.Agregar || p.Editar || p.Consulta || p.Eliminar || p.Detalle
		perms[clave] = &p
	}
	return perms, nil
}

func RequireModuleAccess(moduleKey, action string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Administradores tienen acceso total sin restricción
			user := UserFromCtx(r)
			if user != nil && user.Administrador {
				next.ServeHTTP(w, r)
				return
			}
			perm := PermsFromCtx(r)[moduleKey]
			if perm == nil || !perm.Any {
				denyRequest(w, r)
				return
			}
			if action != "" && !permAction(perm, action) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "message": "No tienes permiso para esta acción"})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func permAction(p *Permission, action string) bool {
	switch action {
	case "agregar":
		return p.Agregar
	case "editar":
		return p.Editar
	case "consulta":
		return p.Consulta
	case "eliminar":
		return p.Eliminar
	case "detalle":
		return p.Detalle
	}
	return false
}

func denyRequest(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "message": "No tienes permiso"})
		return
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

func UserFromCtx(r *http.Request) *UserClaims {
	u, _ := r.Context().Value(UserKey).(*UserClaims)
	return u
}

func PermsFromCtx(r *http.Request) map[string]*Permission {
	p, _ := r.Context().Value(PermissionsKey).(map[string]*Permission)
	if p == nil {
		return map[string]*Permission{}
	}
	return p
}

func getEnvOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
