package goscraper

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
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
	OptOUTTYPE       = "outtype"
	OptOUTPUTCSV     = "csv"
	OptOUTPUTJSON    = "json"
	OptOUTFILE       = "outfile"
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
	From        *url.URL `json:"from"`
	To          *url.URL `json:"to"`
	AttrId      string   `json:"attr_id"`
	AttrOnClick string   `json:"attr_onclick"`
	Text        string   `json:"text"`
	Tag         string   `json:"tag"`
	Method      string   `json:"method"`
	Selector    string   `json:"selector"`
}

type Links map[Link]bool

type LinkScraper struct {
	Collector *colly.Collector
	Links     Links
	Logger    log.Logger
	LoginURL  string
	LoginData map[string]string
	Entry     string
	OutFile   string
	OutType   string
}

type Config struct {
	Links     Links
	Logger    log.Logger
	Collector *colly.Collector
	LoginURL  string
	LoginData map[string]string
	Entry     string
	OutFile   string
	OutType   string
}

func NewLinkScraper(config *Config) (*LinkScraper, error) {

	var cfg *Config
	if config == nil {
		cfg = &Config{}
	} else {
		cfg = config
	}

	return &LinkScraper{
		Collector: func() *colly.Collector {
			if cfg.Collector == nil {
				return colly.NewCollector() // plain colly collector.
			}
			return cfg.Collector
		}(),
		Links: func() Links {
			if cfg.Links == nil {
				return make(Links)
			}
			return cfg.Links
		}(),
		Logger: func() log.Logger {
			if cfg.Logger == nil {
				w := log.NewSyncWriter(os.Stderr)
				logger := log.NewLogfmtLogger(w)
				return logger
			}
			return cfg.Logger
		}(),
		LoginURL: cfg.LoginURL,
		LoginData: func() map[string]string {
			if cfg.LoginData == nil {
				return make(map[string]string)
			}
			return cfg.LoginData
		}(),
		Entry: cfg.Entry,
		OutFile: func() string {
			if cfg.OutFile == "" {
				return "output"
			}
			return cfg.OutFile
		}(),
		OutType: func() string {
			if cfg.OutType == "" {
				return OptOUTPUTCSV
			}
			return cfg.OutType
		}(),
	}, nil
}

func DefaultLinkScraper() *LinkScraper {
	return &LinkScraper{}
}

func (ls *LinkScraper) Scrape() (err error) {

	ls.registHandler()
	ls.Login()
	ls.Collector.Visit(ls.Entry)

	err = ls.Output()
	if err != nil {
		level.Error(ls.Logger).Log("msg", "failed to output", "error", err)
		return err
	}
	return nil
}

func (ls *LinkScraper) registHandler() {
	ls.Collector.OnRequest(func(r *colly.Request) {
		level.Debug(ls.Logger).Log("msg", "requesting...", "url", r.URL.String(), "method", r.Method)
		r.Ctx.Put("url", r.URL.String())
	})

	ls.Collector.OnHTML("a[html],form,[onclick]", func(e *colly.HTMLElement) {
		link, err := E2Link(e)
		if err != nil {
			level.Error(ls.Logger).Log("msg", "failed to create link", "error", err)
		}
		link.Selector = "a[html],form,[onclick]"
		LogLink(level.Info(ls.Logger), "found link", link)
		if exists := ls.Links[*link]; !exists {
			ls.Links[*link] = true
			level.Debug(ls.Logger).Log("msg", "added link", "links", fmt.Sprintf("%v", ls.Links))
			if link.Method == http.MethodPost {
				param := make(map[string]string)
				e.ForEach("input", func(_ int, ce *colly.HTMLElement) {
					if !FormTypeBtn[ce.Attr("type")] {
						param[ce.Attr("name")] = ce.Attr("value")
					}
				})
				level.Debug(ls.Logger).Log("msg", "post", "url", link.To.String(), "param", fmt.Sprintf("%v", param))
				ls.Collector.Post(link.To.String(), param)
			} else {
				if !strings.HasPrefix(link.To.String(), "javascript:") {
					level.Debug(ls.Logger).Log("msg", "visit", "url", e.Request.AbsoluteURL(link.To.String()))
					ls.Collector.Visit(link.To.String())
				}
			}
		} else {
			LogLink(level.Debug(ls.Logger), "already exists in links", link)
		}
	})
}

func (ls *LinkScraper) Login() (err error) {
	err = ls.Collector.Post(ls.LoginURL, ls.LoginData)
	if err != nil {
		level.Error(ls.Logger).Log("msg", "failed login", "error", err)
		return err
	}
	return nil
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

func MakeOutFilename(outfile, outtype string) (filename string) {
	return fmt.Sprintf("%s_%s.%s", outfile, time.Now().Format("20060102150405"), outtype)
}

func (ls *LinkScraper) Output() (err error) {
	filename := MakeOutFilename(ls.OutFile, ls.OutType)
	f, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE, 0600)
	defer f.Close()
	if err != nil {
		return fmt.Errorf("failed to open output file:%s:%v", filename, err)
	}
	switch ls.OutFile {
	case OptOUTPUTCSV:
		err = WriteLinks2Csv(ls.Links, f)
		if err != nil {
			return fmt.Errorf("failed to write csv:%s:%v", f.Name(), err)
		}
		level.Info(ls.Logger).Log("msg", "write output", "filename", filename)
		return nil
	case OptOUTPUTJSON:
		b, err := Links2Json(ls.Links)
		if err != nil {
			return fmt.Errorf("failed to marshal:%v", err)
		}
		_, err = f.Write(b)
		if err != nil {
			return fmt.Errorf("failed to write json:%s:%v", f.Name(), err)
		}
		level.Info(ls.Logger).Log("msg", "write output", "filename", filename)
		return nil
	default:
		return fmt.Errorf("not supported type:%s", ls.OutType)
	}
	return nil
}

func Links2Json(links Links) (b []byte, err error) {
	v := []Link{}
	for link, _ := range links {
		v = append(v, link)
	}
	b, err = json.Marshal(v)
	return b, err
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
