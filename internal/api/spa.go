package api

import (
	"io/fs"
	"net/http"
	"net/url"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/valyala/fasthttp"
)

// newSPAHandler serves an embedded [fs.FS] with SPA fallback to index.html for unknown paths.
func newSPAHandler(fsys fs.FS) fiber.Handler {
	return func(c fiber.Ctx) error {
		p := strings.TrimPrefix(c.Path(), "/")

		// Root path — serve index.html
		if p == "" || p == "/" {
			return serveFile(c, fsys, "index.html")
		}

		// Try to serve the file directly
		if file, openErr := fsys.Open(p); openErr == nil {
			defer file.Close()
			if stat, statErr := file.Stat(); statErr == nil && !stat.IsDir() {
				return serveFile(c, fsys, p)
			}
		}

		// Not found — SPA fallback to index.html for client-side routing
		return serveFile(c, fsys, "index.html")
	}
}

// serveFile serves a file from [fs.FS] by path using Fiber's built-in SendFile.
func serveFile(c fiber.Ctx, fsys fs.FS, path string) error {
	// Read the entire file into memory for serving
	data, err := fs.ReadFile(fsys, path)
	if err != nil {
		return c.Status(http.StatusNotFound).SendString("Not Found")
	}
	// Let Fiber handle content-type detection based on the path
	c.Set("Content-Type", getContentType(path))
	return c.Send(data)
}

// getContentType returns the MIME type for a file based on its extension.
func getContentType(path string) string {
	switch {
	case strings.HasSuffix(path, ".html"):
		return "text/html; charset=utf-8"
	case strings.HasSuffix(path, ".css"):
		return "text/css; charset=utf-8"
	case strings.HasSuffix(path, ".js"):
		return "application/javascript"
	case strings.HasSuffix(path, ".json"):
		return "application/json"
	case strings.HasSuffix(path, ".svg"):
		return "image/svg+xml"
	case strings.HasSuffix(path, ".png"):
		return "image/png"
	case strings.HasSuffix(path, ".jpg") || strings.HasSuffix(path, ".jpeg"):
		return contentTypeImageJPEG
	case strings.HasSuffix(path, ".gif"):
		return "image/gif"
	case strings.HasSuffix(path, ".woff"):
		return "font/woff"
	case strings.HasSuffix(path, ".woff2"):
		return "font/woff2"
	case strings.HasSuffix(path, ".ttf"):
		return "font/ttf"
	case strings.HasSuffix(path, ".eot"):
		return "application/vnd.ms-fontobject"
	default:
		return contentTypeOctetStream
	}
}

// newViteProxy proxies all requests to the Vite dev server on localhost:5173.
func newViteProxy() fiber.Handler {
	target, err := url.Parse("http://localhost:5173")
	if err != nil {
		panic(err)
	}

	return func(c fiber.Ctx) error {
		// We need to convert fasthttp to net/http for the proxy
		// Fiber provides a built-in adapter for this
		// Instead, we'll use a simpler approach: forward the request using fasthttp directly
		req := &fasthttp.Request{}
		c.Request().CopyTo(req)
		req.SetRequestURI(target.Scheme + "://" + target.Host + c.Path())
		if len(c.Request().Body()) > 0 {
			req.SetBody(c.Request().Body())
		}

		resp := &fasthttp.Response{}
		client := &fasthttp.Client{}
		if doErr := client.Do(req, resp); doErr != nil {
			return c.Status(http.StatusInternalServerError).SendString("Proxy error: " + doErr.Error())
		}

		// Copy response headers
		for key, value := range resp.Header.All() {
			c.Set(string(key), string(value))
		}

		// Send response body
		c.Status(resp.StatusCode())
		return c.Send(resp.Body())
	}
}
