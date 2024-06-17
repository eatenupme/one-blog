package tmpl

import (
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"strings"
)

func TemplateDocument(w http.ResponseWriter, r *http.Request, index string, title string, next string, data []string) {
	tmpl, err := template.New("index").Parse(index)
	if err != nil {
		slog.ErrorContext(r.Context(), fmt.Sprintf("%+v", err))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	tmp := map[string]interface{}{"title": title, "next": next, "url": strings.TrimSuffix(r.URL.Path, "/"), "param": data}
	if err := tmpl.Execute(w, tmp); err != nil {
		slog.ErrorContext(r.Context(), fmt.Sprintf("%+v", err))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
}
