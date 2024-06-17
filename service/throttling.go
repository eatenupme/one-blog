package service

import (
	"net/http"
	"sync"

	"github.com/eatenupme/one-blog/config"

	"golang.org/x/sync/errgroup"
)

var g = &errgroup.Group{}
var once = &sync.Once{}

func ThrottlingMiddleware(next http.Handler) http.Handler {
	once.Do(func() {
		g.SetLimit(config.Golbal.RATE_LIMIT)
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c := make(chan struct{}, 1)
		if !g.TryGo(func() error {
			next.ServeHTTP(w, r)
			c <- struct{}{}
			return nil
		}) {
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			return
		}
		<-c
	})
}
