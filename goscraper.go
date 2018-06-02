package goscraper

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-kit/kit/log"

	"github.com/gocolly/colly"
)

type Link struct {
	From        *url.URL
	To          *url.URL
	AttrId      string
	AttrOnClick string
	Text        string
	Tag         string
	Method      string
}

type Links map[Link]bool

var FormTypeBtn = map[string]bool{
	"submit": true,
	"image":  true,
	"reset":  true,
	"button": true,
}

func E2Link(e *colly.HTMLElement) (link *Link, err error) {
	from, err := url.Parse(e.Request.AbsoluteURL(e.Request.Ctx.Get("url")))
	if err != nil {
		return nil, fmt.Errorf("invalid link from:%s:%v", e.Request.Ctx.Get("url"), err)
	}
	var rawTo string
	switch {
	case e.Attr("action") != "":
		rawTo = e.Attr("action")
	case e.Attr("href") != "":
		rawTo = e.Attr("href")
	}
	to, err := url.Parse(e.Request.AbsoluteURL(rawTo))
	if err != nil {
		return nil, fmt.Errorf("invalid link to:%s:%v", rawTo, err)
	}
	var method string
	if e.Attr("method") != "" {
		method = strings.ToUpper(e.Attr("method"))
	} else {
		method = http.MethodGet
	}
	var text string
	text = e.Text
	if e.Name == "form" {
		if e.Attr("name") != "" {
			text = e.Attr("name")
		} else {
			e.ForEach("input[type],button", func(_ int, ce *colly.HTMLElement) {
				if ce.Name == "button" || FormTypeBtn[ce.Attr("type")] {
					switch {
					case ce.Attr("value") != "":
						text = ce.Attr("value")
					case ce.Attr("alt") != "":
						text = ce.Attr("alt")
					case ce.Attr("name") != "":
						text = ce.Attr("name")
					}
				}
			})
		}
	}

	link = &Link{
		From:        from,
		To:          to,
		AttrId:      e.Attr("id"),
		AttrOnClick: e.Attr("onclick"),
		Text:        text,
		Tag:         e.Name,
		Method:      method,
	}
	return link, nil
}

func LogLink(logger log.Logger, msg string, link *Link) {
	if msg == "" {
		msg = "link"
	}
	logger.Log(
		"msg", msg,
		"from", link.From,
		"to", link.To,
		"attrid", link.AttrId,
		"attronclick", link.AttrOnClick,
		"text", link.Text,
		"tag", link.Tag,
		"method", link.Method,
)
}
