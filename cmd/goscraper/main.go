package main

import (
	"database/sql"
	"fmt"
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
	viper.SetDefault(gos.OptDBUSERNAME, "username")
	viper.SetDefault(gos.OptDBPASSWORD, "password")
	viper.SetDefault(gos.OptDBHOST, "localhost")
	viper.SetDefault(gos.OptDBPORT, "3306")
	viper.SetDefault(gos.OptDBDATABASE, "database")
	viper.SetDefault(gos.OptLINKSELECTOR, "a[href],form,[onclick]")
	viper.SetDefault(gos.OptISDOPOST, false)
	viper.SetDefault(gos.OptCHECKLOGIN, "loggedin")

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
	viper.BindEnv(gos.OptDBUSERNAME)
	viper.BindEnv(gos.OptDBPASSWORD)
	viper.BindEnv(gos.OptDBHOST)
	viper.BindEnv(gos.OptDBPORT)
	viper.BindEnv(gos.OptDBDATABASE)
	viper.BindEnv(gos.OptLINKSELECTOR)
	viper.BindEnv(gos.OptISDOPOST)
	viper.BindEnv(gos.OptCHECKLOGIN)

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

	linkScraper, err := gos.NewLinkScraper(
		&gos.Config{
			Collector: colly.NewCollector(opts...),
			Links:     make(gos.Links),
			Logger:    logger,
			LoginURL:  viper.GetString(gos.OptLOGINURL),
			LoginData: map[string]string{
				viper.GetString(gos.OptFORM_USERNAME): viper.GetString(gos.OptUSERNAME),
				viper.GetString(gos.OptFORM_PASSWORD): viper.GetString(gos.OptPASSWORD),
			},
			Entry:        viper.GetString(gos.OptENTRY),
			OutFile:      viper.GetString(gos.OptOUTFILE),
			OutType:      viper.GetString(gos.OptOUTTYPE),
			LinkSelector: viper.GetString(gos.OptLINKSELECTOR),
			IsDoPost:     viper.GetBool(gos.OptISDOPOST),
		},
	)
	if err != nil {
		level.Error(logger).Log("msg", "failed to construct LinkScraper", "error", err)
		os.Exit(1)
	}

	err = linkScraper.Scrape()
	if err != nil {
		level.Error(logger).Log("msg", "failed to scrape", "error", err)
		os.Exit(1)
	}

	links, err := gos.SummaryLink(linkScraper.Links)
	if err != nil {
		level.Error(logger).Log("msg", "failed to summary ", "error", err)
		os.Exit(1)
	}
	b, err := gos.Links2Json(links)
	fmt.Println(string(b))

	linkScraper.FlushURLs()
	fmt.Println(linkScraper.URLs)

	driver, err := gos.NewDriver()
	if err != nil {
		level.Error(logger).Log("msg", "failed to new Web driver", "error", err)
		os.Exit(1)
	}
	db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s:%s)/%s",
		viper.GetString(gos.OptDBUSERNAME),
		viper.GetString(gos.OptDBPASSWORD),
		viper.GetString(gos.OptDBHOST),
		viper.GetString(gos.OptDBPORT),
		viper.GetString(gos.OptDBDATABASE)))
	if err != nil {
		level.Error(logger).Log("msg", "failed to open db connection", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	browser, err := gos.NewBrowser(
		&gos.BrowserConfig{
			Driver: driver,
			Db:     db,
			Logger: logger,
			Links:  links,
		},
	)
	err = browser.Browse()
	if err != nil {
		level.Error(logger).Log("msg", "failed to browse", "error", err)
		os.Exit(1)
	}
}
