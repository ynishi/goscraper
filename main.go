package main

import (
	"fmt"
	"net/url"
	"os"

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
	From *url.URL
	To   *url.URL
	Text string
}

type Links []Link

func (ls *Links) Add(link *Link) {
	var found bool
	for _, l := range *ls {
		if l == *link {
			found = true
			break
		}
	}
	if !found {
		*ls = append(*ls, *link)
	}
	return
}

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
	c.Post(viper.GetString("loginURL"), loginData)

	var links *Links

	c.OnRequest(func(r *colly.Request) {
		r.Ctx.Put("url", r.URL.String())
	})

	c.OnHTML("a[href]", func(e *colly.HTMLElement) {
		from, err := url.Parse(e.Request.AbsoluteURL(e.Request.Ctx.Get("url")))
		if err != nil {
			level.Error(logger).Log("msg", "invalid link from", "error", err, "url", e.Request.Ctx.Get("url"))
		}
		to, err := url.Parse(e.Request.AbsoluteURL(e.Attr("href")))
		if err != nil {
			level.Error(logger).Log("msg", "invalid link to", "error", err, "url", e.Attr("href"))
		}
		link := &Link{
			From: from,
			To:   to,
			Text: e.Text,
		}
		level.Info(logger).Log("msg", "found link", "from", link.From, "to", link.To, "text", link.Text)
		links.Add(link)
		level.Debug(logger).Log("msg", "added link", "links", links.String())
		c.Visit(link.To.String())
	})

	links = &Links{}
	c.Visit(viper.GetString("entry"))
}
