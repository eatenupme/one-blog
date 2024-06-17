package ms

import (
	"fmt"
	"log/slog"
	"net/http"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/microsoft"

	"github.com/eatenupme/one-blog/config"
)

var (
	Token  *oauth2.Token
	Scopes = []string{"Files.ReadWrite.All", "offline_access"}
)

func UnauthorizedMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if Token != nil {
			next.ServeHTTP(w, r)
			return
		}

		redirect := (&oauth2.Config{
			ClientID:    config.Golbal.CLIENT_ID,
			Scopes:      Scopes,
			RedirectURL: config.Golbal.REDIRECT_URI,
			Endpoint:    microsoft.AzureADEndpoint("consumers"),
		}).AuthCodeURL(r.URL.Path)
		http.Redirect(w, r, redirect, http.StatusFound)
	})
}

func AuthorizeClient(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	token, err := (&oauth2.Config{
		ClientID:     config.Golbal.CLIENT_ID,
		ClientSecret: config.Golbal.CLIENT_SECRET,
		Scopes:       Scopes,
		RedirectURL:  config.Golbal.REDIRECT_URI,
		Endpoint:     microsoft.AzureADEndpoint("consumers"),
	}).Exchange(r.Context(), code)
	if err != nil {
		slog.ErrorContext(r.Context(), fmt.Sprintf("%+v", err))
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}
	Token = token
	http.Redirect(w, r, r.URL.Query().Get("state"), http.StatusFound)
}
