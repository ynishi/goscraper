package goscraper

import (
	"fmt"
	"net/url"
	"os"
	"reflect"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
)

var (
	links          Links
	l1, l2, l3, l4 Link
	logger         log.Logger
)

func init() {
	from1, err := url.Parse("http://example.com")
	if err != nil {
		fmt.Errorf("failed prepare from1 url:%v", err)
	}
	to1, err := url.Parse("http://example.com?a1=v11")
	if err != nil {
		fmt.Errorf("failed prepare from1 url:%v", err)
	}
	l1 = Link{
		From:        from1,
		To:          to1,
		AttrOnClick: "",
	}
	from2, err := url.Parse("http://example.com")
	if err != nil {
		fmt.Errorf("failed prepare from1 url:%v", err)
	}
	to2, err := url.Parse("http://example.com?a1=v12")
	if err != nil {
		fmt.Errorf("failed prepare from1 url:%v", err)
	}
	l2 = Link{
		From:        from2,
		To:          to2,
		AttrOnClick: "",
	}
	from3, err := url.Parse("http://example.com/1")
	if err != nil {
		fmt.Errorf("failed prepare from1 url:%v", err)
	}
	to3, err := url.Parse("http://example.com?a1=v11")
	if err != nil {
		fmt.Errorf("failed prepare from1 url:%v", err)
	}
	l3 = Link{
		From:        from3,
		To:          to3,
		AttrOnClick: "",
	}
	l4 = Link{
		From:        from1,
		To:          to1,
		AttrOnClick: "javascript:void(0)",
	}

	links = Links{
		l1: true,
		l2: true,
		l3: true,
		l4: true,
	}

	w := log.NewSyncWriter(os.Stderr)
	logger = log.NewLogfmtLogger(w)
	logger = level.NewFilter(logger, level.AllowDebug())

}

func TestSummaryLink(t *testing.T) {
	expectedLinks := Links{
		l1: true,
		l3: true,
		l4: true,
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

	l := l1
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
