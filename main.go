package main

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/gocolly/colly"
	"github.com/gocolly/colly/debug"
	"github.com/spf13/viper"
)

var logger log.Logger

func init() {

	w := log.NewSyncWriter(os.Stderr)
	logger = log.NewLogfmtLogger(w)
	logger = level.NewFilter(logger, level.AllowDebug())
	logger = log.With(logger, "ts", log.DefaultTimestamp, "caller", log.DefaultCaller)

	viper.SetDefault("domain", "example.com")
	viper.SetDefault("ua", "colly")
	viper.SetDefault("entry", "https://example.com/")
	viper.SetDefault("loginURL", "https://example.com/login")
	viper.SetDefault("form_username", "username")
	viper.SetDefault("username", "username")
	viper.SetDefault("form_password", "password")
	viper.SetDefault("password", "password")
	viper.SetDefault("maxdepth", 2)
	viper.SetDefault("config", "config")
	viper.SetDefault("useConfig", false)

	viper.SetEnvPrefix("scrp") // env SCRP_XXX
	viper.BindEnv("domain")
	viper.BindEnv("ua")
	viper.BindEnv("entry")
	viper.BindEnv("loginURL")
	viper.BindEnv("form_username")
	viper.BindEnv("username")
	viper.BindEnv("form_password")
	viper.BindEnv("password")
	viper.BindEnv("maxdepth")
	viper.BindEnv("config")
	viper.BindEnv("useConfig")

	fmt.Println(viper.Get("domain"))

	if viper.GetBool("useConfig") {
		viper.SetConfigName(viper.GetString("config"))
		viper.AddConfigPath(".")
		err := viper.ReadInConfig()
		if err != nil {
			level.Error(logger).Log("msg", "failed read config", "error", err)
			os.Exit(1)
		}
	}
}

type Link struct {
	From   *url.URL
	To     *url.URL
	Text   string
	Tag    string
	Method string
}

type Links map[Link]bool

func (ls *Links) String() (res string) {
	for _, l := range *ls {
		res = fmt.Sprintf("%s %s", res, l)
	}
	res = fmt.Sprintf("[%s]", res)
	return res
}

func main() {

	u, err := url.Parse(viper.GetString("entry"))
	if err != nil {
		level.Error(logger).Log("msg", "failed parse entry url", "error", err)
		os.Exit(1)
	}
	c := colly.NewCollector(
		colly.UserAgent(viper.GetString("ua")),
		colly.AllowedDomains(viper.GetString("domain"), u.Host),
		colly.AllowURLRevisit(),
		colly.Debugger(&debug.LogDebugger{}),
		colly.MaxDepth(viper.GetInt("maxdepth")),
	)

	loginData := map[string]string{
		viper.GetString("form_username"): viper.GetString("username"),
		viper.GetString("form_password"): viper.GetString("password"),
	}
	err = c.Post(viper.GetString("loginURL"), loginData)
	if err != nil {
		level.Error(logger).Log("msg", "failed login", "error", err)
	}

	var links Links

	c.OnRequest(func(r *colly.Request) {
		level.Debug(logger).Log("msg", "requesting...", "url", r.URL.String(), "method", r.Method)
		r.Ctx.Put("url", r.URL.String())
	})

	c.OnHTML("a[href],form[action]", func(e *colly.HTMLElement) {
		link, err := e2Link(e)
		if err != nil {
			level.Error(logger).Log("msg", "failed to create link", "error", err)
		}
		logLink(level.Info(logger), "found link", link)
		if exists := links[*link]; !exists {
			links[*link] = true
			level.Debug(logger).Log("msg", "added link", "links", fmt.Sprintf("%v", links))
			if link.Method == http.MethodPost {
				param := make(map[string]string)
				e.ForEach("input", func(_ int, ce *colly.HTMLElement) {
					if ce.Attr("type") != "submit" {
						param[ce.Attr("name")] = ce.Attr("value")
					}
				})
				level.Debug(logger).Log("msg", "post", "url", link.To.String(), "param", fmt.Sprintf("%v", param))
				c.Post(link.To.String(), param)
			} else {
				level.Debug(logger).Log("msg", "visit", "url", link.To.String())
				c.Visit(link.To.String())
			}
		} else {
			logLink(level.Debug(logger), "already exists in links", link)
		}
	})

	links = make(Links)
	c.Visit(viper.GetString("entry"))
}

func logLink(logger log.Logger, msg string, link *Link) {
	if msg == "" {
		msg = "link"
	}
	logger.Log("msg", msg, "from", link.From, "to", link.To, "text", link.Text, "tag", link.Tag, "method", link.Method)
}

func e2Link(e *colly.HTMLElement) (link *Link, err error) {
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
		e.ForEach("input", func(_ int, ce *colly.HTMLElement) {
		  if ce.Attr("type") == "submit" {
		  	text = ce.Attr("value")
		  }
		})
	}
	link = &Link{
		From:   from,
		To:     to,
		Text:   text,
		Tag:    e.Name,
		Method: method,
	}
	return link, nil
}
