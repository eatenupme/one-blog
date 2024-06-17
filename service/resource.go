package service

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/eatenupme/one-blog/cache"
	"github.com/eatenupme/one-blog/ms"
	"github.com/eatenupme/one-blog/tmpl"

	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/singleflight"
)

var s = &singleflight.Group{}
var c = cache.NewCache(128)
var tmp = cache.NewCache(64)

func Resource(w http.ResponseWriter, r *http.Request) {
	titlef := func() string {
		u := strings.Split(strings.TrimSuffix(r.URL.Path, "/"), "/")
		if len(u) > 0 {
			return u[len(u)-1]
		}
		return r.URL.Hostname()
	}
	cfile, cfileerr := make(chan *ms.DriveItem, 1), make(chan error, 1)
	tresfile, tfileerr := make(chan string, 1), make(chan error, 1)
	cfolder, cfoldererr := make(chan *ms.Children, 1), make(chan error, 1)
	go func() {
		folder, err := ms.Folder(r.Context(), strings.TrimSuffix(r.URL.Path, "/"), r.URL.Query().Get("skiptoken"))
		if err != nil {
			cfolder <- nil
			cfoldererr <- err
			return
		}
		cfolder <- folder
		cfoldererr <- nil
	}()
	go func() {
		file, err := ms.File(r.Context(), strings.TrimSuffix(r.URL.Path, "/"))
		if err != nil {
			cfile <- nil
			cfileerr <- err
			return
		}
		cfile <- file
		cfileerr <- nil
	}()
	go func() {
		file, err := ms.File(r.Context(), "/theme.html")
		if err != nil {
			tresfile <- ""
			tfileerr <- err
			return
		}
		b, err := ResourceFile(r.Context(), file.DownloadURL)
		if err != nil {
			tresfile <- ""
			tfileerr <- err
			return
		}
		tresfile <- string(b)
		tfileerr <- nil
	}()
	theme, themeerr := <-tresfile, <-tfileerr
	if themeerr != nil {
		slog.ErrorContext(r.Context(), fmt.Sprintf("path:%+v file:%+v", r.URL.Path, tfileerr))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	folder, foldererr := <-cfolder, <-cfoldererr
	file, fileerr := <-cfile, <-cfileerr
	if folder == nil && file == nil {
		slog.ErrorContext(r.Context(), fmt.Sprintf("path:%+v folder:%+v file:%+v", r.URL.Path, foldererr, fileerr))
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	if file.File != nil && file.Size > 1024*1024 {
		http.Redirect(w, r, file.DownloadURL, http.StatusFound)
		return
	} else if file.File != nil && file.Size < 1024*1024 {
		b, err := ResourceFile(r.Context(), file.DownloadURL)
		if err != nil {
			slog.ErrorContext(r.Context(), fmt.Sprintf("%+v", err))
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		}
		w.Header().Set("Content-Type", mime.TypeByExtension(filepath.Ext(file.Name)))
		w.Header().Set("Cache-Control", "max-age=3600")
		w.Header().Set("Expires", time.Now().Add(time.Hour*1).Format(http.TimeFormat))
		if _, err = w.Write(b); err != nil {
			slog.ErrorContext(r.Context(), fmt.Sprintf("%+v", err))
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
		return
	}

	f := func(folder *ms.Children) (file *ms.DriveItem, has bool) {
		for i := (0); i < len(folder.Value); i++ {
			// if strings.ToLower(folder.Value[i].Name) == "index.md" || strings.ToLower(folder.Value[i].Name) == "index.html" {
			if folder.Value[i].File != nil && strings.ToLower(folder.Value[i].Name) == "index.md" {
				return folder.Value[i], true
			}
		}
		return nil, false
	}
	if file, has := f(folder); has && file != nil {
		data, err := ResourceDocument(r.Context(), file.DownloadURL, true)
		if err != nil {
			slog.ErrorContext(r.Context(), fmt.Sprintf("%+v", err))
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		tmpl.TemplateDocument(w, r, theme, titlef(), "", []string{data})
		return
	}

	paths := []string{}
	for i := (0); i < len(folder.Value); i++ {
		paths = append(paths, folder.Value[i].ParentReference.Path+"/"+folder.Value[i].Name)
	}
	multifolder, err := ms.MultiFolder(r.Context(), paths, "")
	if err != nil {
		slog.ErrorContext(r.Context(), fmt.Sprintf("%+v", err))
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	folders := []*ms.Children{}
	for _, path := range paths {
		if _, ok := multifolder[path]; !ok {
			continue
		}
		folders = append(folders, multifolder[path])
	}

	paths = []string{}
	for _, d := range folders {
		file, has := f(d)
		if !has {
			continue
		}
		paths = append(paths, file.DownloadURL)
	}
	multidatas, err := MultiResourceDocument(r.Context(), paths, false)
	if err != nil {
		slog.ErrorContext(r.Context(), fmt.Sprintf("%+v", err))
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	datas := []string{}
	for _, path := range paths {
		if _, ok := multidatas[path]; !ok {
			continue
		}
		datas = append(datas, multidatas[path])
	}
	tmpl.TemplateDocument(w, r, theme, titlef(), folder.Odata_nextLink, datas)
}

func MultiResourceDocument(ctx context.Context, respaths []string, more bool) (data map[string]string, err error) {
	g := &errgroup.Group{}
	g.SetLimit(5)
	m := sync.Map{}

	for i := (0); i < len(respaths); i++ {
		g.Go(func(path string) func() error {
			return func() error {
				data, err := ResourceDocument(ctx, path, more)
				if err != nil {
					return err
				}
				m.LoadOrStore(path, data)
				return nil
			}
		}(respaths[i]))
	}
	err = g.Wait()
	if err != nil {
		return nil, err
	}
	c := map[string]string{}
	m.Range(func(key, value any) bool {
		path := key.(string)
		data := value.(string)
		c[path] = data
		return true
	})
	return c, nil
}

func ResourceFile(ctx context.Context, respath string) (data []byte, err error) {
	return cacheresourcefile(ctx, respath, 1*time.Hour, tmp, s)
}

func ResourceDocument(ctx context.Context, respath string, more bool) (data string, err error) {
	d, err := cacheresourcefile(ctx, respath, 5*time.Minute, c, s)
	if err != nil {
		return "", err
	}
	data = string(d)
	if more {
		return data, err
	}
	if l := strings.Index(data, "<!-- more -->"); l != -1 {
		return data[0:l], err
	}
	return data, err
}

var cacheresourcefile = func(ctx context.Context, respath string, d time.Duration, c *cache.Cache, s *singleflight.Group) (data []byte, err error) {
	datai, err, _ := s.Do(respath, func() (interface{}, error) {
		if datai, ok := c.Get(respath); ok {
			data, _ := datai.([]byte)
			return data, err
		}
		if data, err = resourcefile(ctx, respath); err != nil {
			return nil, err
		}
		c.Put(respath, data, cache.WithExpire(d))
		return data, err
	})
	if err != nil {
		return nil, err
	}
	data, _ = datai.([]byte)
	return data, err
}

var resourcefile = func(ctx context.Context, respath string) (data []byte, err error) {
	_ = ctx
	response, err := http.Get(respath)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	data, err = io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	return data, err
}
