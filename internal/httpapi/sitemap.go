// sitemap.xml（OP-06）：仅收录匿名可见的 standard 文档。
package httpapi

import (
	"io"
	"net/http"

	"element-wiki/internal/model"
	"element-wiki/internal/permission"
)

type sitemapNode struct {
	id       string
	title    string
	children []sitemapNode
}

func (d *Deps) handleSitemap(w http.ResponseWriter, r *http.Request) {
	actor := d.actor(r)
	if !actor.Has(permission.DocRead) {
		writeErr(w, http.StatusForbidden, "permission denied")
		return
	}
	kids, err := d.Docs.ListChildrenForTree(r.Context(), actor, nil)
	if mapServiceErr(w, err) {
		return
	}
	ctx := r.Context()
	var conv func(list []*model.Document) []sitemapNode
	conv = func(list []*model.Document) []sitemapNode {
		out := make([]sitemapNode, 0, len(list))
		for _, m := range list {
			out = append(out, sitemapNode{id: m.ID, title: m.Title, children: conv(nil)})
			sub, serr := d.Docs.ListChildrenForTree(ctx, actor, &m.ID)
			if serr == nil {
				out[len(out)-1].children = conv(sub)
			}
		}
		return out
	}
	nodes := conv(kids)

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	io.WriteString(w, `<?xml version="1.0" encoding="UTF-8"?>`+"\n")
	io.WriteString(w, `<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">`+"\n")
	var walk func(ns []sitemapNode)
	walk = func(ns []sitemapNode) {
		for _, n := range ns {
			io.WriteString(w, "\t<url><loc>/docs/"+n.id+"</loc></url>\n")
			walk(n.children)
		}
	}
	walk(nodes)
	io.WriteString(w, "</urlset>\n")
}
