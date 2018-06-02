package goscraper

import (
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/go-kit/kit/log"
	"github.com/gocolly/colly"
)

const (
	OptSCRP          = "scrp"
	OptDOMAIN        = "domain"
	OptUA            = "ua"
	OptENTRY         = "entry"
	OptLOGINURL      = "loginURL"
	OptFORM_USERNAME = "form_username"
	OptUSERNAME      = "username"
	OptFORM_PASSWORD = "form_password"
	OptPASSWORD      = "password"
	OptMAXDEPTH      = "maxdepth"
	OptCONFIG        = "config"
	OptUSECONFIG     = "useConfig"
	OptCSVFILE       = "csvfile"
	OptDISURLFILTER  = "disurlfilter"
	OptURLFILTER     = "urlfilter"
)

var FormTypeBtn = map[string]bool{
	"submit": true,
	"image":  true,
	"reset":  true,
	"button": true,
}

type Link struct {
	From        *url.URL
	To          *url.URL
	AttrId      string
	AttrOnClick string
	Text        string
	Tag         string
	Method      string
	Selector    string
}

type Links map[Link]bool

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

func WriteLinks2Csv(links Links, w io.Writer) (err error) {
	cw := csv.NewWriter(w)
	cw.Write([]string{
		"no",
		"from",
		"to",
		"onclick",
		"method",
	})
	i := 0
	for k, _ := range links {
		i++
		if err := cw.Write([]string{
			fmt.Sprintf("%d", i),
			k.From.String(),
			k.To.String(),
			k.AttrOnClick,
			k.Method,
		}); err != nil {
			return fmt.Errorf("failed to write csv record:%v", err)
		}
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		return fmt.Errorf("failed to write csv:%v", err)
	}
	return nil
}

func Str2filters(str, sep string) (filters []*regexp.Regexp) {
	for _, filter := range strings.Split(str, sep) {
		filters = append(filters, regexp.MustCompile(filter))
	}
	return filters
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
