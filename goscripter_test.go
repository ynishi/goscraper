package goscraper

import (
	"net/url"
	"reflect"
	"testing"
)

func TestSummaryLink(t *testing.T) {
	from1, err := url.Parse("http://example.com")
	if err != nil {
		t.Errorf("failed prepare from1 url:%v", err)
	}
	to1, err := url.Parse("http://example.com?a1=v11")
	if err != nil {
		t.Errorf("failed prepare from1 url:%v", err)
	}
	l1 := Link{
		From: from1,
		To:   to1,
	}
	from2, err := url.Parse("http://example.com")
	if err != nil {
		t.Errorf("failed prepare from1 url:%v", err)
	}
	to2, err := url.Parse("http://example.com?a1=v12")
	if err != nil {
		t.Errorf("failed prepare from1 url:%v", err)
	}
	l2 := Link{
		From: from2,
		To:   to2,
	}
	from3, err := url.Parse("http://example.com/1")
	if err != nil {
		t.Errorf("failed prepare from1 url:%v", err)
	}
	to3, err := url.Parse("http://example.com?a1=v11")
	if err != nil {
		t.Errorf("failed prepare from1 url:%v", err)
	}
	l3 := Link{
		From: from3,
		To:   to3,
	}

	var links = Links{
		l1: true,
		l2: true,
		l3: true,
	}

	var expectedLinks = Links{
		l1: true,
		l3: true,
	}

	testLinks, err := SummaryLink(links)
	if err != nil {
		t.Errorf("error in SummaryLink:%v", err)
	}
	if !reflect.DeepEqual(expectedLinks, testLinks) {
		t.Errorf("links not matched:\nwant:%v,\nhave:%v\n", expectedLinks, testLinks)
	}
}
