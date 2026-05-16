package handlers

import (
	"fmt"
	"net/http"
	"time"
)

func (a *App) RobotsTxt(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprint(w, `User-agent: *
Allow: /
Allow: /about
Allow: /contact
Allow: /generator
Allow: /pricing
Allow: /features
Disallow: /dashboard
Disallow: /api/
Disallow: /admin/
Disallow: /profile

Sitemap: https://invobill.in/sitemap.xml
`)
}

func (a *App) SitemapXML(w http.ResponseWriter, r *http.Request) {
	base := "https://invobill.in"
	now := time.Now().Format("2006-01-02")

	pages := []struct {
		loc        string
		changefreq string
		priority   string
	}{
		{"/", "weekly", "1.0"},
		{"/about", "monthly", "0.8"},
		{"/pricing", "weekly", "0.9"},
		{"/features", "monthly", "0.8"},
		{"/contact", "monthly", "0.6"},
		{"/generator", "monthly", "0.7"},
		{"/register", "monthly", "0.9"},
		{"/login", "monthly", "0.5"},
	}

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">`)
	for _, p := range pages {
		fmt.Fprintf(w, `
  <url>
    <loc>%s%s</loc>
    <lastmod>%s</lastmod>
    <changefreq>%s</changefreq>
    <priority>%s</priority>
  </url>`, base, p.loc, now, p.changefreq, p.priority)
	}
	fmt.Fprint(w, "\n</urlset>")
}
