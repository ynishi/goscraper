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

	viper.SetDefault(gos.OptDOMAIN, "example.com")
	viper.SetDefault(gos.OptUA, "goscraper")
	viper.SetDefault(gos.OptENTRY, "https://example.com/")
	viper.SetDefault(gos.OptLOGINURL, "https://example.com/login")
	viper.SetDefault(gos.OptFORM_USERNAME, "username")
	viper.SetDefault(gos.OptUSERNAME, "username")
	viper.SetDefault(gos.OptFORM_PASSWORD, "password")
	viper.SetDefault(gos.OptPASSWORD, "password")
	viper.SetDefault(gos.OptMAXDEPTH, 2)
	viper.SetDefault(gos.OptCONFIG, "config")
	viper.SetDefault(gos.OptUSECONFIG, false)
	viper.SetDefault(gos.OptOUTFILE, "output")
	viper.SetDefault(gos.OptOUTTYPE, gos.OptOUTPUTCSV)

	viper.SetEnvPrefix(gos.OptSCRP) // env SCRP_XXX
	viper.BindEnv(gos.OptDOMAIN)    // comma separated list, no use colly default env
	viper.BindEnv(gos.OptUA)        // no use colly default env
	viper.BindEnv(gos.OptENTRY)
	viper.BindEnv(gos.OptLOGINURL)
	viper.BindEnv(gos.OptFORM_USERNAME)
	viper.BindEnv(gos.OptUSERNAME)
	viper.BindEnv(gos.OptFORM_PASSWORD)
	viper.BindEnv(gos.OptPASSWORD)
	viper.BindEnv(gos.OptMAXDEPTH) // no use colly default env
	viper.BindEnv(gos.OptCONFIG)
	viper.BindEnv(gos.OptUSECONFIG)
	viper.BindEnv(gos.OptOUTFILE)
	viper.BindEnv(gos.OptOUTTYPE)
	viper.BindEnv(gos.OptURLFILTER)    // comma separated list
	viper.BindEnv(gos.OptDISURLFILTER) //comma separated list

	if viper.GetBool(gos.OptUSECONFIG) {
		viper.SetConfigName(viper.GetString(gos.OptCONFIG))
		viper.AddConfigPath(".")
		err := viper.ReadInConfig()
		if err != nil {
			level.Error(logger).Log("msg", "failed read config", "error", err)
			os.Exit(1)
		}
	}
}

func main() {

	u, err := url.Parse(viper.GetString(gos.OptENTRY))
	if err != nil {
		level.Error(logger).Log("msg", "failed parse entry url", "error", err)
		os.Exit(1)
	}

	opts := []func(*colly.Collector){
		colly.UserAgent(viper.GetString(gos.OptUA)),
		colly.AllowedDomains(append(strings.Split(viper.GetString(gos.OptDOMAIN), ","), u.Host)...),
		colly.AllowURLRevisit(),
		colly.Debugger(&debug.LogDebugger{}),
		colly.MaxDepth(viper.GetInt(gos.OptMAXDEPTH)),
	}

	if viper.GetString(gos.OptURLFILTER) != "" {
		opts = append(opts, colly.URLFilters(gos.Str2filters(viper.GetString(gos.OptURLFILTER), ",")...))
	}

	if viper.GetString(gos.OptDISURLFILTER) != "" {
		opts = append(opts, colly.DisallowedURLFilters(gos.Str2filters(viper.GetString(gos.OptDISURLFILTER), ",")...))
	}

	c := colly.NewCollector(opts...)

	loginData := map[string]string{
		viper.GetString(gos.OptFORM_USERNAME): viper.GetString(gos.OptUSERNAME),
		viper.GetString(gos.OptFORM_PASSWORD): viper.GetString(gos.OptPASSWORD),
	}
	err = c.Post(viper.GetString(gos.OptLOGINURL), loginData)
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
		link.Selector = "a[html],form,[onclick]"
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
	c.Visit(viper.GetString(gos.OptENTRY))
	filename, err := gos.Output(links, viper.GetString(gos.OptOUTFILE), viper.GetString(gos.OptOUTTYPE))
	if err != nil {
		level.Error(logger).Log("msg", "failed to output", "error", err)
		os.Exit(1)
	}
	level.Info(logger).Log("msg", "write output", "filename", filename)
}
