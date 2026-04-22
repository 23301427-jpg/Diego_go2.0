package services

import (
	"database/sql"

	dbpkg "github.com/23301427-jpg/Diego_go2.0/internal/db"
	mw "github.com/23301427-jpg/Diego_go2.0/internal/middleware"
)

type MenuModulo struct {
	ID     int
	Nombre string
	Clave  string
	Ruta   string
}

type Menu struct {
	ID      int
	Nombre  string
	Modulos []MenuModulo
}

type UserInfo struct {
	ID               int
	StrNombreUsuario string
	StrCorreo        string
	StrNumeroCelular sql.NullString
	StrImagen        sql.NullString
	StrNombrePerfil  string
}

type DashboardPayload struct {
	User        UserInfo
	Menus       []Menu
	Permissions map[string]*mw.Permission
}

func GetDashboardPayload(idUsuario, idPerfil int, permissions map[string]*mw.Permission) (*DashboardPayload, error) {
	rows, err := dbpkg.DB.Query(`
		SELECT mn.id, mn.strNombreMenu, mn.intOrdenMenu,
			m.id, m.strNombreModulo, m.strClaveModulo, m.strRuta,
			ISNULL(pp.bitAgregar,0), ISNULL(pp.bitEditar,0),
			ISNULL(pp.bitConsulta,0), ISNULL(pp.bitEliminar,0), ISNULL(pp.bitDetalle,0)
		FROM Menu mn
		INNER JOIN MenuModulo mm ON mm.idMenu = mn.id
		INNER JOIN Modulo m ON m.id = mm.idModulo
		LEFT JOIN PermisosPerfil pp ON pp.idModulo=m.id AND pp.idPerfil=@p1
		ORDER BY mn.intOrdenMenu, m.id
	`, sql.Named("p1", idPerfil))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	menuMap := make(map[int]*Menu)
	menuOrder := []int{}

	for rows.Next() {
		var mnID, mnOrden, mID int
		var mnNombre, mNombre, mClave, mRuta string
		var ag, ed, co, el, de bool
		if err := rows.Scan(&mnID, &mnNombre, &mnOrden, &mID, &mNombre, &mClave, &mRuta, &ag, &ed, &co, &el, &de); err != nil {
			return nil, err
		}
		allowed := ag || ed || co || el || de
		if _, ok := menuMap[mnID]; !ok {
			menuMap[mnID] = &Menu{ID: mnID, Nombre: mnNombre}
			menuOrder = append(menuOrder, mnID)
		}
		if allowed {
			menuMap[mnID].Modulos = append(menuMap[mnID].Modulos, MenuModulo{
				ID: mID, Nombre: mNombre, Clave: mClave, Ruta: mRuta,
			})
		}
	}

	visibleMenus := []Menu{}
	for _, id := range menuOrder {
		m := menuMap[id]
		if len(m.Modulos) > 0 {
			visibleMenus = append(visibleMenus, *m)
		}
	}

	var u UserInfo
	err = dbpkg.DB.QueryRow(`
		SELECT u.id, u.strNombreUsuario, u.strCorreo, u.strNumeroCelular, u.strImagen, p.strNombrePerfil
		FROM Usuario u
		INNER JOIN Perfil p ON p.id = u.idPerfil
		WHERE u.id = @p1
	`, sql.Named("p1", idUsuario)).Scan(&u.ID, &u.StrNombreUsuario, &u.StrCorreo, &u.StrNumeroCelular, &u.StrImagen, &u.StrNombrePerfil)
	if err != nil {
		return nil, err
	}

	return &DashboardPayload{User: u, Menus: visibleMenus, Permissions: permissions}, nil
}
