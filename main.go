package main

import (
	"log"
	"log/slog"
	"net/http"
	"os"

	"github.com/eatenupme/one-blog/config"
	"github.com/eatenupme/one-blog/ms"
	"github.com/eatenupme/one-blog/service"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: true,
	})))
	err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	router := http.NewServeMux()
	router.Handle("/", service.ThrottlingMiddleware(ms.UnauthorizedMiddleware(http.HandlerFunc(service.Resource))))
	router.Handle("/authorize", http.HandlerFunc(ms.AuthorizeClient))
	slog.Info("Server listening on port https...")
	log.Fatal(http.ListenAndServeTLS(":443", "server.crt", "server.key", router))
}
