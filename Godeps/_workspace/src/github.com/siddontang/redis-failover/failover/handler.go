package failover

import (
	"net/http"
	"strings"
)

type masterHandler struct {
	a *App
}

func (h *masterHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		masters := h.a.masters.GetMasters()
		w.Write([]byte(strings.Join(masters, ",")))
	case "POST":
		masters := strings.Split(r.FormValue("masters"), ",")
		h.a.addMasters(masters)
	case "PUT":
		masters := strings.Split(r.FormValue("masters"), ",")
		h.a.setMasters(masters)
	case "DELETE":
		masters := strings.Split(r.FormValue("masters"), ",")
		h.a.delMasters(masters)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
}
