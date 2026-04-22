package handlers

import (
	"database/sql"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	dbpkg "mi-servidor-go/internal/db"
	mw "mi-servidor-go/internal/middleware"
)

// ─── Helpers ──────────────────────────────────────────────────────────────

func parsePage(r *http.Request) (int, int, int) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	limit := 5
	offset := (page - 1) * limit
	return page, limit, offset
}

func boolBody(v string) bool {
	return v == "true" || v == "1" || v == "on"
}

func onlyDigits(v string) string {
	re := regexp.MustCompile(`\D`)
	return re.ReplaceAllString(v, "")
}

func isValidPhone(v string) bool {
	digits := onlyDigits(v)
	return len(digits) == 10
}

func intParam(r *http.Request, key string) int {
	v, _ := strconv.Atoi(mux.Vars(r)[key])
	return v
}

func parseBody(r *http.Request) map[string]string {
	r.ParseForm()
	out := make(map[string]string)
	for k, vs := range r.Form {
		if len(vs) > 0 {
			out[k] = vs[0]
		}
	}
	return out
}

// ─── Catálogos base ───────────────────────────────────────────────────────

func GetCatalogosBase(w http.ResponseWriter, r *http.Request) {
	type row struct {
		ID     int    `json:"id"`
		Nombre string `json:"nombre"`
	}
	query := func(q string) ([]row, error) {
		rows, err := dbpkg.DB.Query(q)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var result []row
		for rows.Next() {
			var r row
			rows.Scan(&r.ID, &r.Nombre)
			result = append(result, r)
		}
		if result == nil {
			result = []row{}
		}
		return result, nil
	}

	perfiles, err := query("SELECT id, strNombrePerfil FROM Perfil ORDER BY strNombrePerfil")
	if err != nil {
		WriteErr(w, 500, err.Error()); return
	}
	estados, err := query("SELECT id, strNombreEstado FROM EstadoUsuario ORDER BY id")
	if err != nil {
		WriteErr(w, 500, err.Error()); return
	}
	modulos, err := query("SELECT id, strNombreModulo FROM Modulo ORDER BY id")
	if err != nil {
		WriteErr(w, 500, err.Error()); return
	}
	menus, err := query("SELECT id, strNombreMenu FROM Menu ORDER BY intOrdenMenu, id")
	if err != nil {
		WriteErr(w, 500, err.Error()); return
	}
	WriteJSON(w, 200, map[string]interface{}{
		"ok": true, "perfiles": perfiles, "estados": estados,
		"modulos": modulos, "menus": menus,
	})
}

func GetStaticPermissions(w http.ResponseWriter, r *http.Request) {
	moduleKey := mux.Vars(r)["moduleKey"]
	perms := mw.PermsFromCtx(r)
	perm := perms[moduleKey]
	if perm == nil || !perm.Any {
		WriteErr(w, 403, "No tienes permiso"); return
	}
	WriteJSON(w, 200, map[string]interface{}{
		"ok": true,
		"actions": map[string]bool{
			"agregar": perm.Agregar, "editar": perm.Editar, "consulta": perm.Consulta,
			"eliminar": perm.Eliminar, "detalle": perm.Detalle,
		},
	})
}

// ─── Perfiles ─────────────────────────────────────────────────────────────

func GetPerfiles(w http.ResponseWriter, r *http.Request) {
	page, limit, offset := parsePage(r)
	search := "%" + r.URL.Query().Get("search") + "%"

	var total int
	dbpkg.DB.QueryRow("SELECT COUNT(*) FROM Perfil WHERE strNombrePerfil LIKE @p1", sql.Named("p1", search)).Scan(&total)

	rows, err := dbpkg.DB.Query(fmt.Sprintf(
		"SELECT id, strNombrePerfil, bitAdministrador FROM Perfil WHERE strNombrePerfil LIKE @p1 ORDER BY id DESC OFFSET %d ROWS FETCH NEXT %d ROWS ONLY",
		offset, limit,
	), sql.Named("p1", search))
	if err != nil {
		WriteErr(w, 500, err.Error()); return
	}
	defer rows.Close()

	type item struct {
		ID              int    `json:"id"`
		StrNombrePerfil string `json:"strNombrePerfil"`
		BitAdministrador bool  `json:"bitAdministrador"`
	}
	data := []item{}
	for rows.Next() {
		var it item
		rows.Scan(&it.ID, &it.StrNombrePerfil, &it.BitAdministrador)
		data = append(data, it)
	}
	totalPages := int(math.Ceil(float64(total) / float64(limit)))
	if totalPages < 1 {
		totalPages = 1
	}
	WriteJSON(w, 200, map[string]interface{}{"ok": true, "data": data, "page": page, "totalPages": totalPages})
}

func GetPerfilByID(w http.ResponseWriter, r *http.Request) {
	id := intParam(r, "id")
	var it struct {
		ID               int    `json:"id"`
		StrNombrePerfil  string `json:"strNombrePerfil"`
		BitAdministrador bool   `json:"bitAdministrador"`
	}
	err := dbpkg.DB.QueryRow("SELECT id, strNombrePerfil, bitAdministrador FROM Perfil WHERE id=@p1", sql.Named("p1", id)).
		Scan(&it.ID, &it.StrNombrePerfil, &it.BitAdministrador)
	if err != nil {
		WriteJSON(w, 200, map[string]interface{}{"ok": true, "data": nil}); return
	}
	WriteJSON(w, 200, map[string]interface{}{"ok": true, "data": it})
}

func CreatePerfil(w http.ResponseWriter, r *http.Request) {
	b := parseBody(r)
	_, err := dbpkg.DB.Exec(
		"INSERT INTO Perfil (strNombrePerfil, bitAdministrador) VALUES (@p1, @p2)",
		sql.Named("p1", b["strNombrePerfil"]),
		sql.Named("p2", boolBody(b["bitAdministrador"])),
	)
	if err != nil {
		WriteErr(w, 500, err.Error()); return
	}
	WriteOK(w)
}

func UpdatePerfil(w http.ResponseWriter, r *http.Request) {
	id := intParam(r, "id")
	b := parseBody(r)
	_, err := dbpkg.DB.Exec(
		"UPDATE Perfil SET strNombrePerfil=@p1, bitAdministrador=@p2 WHERE id=@p3",
		sql.Named("p1", b["strNombrePerfil"]),
		sql.Named("p2", boolBody(b["bitAdministrador"])),
		sql.Named("p3", id),
	)
	if err != nil {
		WriteErr(w, 500, err.Error()); return
	}
	WriteOK(w)
}

func DeletePerfil(w http.ResponseWriter, r *http.Request) {
	id := intParam(r, "id")
	_, err := dbpkg.DB.Exec("DELETE FROM Perfil WHERE id=@p1", sql.Named("p1", id))
	if err != nil {
		WriteErr(w, 500, err.Error()); return
	}
	WriteOK(w)
}

// ─── Módulos ──────────────────────────────────────────────────────────────

func GetModulos(w http.ResponseWriter, r *http.Request) {
	page, limit, offset := parsePage(r)
	search := "%" + r.URL.Query().Get("search") + "%"

	var total int
	dbpkg.DB.QueryRow("SELECT COUNT(*) FROM Modulo WHERE strNombreModulo LIKE @p1", sql.Named("p1", search)).Scan(&total)

	rows, err := dbpkg.DB.Query(fmt.Sprintf(
		"SELECT id, strNombreModulo, strClaveModulo, strRuta FROM Modulo WHERE strNombreModulo LIKE @p1 ORDER BY id DESC OFFSET %d ROWS FETCH NEXT %d ROWS ONLY",
		offset, limit,
	), sql.Named("p1", search))
	if err != nil {
		WriteErr(w, 500, err.Error()); return
	}
	defer rows.Close()

	type item struct {
		ID             int    `json:"id"`
		StrNombreModulo string `json:"strNombreModulo"`
		StrClaveModulo  string `json:"strClaveModulo"`
		StrRuta         string `json:"strRuta"`
	}
	data := []item{}
	for rows.Next() {
		var it item
		rows.Scan(&it.ID, &it.StrNombreModulo, &it.StrClaveModulo, &it.StrRuta)
		data = append(data, it)
	}
	totalPages := int(math.Ceil(float64(total) / float64(limit)))
	if totalPages < 1 {
		totalPages = 1
	}
	WriteJSON(w, 200, map[string]interface{}{"ok": true, "data": data, "page": page, "totalPages": totalPages})
}

func GetModuloByID(w http.ResponseWriter, r *http.Request) {
	id := intParam(r, "id")
	var it struct {
		ID              int    `json:"id"`
		StrNombreModulo string `json:"strNombreModulo"`
		StrClaveModulo  string `json:"strClaveModulo"`
		StrRuta         string `json:"strRuta"`
	}
	err := dbpkg.DB.QueryRow("SELECT id, strNombreModulo, strClaveModulo, strRuta FROM Modulo WHERE id=@p1", sql.Named("p1", id)).
		Scan(&it.ID, &it.StrNombreModulo, &it.StrClaveModulo, &it.StrRuta)
	if err != nil {
		WriteJSON(w, 200, map[string]interface{}{"ok": true, "data": nil}); return
	}
	WriteJSON(w, 200, map[string]interface{}{"ok": true, "data": it})
}

func CreateModulo(w http.ResponseWriter, r *http.Request) {
	b := parseBody(r)
	_, err := dbpkg.DB.Exec(
		"INSERT INTO Modulo (strNombreModulo, strClaveModulo, strRuta) VALUES (@p1, @p2, @p3)",
		sql.Named("p1", b["strNombreModulo"]),
		sql.Named("p2", b["strClaveModulo"]),
		sql.Named("p3", b["strRuta"]),
	)
	if err != nil {
		WriteErr(w, 500, err.Error()); return
	}
	WriteOK(w)
}

func UpdateModulo(w http.ResponseWriter, r *http.Request) {
	id := intParam(r, "id")
	b := parseBody(r)
	_, err := dbpkg.DB.Exec(
		"UPDATE Modulo SET strNombreModulo=@p1, strClaveModulo=@p2, strRuta=@p3 WHERE id=@p4",
		sql.Named("p1", b["strNombreModulo"]),
		sql.Named("p2", b["strClaveModulo"]),
		sql.Named("p3", b["strRuta"]),
		sql.Named("p4", id),
	)
	if err != nil {
		WriteErr(w, 500, err.Error()); return
	}
	WriteOK(w)
}

func DeleteModulo(w http.ResponseWriter, r *http.Request) {
	id := intParam(r, "id")
	_, err := dbpkg.DB.Exec("DELETE FROM Modulo WHERE id=@p1", sql.Named("p1", id))
	if err != nil {
		WriteErr(w, 500, err.Error()); return
	}
	WriteOK(w)
}

// ─── Permisos Perfil ──────────────────────────────────────────────────────

func GetPermisosPerfil(w http.ResponseWriter, r *http.Request) {
	page, limit, offset := parsePage(r)
	idPerfil, _ := strconv.Atoi(r.URL.Query().Get("idPerfil"))

	if idPerfil > 0 {
		var total int
		dbpkg.DB.QueryRow("SELECT COUNT(*) FROM Modulo").Scan(&total)

		rows, err := dbpkg.DB.Query(fmt.Sprintf(`
			SELECT ISNULL(pp.id,0), m.id, @p1, ISNULL(pp.bitAgregar,0), ISNULL(pp.bitEditar,0),
				ISNULL(pp.bitConsulta,0), ISNULL(pp.bitEliminar,0), ISNULL(pp.bitDetalle,0),
				p.strNombrePerfil, m.strNombreModulo
			FROM Modulo m
			CROSS JOIN Perfil p
			LEFT JOIN PermisosPerfil pp ON pp.idModulo=m.id AND pp.idPerfil=p.id
			WHERE p.id=@p1
			ORDER BY m.id
			OFFSET %d ROWS FETCH NEXT %d ROWS ONLY`, offset, limit),
			sql.Named("p1", idPerfil))
		if err != nil {
			WriteErr(w, 500, err.Error()); return
		}
		defer rows.Close()

		type item struct {
			ID               int    `json:"id"`
			IDModulo         int    `json:"idModulo"`
			IDPerfil         int    `json:"idPerfil"`
			BitAgregar       bool   `json:"bitAgregar"`
			BitEditar        bool   `json:"bitEditar"`
			BitConsulta      bool   `json:"bitConsulta"`
			BitEliminar      bool   `json:"bitEliminar"`
			BitDetalle       bool   `json:"bitDetalle"`
			StrNombrePerfil  string `json:"strNombrePerfil"`
			StrNombreModulo  string `json:"strNombreModulo"`
		}
		data := []item{}
		for rows.Next() {
			var it item
			rows.Scan(&it.ID, &it.IDModulo, &it.IDPerfil, &it.BitAgregar, &it.BitEditar, &it.BitConsulta, &it.BitEliminar, &it.BitDetalle, &it.StrNombrePerfil, &it.StrNombreModulo)
			data = append(data, it)
		}
		totalPages := int(math.Ceil(float64(total) / float64(limit)))
		if totalPages < 1 {
			totalPages = 1
		}
		WriteJSON(w, 200, map[string]interface{}{"ok": true, "data": data, "page": page, "totalPages": totalPages})
		return
	}

	// No idPerfil: return perfiles list
	rows, err := dbpkg.DB.Query("SELECT id, strNombrePerfil FROM Perfil ORDER BY strNombrePerfil")
	if err != nil {
		WriteErr(w, 500, err.Error()); return
	}
	defer rows.Close()
	type pf struct {
		ID              int    `json:"id"`
		StrNombrePerfil string `json:"strNombrePerfil"`
	}
	perfiles := []pf{}
	for rows.Next() {
		var it pf
		rows.Scan(&it.ID, &it.StrNombrePerfil)
		perfiles = append(perfiles, it)
	}
	WriteJSON(w, 200, map[string]interface{}{"ok": true, "perfiles": perfiles})
}

func UpsertPermisosPerfil(w http.ResponseWriter, r *http.Request) {
	b := parseBody(r)
	idStr := mux.Vars(r)["id"]
	id, _ := strconv.Atoi(idStr)
	idModulo, _ := strconv.Atoi(b["idModulo"])
	idPerfil, _ := strconv.Atoi(b["idPerfil"])

	ag := boolBody(b["bitAgregar"])
	ed := boolBody(b["bitEditar"])
	co := boolBody(b["bitConsulta"])
	el := boolBody(b["bitEliminar"])
	de := boolBody(b["bitDetalle"])

	if id > 0 {
		_, err := dbpkg.DB.Exec(`UPDATE PermisosPerfil SET idModulo=@p1,idPerfil=@p2,bitAgregar=@p3,bitEditar=@p4,bitConsulta=@p5,bitEliminar=@p6,bitDetalle=@p7 WHERE id=@p8`,
			sql.Named("p1", idModulo), sql.Named("p2", idPerfil),
			sql.Named("p3", ag), sql.Named("p4", ed), sql.Named("p5", co), sql.Named("p6", el), sql.Named("p7", de),
			sql.Named("p8", id))
		if err != nil {
			WriteErr(w, 500, err.Error()); return
		}
	} else {
		var existingID int
		err := dbpkg.DB.QueryRow("SELECT TOP 1 id FROM PermisosPerfil WHERE idPerfil=@p1 AND idModulo=@p2",
			sql.Named("p1", idPerfil), sql.Named("p2", idModulo)).Scan(&existingID)
		if err == nil && existingID > 0 {
			_, err = dbpkg.DB.Exec(`UPDATE PermisosPerfil SET idModulo=@p1,idPerfil=@p2,bitAgregar=@p3,bitEditar=@p4,bitConsulta=@p5,bitEliminar=@p6,bitDetalle=@p7 WHERE id=@p8`,
				sql.Named("p1", idModulo), sql.Named("p2", idPerfil),
				sql.Named("p3", ag), sql.Named("p4", ed), sql.Named("p5", co), sql.Named("p6", el), sql.Named("p7", de),
				sql.Named("p8", existingID))
		} else {
			_, err = dbpkg.DB.Exec(`INSERT INTO PermisosPerfil (idModulo,idPerfil,bitAgregar,bitEditar,bitConsulta,bitEliminar,bitDetalle) VALUES (@p1,@p2,@p3,@p4,@p5,@p6,@p7)`,
				sql.Named("p1", idModulo), sql.Named("p2", idPerfil),
				sql.Named("p3", ag), sql.Named("p4", ed), sql.Named("p5", co), sql.Named("p6", el), sql.Named("p7", de))
		}
		if err != nil {
			WriteErr(w, 500, err.Error()); return
		}
	}
	WriteOK(w)
}

func DeletePermisosPerfil(w http.ResponseWriter, r *http.Request) {
	id := intParam(r, "id")
	_, err := dbpkg.DB.Exec("DELETE FROM PermisosPerfil WHERE id=@p1", sql.Named("p1", id))
	if err != nil {
		WriteErr(w, 500, err.Error()); return
	}
	WriteOK(w)
}

// ─── Usuarios ─────────────────────────────────────────────────────────────

func GetUsuarios(w http.ResponseWriter, r *http.Request) {
	page, limit, offset := parsePage(r)
	search := "%" + r.URL.Query().Get("search") + "%"

	var total int
	dbpkg.DB.QueryRow("SELECT COUNT(*) FROM Usuario WHERE strNombreUsuario LIKE @p1 OR strCorreo LIKE @p1",
		sql.Named("p1", search)).Scan(&total)

	rows, err := dbpkg.DB.Query(fmt.Sprintf(`
		SELECT u.id, u.strNombreUsuario, u.idPerfil, u.strPwd, u.idEstadoUsuario,
			u.strCorreo, u.strNumeroCelular, u.strImagen, p.strNombrePerfil, eu.strNombreEstado
		FROM Usuario u
		INNER JOIN Perfil p ON p.id=u.idPerfil
		INNER JOIN EstadoUsuario eu ON eu.id=u.idEstadoUsuario
		WHERE u.strNombreUsuario LIKE @p1 OR u.strCorreo LIKE @p1
		ORDER BY u.id DESC OFFSET %d ROWS FETCH NEXT %d ROWS ONLY`, offset, limit),
		sql.Named("p1", search))
	if err != nil {
		WriteErr(w, 500, err.Error()); return
	}
	defer rows.Close()

	type item struct {
		ID               int            `json:"id"`
		StrNombreUsuario string         `json:"strNombreUsuario"`
		IDPerfil         int            `json:"idPerfil"`
		StrPwd           string         `json:"strPwd"`
		IDEstadoUsuario  int            `json:"idEstadoUsuario"`
		StrCorreo        string         `json:"strCorreo"`
		StrNumeroCelular sql.NullString `json:"strNumeroCelular"`
		StrImagen        sql.NullString `json:"strImagen"`
		StrNombrePerfil  string         `json:"strNombrePerfil"`
		StrNombreEstado  string         `json:"strNombreEstado"`
	}
	data := []item{}
	for rows.Next() {
		var it item
		rows.Scan(&it.ID, &it.StrNombreUsuario, &it.IDPerfil, &it.StrPwd, &it.IDEstadoUsuario,
			&it.StrCorreo, &it.StrNumeroCelular, &it.StrImagen, &it.StrNombrePerfil, &it.StrNombreEstado)
		data = append(data, it)
	}
	totalPages := int(math.Ceil(float64(total) / float64(limit)))
	if totalPages < 1 {
		totalPages = 1
	}
	WriteJSON(w, 200, map[string]interface{}{"ok": true, "data": data, "page": page, "totalPages": totalPages})
}

func GetUsuarioByID(w http.ResponseWriter, r *http.Request) {
	id := intParam(r, "id")
	var it struct {
		ID               int            `json:"id"`
		StrNombreUsuario string         `json:"strNombreUsuario"`
		IDPerfil         int            `json:"idPerfil"`
		StrPwd           string         `json:"strPwd"`
		IDEstadoUsuario  int            `json:"idEstadoUsuario"`
		StrCorreo        string         `json:"strCorreo"`
		StrNumeroCelular sql.NullString `json:"strNumeroCelular"`
		StrImagen        sql.NullString `json:"strImagen"`
	}
	err := dbpkg.DB.QueryRow(`SELECT id, strNombreUsuario, idPerfil, strPwd, idEstadoUsuario, strCorreo, strNumeroCelular, strImagen FROM Usuario WHERE id=@p1`,
		sql.Named("p1", id)).Scan(&it.ID, &it.StrNombreUsuario, &it.IDPerfil, &it.StrPwd, &it.IDEstadoUsuario, &it.StrCorreo, &it.StrNumeroCelular, &it.StrImagen)
	if err != nil {
		WriteJSON(w, 200, map[string]interface{}{"ok": true, "data": nil}); return
	}
	WriteJSON(w, 200, map[string]interface{}{"ok": true, "data": it})
}

func CreateUsuario(uploadDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.ParseMultipartForm(3 << 20)
		telefono := onlyDigits(r.FormValue("strNumeroCelular"))
		if !isValidPhone(telefono) {
			WriteErr(w, 400, "El número de teléfono debe tener exactamente 10 dígitos."); return
		}
		var strImagen sql.NullString
		if fh := getFileHeader(r, "strImagen"); fh != nil {
			imgPath, err := saveUpload(fh, uploadDir)
			if err != nil {
				WriteErr(w, 500, err.Error()); return
			}
			strImagen = sql.NullString{String: imgPath, Valid: true}
		}
		_, err := dbpkg.DB.Exec(`INSERT INTO Usuario (strNombreUsuario,idPerfil,strPwd,idEstadoUsuario,strCorreo,strNumeroCelular,strImagen) VALUES (@p1,@p2,@p3,@p4,@p5,@p6,@p7)`,
			sql.Named("p1", r.FormValue("strNombreUsuario")),
			sql.Named("p2", r.FormValue("idPerfil")),
			sql.Named("p3", r.FormValue("strPwd")),
			sql.Named("p4", r.FormValue("idEstadoUsuario")),
			sql.Named("p5", r.FormValue("strCorreo")),
			sql.Named("p6", telefono),
			sql.Named("p7", strImagen))
		if err != nil {
			WriteErr(w, 500, err.Error()); return
		}
		WriteOK(w)
	}
}

func UpdateUsuario(uploadDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := intParam(r, "id")
		r.ParseMultipartForm(3 << 20)
		telefono := onlyDigits(r.FormValue("strNumeroCelular"))
		if !isValidPhone(telefono) {
			WriteErr(w, 400, "El número de teléfono debe tener exactamente 10 dígitos."); return
		}
		var strImagen sql.NullString
		if fh := getFileHeader(r, "strImagen"); fh != nil {
			imgPath, err := saveUpload(fh, uploadDir)
			if err != nil {
				WriteErr(w, 500, err.Error()); return
			}
			strImagen = sql.NullString{String: imgPath, Valid: true}
		} else {
			var old sql.NullString
			dbpkg.DB.QueryRow("SELECT strImagen FROM Usuario WHERE id=@p1", sql.Named("p1", id)).Scan(&old)
			strImagen = old
		}
		_, err := dbpkg.DB.Exec(`UPDATE Usuario SET strNombreUsuario=@p1,idPerfil=@p2,strPwd=@p3,idEstadoUsuario=@p4,strCorreo=@p5,strNumeroCelular=@p6,strImagen=@p7 WHERE id=@p8`,
			sql.Named("p1", r.FormValue("strNombreUsuario")),
			sql.Named("p2", r.FormValue("idPerfil")),
			sql.Named("p3", r.FormValue("strPwd")),
			sql.Named("p4", r.FormValue("idEstadoUsuario")),
			sql.Named("p5", r.FormValue("strCorreo")),
			sql.Named("p6", telefono),
			sql.Named("p7", strImagen),
			sql.Named("p8", id))
		if err != nil {
			WriteErr(w, 500, err.Error()); return
		}
		WriteOK(w)
	}
}

func DeleteUsuario(w http.ResponseWriter, r *http.Request) {
	id := intParam(r, "id")
	_, err := dbpkg.DB.Exec("DELETE FROM Usuario WHERE id=@p1", sql.Named("p1", id))
	if err != nil {
		WriteErr(w, 500, err.Error()); return
	}
	WriteOK(w)
}

// ─── File upload helpers ──────────────────────────────────────────────────

func getFileHeader(r *http.Request, field string) *multipart.FileHeader {
	if r.MultipartForm == nil {
		return nil
	}
	files := r.MultipartForm.File[field]
	if len(files) == 0 {
		return nil
	}
	return files[0]
}

func saveUpload(fh *multipart.FileHeader, dir string) (string, error) {
	allowed := map[string]bool{"image/png": true, "image/jpeg": true, "image/jpg": true, "image/webp": true}
	ct := fh.Header.Get("Content-Type")
	if !allowed[ct] {
		return "", fmt.Errorf("formato de imagen no permitido")
	}
	ext := strings.ToLower(filepath.Ext(fh.Filename))
	if ext == "" {
		ext = ".png"
	}
	filename := fmt.Sprintf("%d-%d%s", time.Now().UnixMilli(), time.Now().UnixNano()%1e9, ext)
	dst := filepath.Join(dir, filename)
	f, err := fh.Open()
	if err != nil {
		return "", err
	}
	defer f.Close()
	out, err := os.Create(dst)
	if err != nil {
		return "", err
	}
	defer out.Close()
	io.Copy(out, f)
	return "/uploads/users/" + filename, nil
}


