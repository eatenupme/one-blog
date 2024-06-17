package ms

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/eatenupme/one-blog/cache"
	"github.com/eatenupme/one-blog/config"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/microsoft"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/singleflight"
)

var cfolder = cache.NewCache(128)
var sfolder = &singleflight.Group{}

func MultiFolder(ctx context.Context, paths []string, pagetoken string) (map[string]*Children, error) {
	g := &errgroup.Group{}
	g.SetLimit(5)
	m := sync.Map{}

	for i := (0); i < len(paths); i++ {
		g.Go(func(path string) func() error {
			return func() error {
				folder, err := cachefolder(ctx, path, pagetoken, cfolder, sfolder)
				if err != nil {
					return err
				}
				m.LoadOrStore(path, folder)
				return nil
			}
		}(paths[i]))
	}
	err := g.Wait()
	if err != nil {
		return nil, err
	}
	c := map[string]*Children{}
	m.Range(func(key, value any) bool {
		path := key.(string)
		data := value.(*Children)
		c[path] = data
		return true
	})
	return c, nil
}

func Folder(ctx context.Context, path string, pagetoken string) (*Children, error) {
	children, err := cachefolder(ctx, path, pagetoken, cfolder, sfolder)
	if err != nil {
		return nil, err
	}
	return children, err
}

var cachefolder = func(ctx context.Context, path string, skiptoken string, c *cache.Cache, s *singleflight.Group) (*Children, error) {
	childreni, err, _ := s.Do(path, func() (interface{}, error) {
		key := path
		if skiptoken != "" {
			key = path + "?skiptoken=" + skiptoken
		}
		if datai, ok := c.Get(key); ok {
			return datai, nil
		}
		slog.InfoContext(ctx, fmt.Sprintf("skiptoken:%v", path))
		children, err := folder(ctx, path, skiptoken)
		if err != nil {
			return nil, err
		}
		c.Put(key, children, cache.WithExpire(60*time.Second))
		return children, err
	})
	if err != nil {
		return nil, err
	}
	children, _ := childreni.(*Children)
	return children, err
}

var folder = func(ctx context.Context, path string, skiptoken string) (*Children, error) {
	mspath := fmt.Sprintf(":%v%v:", config.Golbal.ROOT_PATH, path)
	if mspath == "::" {
		mspath = ""
	}
	uri := fmt.Sprintf("https://graph.microsoft.com/v1.0/me/drive/root%v/children", mspath)
	params := []string{"orderby=lastModifiedDateTime%20desc", "top=10"}
	if skiptoken != "" {
		params = append(params, fmt.Sprintf("skiptoken=%v", skiptoken))
	}

	param := "?" + strings.Join(params, "&")
	tokenSource := (&oauth2.Config{
		ClientID:     config.Golbal.CLIENT_ID,
		ClientSecret: config.Golbal.CLIENT_SECRET,
		Scopes:       Scopes,
		RedirectURL:  config.Golbal.REDIRECT_URI,
		Endpoint:     microsoft.AzureADEndpoint("consumers"),
	}).TokenSource(ctx, Token)
	resp, err := oauth2.NewClient(ctx, tokenSource).Get(uri + param)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	Token, err = tokenSource.Token()
	if err != nil {
		return nil, err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	children := &Children{}
	err = json.Unmarshal(body, children)
	if err != nil {
		return nil, err
	} else if children.Error != nil {
		return nil, fmt.Errorf("code:%+v msg:%+v", children.Error.Code, children.Error.Message)
	}
	if children.Odata_nextLink != "" {
		u, err := url.Parse(children.Odata_nextLink)
		if err != nil {
			return nil, err
		}
		children.Odata_nextLink = u.Query().Get("$skiptoken")
	}
	for idx := range children.Value {
		children.Value[idx].ParentReference.Path = strings.TrimPrefix(children.Value[idx].ParentReference.Path,
			"/drive/root:"+config.Golbal.ROOT_PATH)
	}
	return children, nil
}

var cfile = cache.NewCache(128)
var sfile = &singleflight.Group{}

func File(ctx context.Context, path string) (*DriveItem, error) {
	file, err := cachefile(ctx, path, cfile, sfile)
	if err != nil {
		return nil, err
	}
	return file, err
}

var cachefile = func(ctx context.Context, path string, c *cache.Cache, s *singleflight.Group) (*DriveItem, error) {
	driveItemi, err, _ := s.Do(path, func() (interface{}, error) {
		if datai, ok := c.Get(path); ok {
			return datai, nil
		}

		driveItem, err := file(ctx, path)
		if err != nil {
			return nil, err
		}
		c.Put(path, driveItem, cache.WithExpire(60*time.Second))
		return driveItem, err
	})
	if err != nil {
		return nil, err
	}
	driveItem, _ := driveItemi.(*DriveItem)
	return driveItem, nil
}

var file = func(ctx context.Context, path string) (*DriveItem, error) {
	mspath := fmt.Sprintf(":%v%v:", config.Golbal.ROOT_PATH, path)
	if mspath == "::" {
		mspath = ""
	}
	url := fmt.Sprintf("https://graph.microsoft.com/v1.0/me/drive/root%v", mspath)

	tokenSource := (&oauth2.Config{
		ClientID:     config.Golbal.CLIENT_ID,
		ClientSecret: config.Golbal.CLIENT_SECRET,
		Scopes:       Scopes,
		RedirectURL:  config.Golbal.REDIRECT_URI,
		Endpoint:     microsoft.AzureADEndpoint("consumers"),
	}).TokenSource(ctx, Token)
	resp, err := oauth2.NewClient(ctx, tokenSource).Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	Token, err = tokenSource.Token()
	if err != nil {
		return nil, err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	driveItem := &DriveItem{}
	json.Unmarshal(body, driveItem)
	if err != nil {
		return nil, err
	} else if driveItem.Error != nil {
		return nil, fmt.Errorf("code:%+v msg:%+v", driveItem.Error.Code, driveItem.Error.Message)
	}

	driveItem.ParentReference.Path = strings.TrimPrefix(driveItem.ParentReference.Path,
		"/drive/root:"+config.Golbal.ROOT_PATH)
	return driveItem, nil
}

type Children struct {
	Odata_count    int64        `json:"@odata.count"`
	Odata_nextLink string       `json:"@odata.nextLink"`
	Value          []*DriveItem `json:"value"`
	Error          *Error       `json:"error"`
}

type DriveItem struct {
	DownloadURL    string `json:"@microsoft.graph.downloadUrl"`
	Name           string `json:"name"`
	Size           int    `json:"size"`
	FileSystemInfo struct {
		CreatedDateTime      time.Time `json:"createdDateTime"`
		LastModifiedDateTime time.Time `json:"lastModifiedDateTime"`
	} `json:"fileSystemInfo"`
	ParentReference *struct {
		Name string `json:"name"`
		Path string `json:"path"`
	} `json:"parentReference"`
	File *struct {
		MimeType string `json:"mimeType"`
	} `json:"file"`
	Folder *struct {
		ChildCount int64 `json:"childCount"`
	} `json:"folder"`
	Error *Error `json:"error"`
}

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
