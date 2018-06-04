package goscraper

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
)

var (
	links  Links
	ls     []*Link
	logger log.Logger
)

type testData struct {
	FromURL     string `json:"from_url"`
	ToURL       string `json:"to_url"`
	AttrOnClick string `json:"attr_on_click"`
}

func init() {

	path := filepath.Join("testdata", "goscraper_test.json")
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		fmt.Errorf("failed read testdata: %v", err)
	}

	tD := []testData{}
	err = json.Unmarshal(bytes, &tD)
	if err != nil {
		fmt.Errorf("failed unmarshal testdata: %v", err)
	}

	links = make(Links)
	for _, d := range tD {
		from, err := url.Parse(d.FromURL)
		if err != nil {
			fmt.Errorf("failed prepare from url:%v", err)
		}
		to, err := url.Parse(d.ToURL)
		if err != nil {
			fmt.Errorf("failed prepare to url:%v", err)
		}

		ls = append(ls, &Link{
			From:        *from,
			To:          *to,
			AttrOnClick: d.AttrOnClick,
		})
	}

	ls[3].From = ls[0].From
	ls[3].To = ls[0].To

	for _, l := range ls {
		links[*l] = true
	}

	w := log.NewSyncWriter(os.Stderr)
	logger = log.NewLogfmtLogger(w)
	logger = level.NewFilter(logger, level.AllowDebug())

}

func TestSummaryLink(t *testing.T) {
	expectedLinks := Links{
		*ls[0]: true,
		*ls[1]: true,
		*ls[2]: true,
		*ls[3]: true,
	}

	testLinks, err := SummaryLink(links)
	if err != nil {
		t.Errorf("error in SummaryLink:%v", err)
	}
	if !reflect.DeepEqual(expectedLinks, testLinks) {
		t.Errorf("links not matched:\nwant:%v,\nhave:%v\n", expectedLinks, testLinks)
	}
}

func TestBrowseLinks(t *testing.T) {

	l := *ls[0]
	l.Tag = "a"
	l.Text = "More information..."

	blinks := Links{
		l: true,
	}
	driver, err := NewDriver()
	if err != nil {
		t.Errorf("error in NewDriver:%v", err)
	}
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	b, err := NewBrowser(
		&BrowserConfig{
			Logger: logger,
		},
	)
	err = b.BrowseLinks(blinks, driver, db)
	if err != nil {
		t.Errorf("error in BrowseLinks:%v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestNewDriver(t *testing.T) {
	_, err := NewDriver()
	if err != nil {
		t.Errorf("error in NewDriver:%v", err)
	}
}

func TestAdd(t *testing.T) {
	testLinks := Links{
		*ls[0]: true,
	}
	expect := Links{
		*ls[0]: true,
		*ls[1]: true,
	}
	from, _ := url.Parse("http://example.com")
	to, _ := url.Parse("http://example.com")
	l := Link{
		From: *from,
		To:   *to,
	}
	if testLinks, ok := Add(testLinks, ls[0]); ok {
		t.Errorf("added same link: %v", testLinks)
	}
	if testLinks, ok := Add(testLinks, &l); ok {
		t.Errorf("added same from and to: %v", testLinks)
	}
	if testLinks, ok := Add(testLinks, ls[1]); !ok {
		t.Errorf("not added diff link: %v", testLinks)
	}
	if !reflect.DeepEqual(expect, testLinks) {
		t.Errorf("not matched,\nwant: %v,\nhave: %v", expect, testLinks)
	}
}

func TestUniqURL(t *testing.T) {
	expect := []*url.URL{
		&ls[0].From,
		&ls[0].To,
		&ls[1].To,
	}
	testLinks := Links{
		*ls[0]: true,
		*ls[1]: true,
	}
	testURLs := UniqURL(testLinks)
	if !reflect.DeepEqual(expect, testURLs) {
		t.Errorf("not matched,\nwant: %v,\nhave: %v", expect, testURLs)
	}
}
