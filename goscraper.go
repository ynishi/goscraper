package goscraper

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/sclevine/agouti"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/gocolly/colly"

	_ "github.com/go-sql-driver/mysql"
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
	OptDBUSERNAME    = "dbusername"
	OptDBPASSWORD    = "dbpassword"
	OptDBDATABASE    = "dbdatabase"
	OptDBHOST        = "dbhost"
	OptDBPORT        = "dbport"
	OptLINKSELECTOR  = "linkselector"
	OptISDOPOST      = "isdopost"
	OptCHECKLOGIN    = "checklogin"
)

var FormTypeBtn = map[string]bool{
	"submit": true,
	"image":  true,
	"reset":  true,
	"button": true,
}

type Link struct {
	From        url.URL `json:"from"`
	To          url.URL `json:"to"`
	AttrId      string  `json:"attr_id"`
	AttrOnClick string  `json:"attr_onclick"`
	Text        string  `json:"text"`
	Tag         string  `json:"tag"`
	Method      string  `json:"method"`
	Selector    string  `json:"selector"`
}

type Links map[Link]bool

type LinkScraper struct {
	Collector    *colly.Collector
	Links        Links
	Logger       log.Logger
	LoginURL     string
	LoginData    map[string]string
	Entry        string
	OutFile      string
	OutType      string
	LinkSelector string
	IsDoPost     bool
	CheckLogin   string
	URLs         []*url.URL
}

type Config struct {
	Links        Links
	Logger       log.Logger
	Collector    *colly.Collector
	LoginURL     string
	LoginData    map[string]string
	Entry        string
	OutFile      string
	OutType      string
	LinkSelector string
	IsDoPost     bool
	CheckLogin   string
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
		LinkSelector: func() string {
			if cfg.LinkSelector == "" {
				return "a[href],form,[onclick]"
			}
			return cfg.LinkSelector
		}(),
		IsDoPost: cfg.IsDoPost,
		CheckLogin: func() string {
			if cfg.CheckLogin == "" {
				return "loggedin"
			}
			return cfg.CheckLogin
		}(),
		URLs: make([]*url.URL, 0),
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

	ls.Collector.OnHTML(ls.LinkSelector, func(e *colly.HTMLElement) {
		link, err := E2Link(e)
		if err != nil {
			level.Error(ls.Logger).Log("msg", "failed to create link", "error", err)
		}
		link.Selector = ls.LinkSelector
		LogLink(level.Error(ls.Logger), "found link", link)
		if _, ok := Add(ls.Links, link); ok {
			level.Debug(ls.Logger).Log("msg", "added link", "link", link)
			if ls.IsDoPost && link.Method == http.MethodPost {
				param := make(map[string]string)
				e.ForEach("input", func(_ int, ce *colly.HTMLElement) {
					if !FormTypeBtn[ce.Attr("type")] {
						param[ce.Attr("name")] = ce.Attr("value")
					}
				})
				level.Debug(ls.Logger).Log("msg", "post", "url", link.To.String(), "param", fmt.Sprintf("%v", param))
				if !ls.IsLogin(e) {
					ls.LoginE(e)
				}
				e.Request.Post(link.To.String(), param)
				return
			}
			if !strings.HasPrefix(strings.TrimSpace(link.To.String()), "javascript:") {
				level.Debug(ls.Logger).Log("msg", "visit", "url", e.Request.AbsoluteURL(link.To.String()))
				if !ls.IsLogin(e) {
					ls.LoginE(e)
				}
				e.Request.Visit(link.To.String())
				return
			}
			LogLink(level.Debug(ls.Logger), "not visited link", link)
			return
		} else {
			LogLink(level.Debug(ls.Logger), "already exists in links", link)
			return
		}
	})
}

func Add(links Links, link *Link) (res Links, ok bool) {
	switch {
	case link.From.String() == link.To.String():
		return links, false
	case links[*link]:
		return links, false
	default:
		links[*link] = true
		return links, true
	}
}

func (ls *LinkScraper) Login() (err error) {

	err = ls.Collector.Post(ls.LoginURL, ls.LoginData)
	if err != nil {
		level.Error(ls.Logger).Log("msg", "failed login", "error", err)
		return err
	}
	return nil
}

func (ls *LinkScraper) LoginE(e *colly.HTMLElement) (err error) {
	e.Request.Post(ls.LoginURL, ls.LoginData)
	return nil
}

func (ls *LinkScraper) IsLogin(e *colly.HTMLElement) bool {
	if strings.Index(string(e.Response.Body), ls.CheckLogin) > -1 {
		return true
	}
	return false
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
		From:        *from,
		To:          *to,
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
	switch ls.OutType {
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

func (ls *LinkScraper) FlushURLs() []*url.URL {
	ls.URLs = UniqURL(ls.Links)
	return ls.URLs
}

func UniqURL(links Links) (urls []*url.URL) {
	us := []url.URL{}
	for l, _ := range links {
		foundf := false
		foundt := false
		for _, u := range us {
			if isSimilerURL(&l.From, &u) {
				foundf = true
			}
			if isSimilerURL(&l.To, &u) {
				foundt = true
			}
		}
		if !foundf {
			us = append(us, l.From)
		}
		if !foundt {
			us = append(us, l.To)
		}
	}
	for i, _ := range us {
		urls = append(urls, &us[i])
	}
	return urls
}

func SummaryLink(links Links) (res Links, err error) {
	res = make(Links)
	for l, _ := range links {
		res, _ = addNotSimiler(res, l)
	}
	return res, nil
}

func addNotSimiler(links Links, link Link) (resLinks Links, isAdded bool) {
	found := false
	for l, _ := range links {
		if isSimilerURL(&link.From, &l.From) && isSimilerURL(&link.To, &l.To) && link.AttrOnClick == l.AttrOnClick {
			found = true
			break
		}
	}
	if found {
		return links, false
	} else {
		links[link] = true
		return links, true
	}
}

func isSimilerURL(u1, u2 *url.URL) (same bool) {
	if u1.Host == u2.Host && u1.Path == u2.Path {
		q1 := u1.Query()
		q2 := u2.Query()

		if len(q1) != len(q2) {
			return false
		}

		same = true
		for k, _ := range q1 {
			if _, b := q2[k]; !b {
				same = false
				break
			}
		}
		return same
	} else {
		return false
	}
}

func SummaryURL(u1, u2 *url.URL) (map[url.URL]bool, error) {
	// TODO: impl
	us := map[url.URL]bool{*u1: true}
	return us, nil
}

type Browser struct {
	Driver *agouti.WebDriver
	Db     *sql.DB
	Logger log.Logger
	Links  Links
}

type BrowserConfig struct {
	Driver *agouti.WebDriver
	Db     *sql.DB
	Logger log.Logger
	Links  Links
}

func NewBrowser(config *BrowserConfig) (*Browser, error) {
	rand.Seed(time.Now().UnixNano())

	var cfg *BrowserConfig
	if config == nil {
		cfg = &BrowserConfig{}
	} else {
		cfg = config
	}
	return &Browser{
		Driver: cfg.Driver,
		Logger: cfg.Logger,
		Db:     cfg.Db,
		Links:  cfg.Links,
	}, nil
}

func (b *Browser) Browse() error {
	err := b.BrowseLinks(b.Links, b.Driver, b.Db)
	if err != nil {
		return err
	}
	return nil
}

func (b *Browser) BrowseLinks(links Links, driver *agouti.WebDriver, db *sql.DB) (err error) {

	if err != nil {
		return fmt.Errorf("Failed to start driver:%v", err)
	}

	if err := driver.Start(); err != nil {
		return fmt.Errorf("Failed to start driver:%v", err)
	}
	defer driver.Stop()

	for link, _ := range links {
		var id *string
		if id, err = BrowseLink(link, driver, db); err != nil {
			return fmt.Errorf("Failed to browse link:%v:%v", link, err)
		}
		level.Info(b.Logger).Log("msg", "browsed link", "id", id, "from", link.From.String(), "to", link.To.String())
	}
	return nil
}

func BrowseLink(link Link, driver *agouti.WebDriver, db *sql.DB) (id *string, err error) {

	bid := makeBrowseId()

	page, err := driver.NewPage(agouti.Browser("chrome"))
	if err != nil {
		return nil, fmt.Errorf("Failed new page:%v", err)
	}

	if err := page.Navigate(link.From.String()); err != nil {
		return nil, fmt.Errorf("Failed to navigate:%v", err)
	}

	startQuery := fmt.Sprintf("SELECT 1 FROM DUAL -- start browse: %s", bid)
	db.QueryRow(startQuery)

	err = Link2Click(link, page.Find("body")).Click()
	if err != nil {
		return nil, fmt.Errorf("Failed to click:%v", err)
	}

	endQuery := fmt.Sprintf("SELECT 1 FROM DUAL -- end browse: %s", bid)
	db.QueryRow(endQuery)

	err = page.Screenshot(fmt.Sprintf("%s.png", bid))
	if err != nil {
		return nil, fmt.Errorf("Failed to save snapshot:%v", err)
	}

	html, err := page.HTML()
	if err != nil {
		return nil, fmt.Errorf("Failed to open page:%v", err)
	}
	err = ioutil.WriteFile(fmt.Sprintf("%s.html", bid), []byte(html), 0644)
	if err != nil {
		return nil, fmt.Errorf("Failed to save html:%v", err)
	}
	page.CloseWindow()
	saveGeneralLog(bid, db)

	return &bid, nil
}

func Link2Click(link Link, selection *agouti.Selection) *agouti.Selection {
	if link.AttrId != "" {
		return selection.FindByID(link.AttrId)
	}
	if link.Tag == "a" {
		return selection.FirstByLink(link.Text)
	}
	if link.Tag == "form" {
		return selection.FirstByButton(link.Text)
	}
	return selection.FindByName(link.Text)
}

func saveGeneralLog(bid string, db *sql.DB) (err error) {
	genQuery := fmt.Sprintf(`
      SELECT
        event_time,
        user_host,
        argument
      FROM
        mysql.general_log
      where
        event_time > '%s/%s/%s %s:%s'
        and argument like '%%%s%%'`, bid[0:4], bid[4:6], bid[6:8], bid[8:10], bid[10:12], bid)
	row := db.QueryRow(genQuery)
	var gen GeneralLog
	row.Scan((&gen.Event_time), (&gen.User_host), (&gen.Argument))

	err = ioutil.WriteFile(fmt.Sprintf("%s.sql_log", bid), []byte(fmt.Sprintf("%v", gen)), 0644)
	if err != nil {
		return fmt.Errorf("Failed to save sql_log:%v", err)
	}
	return nil
}

type GeneralLog struct {
	Event_time string
	User_host  string
	Argument   string
}

var rLetters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func makeBrowseId() string {
	return fmt.Sprintf("%s%s", time.Now().Format("20060102150405"), string(rLetters[rand.Intn(len(rLetters))]))
}

func NewDriver() (*agouti.WebDriver, error) {
	return agouti.ChromeDriver(
		agouti.ChromeOptions("args", []string{
			//"--headless",
			"--window-size=1280,800",
		}),
		agouti.Debug,
	), nil
}
