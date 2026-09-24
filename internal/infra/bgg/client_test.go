package bgg

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/superwhys/one-more-round/internal/errcode"
)

func TestSearchAndLookup(t *testing.T) {
	var searches int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("auth = %q", r.Header.Get("Authorization"))
		}
		if strings.HasSuffix(r.URL.Path, "/search") {
			searches++
			_, _ = w.Write(
				[]byte(
					`<items total="2"><item type="boardgame" id="13"><name type="primary" value="Catan"/><yearpublished value="1995"/></item><item type="boardgame" id="14"><name type="alternate" value="Only Alias"/></item></items>`,
				),
			)
			return
		}
		_, _ = w.Write(
			[]byte(
				`<items><item type="boardgame" id="13"><thumbnail>https://cf.geekdo-images.com/catan.jpg</thumbnail><name type="alternate" value="卡坦岛"/><name type="primary" value="Catan"/><yearpublished value="1995"/></item><item type="boardgame" id="14"><image>http://insecure.example/x.jpg</image><name type="primary" value="Only Alias"/></item></items>`,
			),
		)
	}))
	defer srv.Close()
	client := New("test-token")
	client.searchURL = srv.URL + "/search"
	client.thingURL = srv.URL + "/thing"

	hits, err := client.SearchBoardGames(context.Background(), "Catan")
	if err != nil || len(hits) != 2 || hits[0].Name != "Catan" || hits[0].Year == nil ||
		*hits[0].Year != 1995 ||
		hits[0].Thumbnail != "https://cf.geekdo-images.com/catan.jpg" ||
		hits[1].Name != "Only Alias" ||
		hits[1].Thumbnail != "" {
		t.Fatalf("hits=%v err=%v", hits, err)
	}
	if _, err = client.SearchBoardGames(
		context.Background(),
		"catan",
	); err != nil ||
		searches != 1 {
		t.Fatalf("cache missed: calls=%d err=%v", searches, err)
	}
	game, err := client.LookupBoardGame(context.Background(), 13)
	if err != nil || game.Name != "Catan" || game.ID != 13 {
		t.Fatalf("lookup=%v err=%v", game, err)
	}
}

func TestSearchFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()
	client := New("test-token")
	client.searchURL = srv.URL
	_, err := client.SearchBoardGames(context.Background(), "Catan")
	if !errcode.ErrBGGSearch.Is(err) {
		t.Fatal(err)
	}
}
