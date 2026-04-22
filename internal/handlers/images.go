package handlers

import (
	"context"
	"net/http"
	"os"
	"sync"
	"time"

	cloudinary "github.com/cloudinary/cloudinary-go/v2"
	"github.com/cloudinary/cloudinary-go/v2/api/admin"
	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
)

var (
	cldOnce  sync.Once
	cld      *cloudinary.Cloudinary
	imgCache []string
	cacheTS  time.Time
	cacheMu  sync.Mutex
)

func getCld() *cloudinary.Cloudinary {
	cldOnce.Do(func() {
		var err error
		cld, err = cloudinary.NewFromParams(
			getEnvOr("CLOUDINARY_CLOUD_NAME", "dqhqrjoe6"),
			getEnvOr("CLOUDINARY_API_KEY", "359285996587242"),
			os.Getenv("CLOUDINARY_API_SECRET"),
		)
		if err != nil {
			panic("cloudinary init error: " + err.Error())
		}
	})
	return cld
}

func UploadImage(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		WriteErr(w, 400, "Error al procesar archivo")
		return
	}
	f, _, err := r.FormFile("imagen")
	if err != nil {
		WriteErr(w, 400, "No se recibió ningún archivo")
		return
	}
	defer f.Close()

	ctx := context.Background()
	_, err = getCld().Upload.Upload(ctx, f, uploader.UploadParams{Folder: "gafitas"})
	if err != nil {
		WriteErr(w, 500, err.Error())
		return
	}

	// Bust cache so next GET fetches fresh list
	cacheMu.Lock()
	imgCache = nil
	cacheMu.Unlock()

	WriteOK(w)
}

func GetImages(w http.ResponseWriter, r *http.Request) {
	cacheMu.Lock()
	if len(imgCache) > 0 && time.Since(cacheTS) < 60*time.Second {
		imgs := imgCache
		cacheMu.Unlock()
		WriteJSON(w, 200, imgs)
		return
	}
	cacheMu.Unlock()

	ctx := context.Background()
	folder := "gafitas"
	maxResults := 50

	result, err := getCld().Admin.Assets(ctx, admin.AssetsParams{
		AssetType:  "image",
		Prefix:     folder + "/",
		MaxResults: maxResults,
	})
	if err != nil {
		// Return empty list rather than error — gallery just shows nothing
		WriteJSON(w, 200, []string{})
		return
	}

	urls := make([]string, 0, len(result.Assets))
	for _, asset := range result.Assets {
		urls = append(urls, asset.SecureURL)
	}

	cacheMu.Lock()
	imgCache = urls
	cacheTS = time.Now()
	cacheMu.Unlock()

	WriteJSON(w, 200, urls)
}
