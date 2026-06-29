package okf

import "html/template"

type HTMLResult struct {
	Root    string   `json:"root"`
	Out     string   `json:"out"`
	Written []string `json:"written"`
}

type htmlPageData struct {
	Title string
	Path  string
	Body  template.HTML
}
