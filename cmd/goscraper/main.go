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
	gos "github.com/ynishi/goscraper"
)

var logger log.Logger

func init() {

	w := log.NewSyncWriter(os.Stderr)
	logger = log.NewLogfmtLogger(w)
	logger = level.NewFilter(logger, level.AllowDebug())
	logger = log.With(logger, "ts", log.DefaultTimestamp, "caller", log.DefaultCaller)

	viper.SetDefault("domain", "example.com")
	viper.SetDefault("ua", "goscraper")
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

	var links gos.Links

	c.OnRequest(func(r *colly.Request) {
		level.Debug(logger).Log("msg", "requesting...", "url", r.URL.String(), "method", r.Method)
		r.Ctx.Put("url", r.URL.String())
	})

	c.OnHTML("a[html],form,[onclick]", func(e *colly.HTMLElement) {
		link, err := gos.E2Link(e)
		if err != nil {
			level.Error(logger).Log("msg", "failed to create link", "error", err)
		}
		gos.LogLink(level.Info(logger), "found link", link)
		if exists := links[*link]; !exists {
			links[*link] = true
			level.Debug(logger).Log("msg", "added link", "links", fmt.Sprintf("%v", links))
			if link.Method == http.MethodPost {
				param := make(map[string]string)
				e.ForEach("input", func(_ int, ce *colly.HTMLElement) {
					if !gos.FormTypeBtn[ce.Attr("type")] {
						param[ce.Attr("name")] = ce.Attr("value")
					}
				})
				level.Debug(logger).Log("msg", "post", "url", link.To.String(), "param", fmt.Sprintf("%v", param))
				c.Post(link.To.String(), param)
			} else {
				if !strings.HasPrefix(link.To.String(), "javascript:") {
					level.Debug(logger).Log("msg", "visit", "url", e.Request.AbsoluteURL(link.To.String()))
					c.Visit(link.To.String())
				}
			}
		} else {
			gos.LogLink(level.Debug(logger), "already exists in links", link)
		}
	})

	links = make(gos.Links)
	c.Visit(viper.GetString("entry"))
}
