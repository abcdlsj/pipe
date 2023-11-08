package server

import (
	"embed"
	"html/template"
	"io/fs"
	"net/http"
	"strconv"

	"github.com/abcdlsj/pipe/logger"
)

var (
	//go:embed assets
	assetsFs embed.FS
)

func (s *Server) startAdmin() {
	tmpl := template.Must(template.New("").ParseFS(assetsFs, "assets/*.html"))

	fe, _ := fs.Sub(assetsFs, "assets/static")
	http.Handle("/static/", http.StripPrefix("/static", http.FileServer(http.FS(fe))))

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if err := tmpl.ExecuteTemplate(w, "index.html", map[string]interface{}{
			"proxys": s.proxys,
		}); err != nil {
			logger.Errorf("execute index.html error: %v", err)
		}
	})

	if err := http.ListenAndServe(":"+strconv.Itoa(s.cfg.AdminPort), nil); err != nil {
		logger.Fatalf("admin server error: %v", err)
	}
}
