// Package bgg calls the BoardGameGeek XML API from the server. The application
// token never leaves this adapter.
package bgg

import (
	"context"
	"encoding/xml"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/miebyte/goutils/logging"
	"github.com/superwhys/one-more-round/internal/app/ports"
	"github.com/superwhys/one-more-round/internal/errcode"
)

const (
	defaultSearchURL = "https://boardgamegeek.com/xmlapi2/search"
	defaultThingURL  = "https://boardgamegeek.com/xmlapi2/thing"
	cacheTTL         = time.Hour
	maxHits          = 20
	maxCache         = 200
)

// Client searches and looks up board games. Results are cached so repeated
// queries do not each hit the remote API.
type Client struct {
	token     string
	http      *http.Client
	searchURL string
	thingURL  string
	mu        sync.Mutex
	search    map[string]cached[[]ports.ExternalGame]
	things    map[int]cached[ports.ExternalGame]
}

type cached[T any] struct {
	value   T
	expires time.Time
}

type itemsXML struct {
	Items []itemXML `xml:"item"`
}

type itemXML struct {
	ID        int       `xml:"id,attr"`
	Thumbnail string    `xml:"thumbnail"`
	Image     string    `xml:"image"`
	Names     []nameXML `xml:"name"`
	Year      *yearXML  `xml:"yearpublished"`
}

type nameXML struct {
	Type  string `xml:"type,attr"`
	Value string `xml:"value,attr"`
}

type yearXML struct {
	Value string `xml:"value,attr"`
}

// New builds a client for an approved application token.
func New(token string) *Client {
	return &Client{
		token:     strings.TrimSpace(token),
		http:      &http.Client{Timeout: 20 * time.Second},
		searchURL: defaultSearchURL,
		thingURL:  defaultThingURL,
		search:    map[string]cached[[]ports.ExternalGame]{},
		things:    map[int]cached[ports.ExternalGame]{},
	}
}

// SearchBoardGames returns matching base games, newest cache first.
func (c *Client) SearchBoardGames(ctx context.Context, query string) ([]ports.ExternalGame, error) {
	query = strings.TrimSpace(query)
	if query == "" || utf8.RuneCountInString(query) > 80 {
		return nil, errcode.ErrBGGQuery
	}
	key := strings.ToLower(query)
	if hits, ok := c.cachedSearch(key); ok {
		return hits, nil
	}
	body, err := c.get(ctx, c.searchURL, url.Values{"query": {query}, "type": {"boardgame"}})
	if err != nil {
		return nil, err
	}
	var doc itemsXML
	if err = xml.Unmarshal(body, &doc); err != nil {
		logging.Errorc(ctx, "bgg search response is not usable")
		return nil, errcode.ErrBGGSearch
	}
	hits := make([]ports.ExternalGame, 0, min(len(doc.Items), maxHits))
	for _, item := range doc.Items {
		if len(hits) == maxHits {
			break
		}
		name := primaryName(item.Names)
		if item.ID <= 0 || name == "" {
			continue
		}
		hits = append(hits, ports.ExternalGame{ID: item.ID, Name: name, Year: yearOf(item.Year)})
	}
	c.attachCovers(ctx, hits)
	c.storeSearch(key, hits)
	return hits, nil
}

// LookupBoardGame reads the canonical name of one board game.
func (c *Client) LookupBoardGame(ctx context.Context, id int) (ports.ExternalGame, error) {
	if id <= 0 {
		return ports.ExternalGame{}, errcode.ErrGameName
	}
	if game, ok := c.cachedThing(id); ok {
		return game, nil
	}
	body, err := c.get(ctx, c.thingURL, url.Values{"id": {strconv.Itoa(id)}, "type": {"boardgame"}})
	if err != nil {
		return ports.ExternalGame{}, err
	}
	var doc itemsXML
	if err = xml.Unmarshal(body, &doc); err != nil || len(doc.Items) == 0 {
		logging.Errorc(ctx, "bgg thing %d is not usable", id)
		return ports.ExternalGame{}, errcode.ErrNotFound.WithMessage("没有找到这款桌游，可以手动添加")
	}
	item := doc.Items[0]
	name := primaryName(item.Names)
	if item.ID != id || name == "" {
		return ports.ExternalGame{}, errcode.ErrNotFound.WithMessage("没有找到这款桌游，可以手动添加")
	}
	game := ports.ExternalGame{ID: item.ID, Name: name, Year: yearOf(item.Year), Thumbnail: coverURL(item)}
	c.storeThing(game)
	return game, nil
}

// get performs one authorized request and reads a bounded XML body.
func (c *Client) get(ctx context.Context, endpoint string, query url.Values) ([]byte, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, errcode.ErrBGGSearch
	}
	u.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, errcode.ErrBGGSearch
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/xml")
	req.Header.Set("User-Agent", "one-more-round")
	resp, err := c.http.Do(req)
	if err != nil {
		if errors.Is(err, context.Canceled) && ctx.Err() != nil {
			return nil, err
		}
		logging.Errorc(ctx, "bgg request failed")
		return nil, errcode.ErrBGGSearch
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, errcode.ErrBGGSearch
	}
	switch resp.StatusCode {
	case http.StatusOK:
		return body, nil
	case http.StatusUnauthorized, http.StatusForbidden:
		logging.Errorc(ctx, "bgg rejected the application token: %d", resp.StatusCode)
		return nil, errcode.ErrBGGUnavailable.WithMessage("BGG 授权无效，请使用本组桌游或手动添加")
	case http.StatusTooManyRequests:
		logging.Errorc(ctx, "bgg rate limited the search")
		return nil, errcode.ErrBGGSearch
	default:
		logging.Errorc(ctx, "bgg returned status %d", resp.StatusCode)
		return nil, errcode.ErrBGGSearch
	}
}

// attachCovers fills thumbnails from one thing request. A cover failure leaves
// the names in place so search still succeeds.
func (c *Client) attachCovers(ctx context.Context, hits []ports.ExternalGame) {
	if len(hits) == 0 {
		return
	}
	ids := make([]string, len(hits))
	for i, hit := range hits {
		ids[i] = strconv.Itoa(hit.ID)
	}
	body, err := c.get(ctx, c.thingURL, url.Values{"id": {strings.Join(ids, ",")}, "type": {"boardgame"}})
	if err != nil {
		logging.Errorc(ctx, "bgg covers unavailable")
		return
	}
	var doc itemsXML
	if xml.Unmarshal(body, &doc) != nil {
		logging.Errorc(ctx, "bgg cover response is not usable")
		return
	}
	covers := make(map[int]string, len(doc.Items))
	for _, item := range doc.Items {
		if cover := coverURL(item); cover != "" {
			covers[item.ID] = cover
		}
	}
	for i := range hits {
		hits[i].Thumbnail = covers[hits[i].ID]
	}
}

// coverURL returns the thumbnail, or the full image when no thumbnail exists.
// Only https links are accepted.
func coverURL(item itemXML) string {
	for _, raw := range []string{item.Thumbnail, item.Image} {
		value := strings.TrimSpace(raw)
		if strings.HasPrefix(value, "https://") {
			return value
		}
	}
	return ""
}

func primaryName(names []nameXML) string {
	var fallback string
	for _, name := range names {
		value := strings.TrimSpace(name.Value)
		if value == "" {
			continue
		}
		if name.Type == "primary" || name.Type == "" {
			return value
		}
		if fallback == "" {
			fallback = value
		}
	}
	return fallback
}

func yearOf(year *yearXML) *int {
	if year == nil {
		return nil
	}
	value, err := strconv.Atoi(strings.TrimSpace(year.Value))
	if err != nil {
		return nil
	}
	return &value
}

func (c *Client) cachedSearch(key string) ([]ports.ExternalGame, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	hit, ok := c.search[key]
	if !ok || time.Now().After(hit.expires) {
		return nil, false
	}
	return append([]ports.ExternalGame(nil), hit.value...), true
}

func (c *Client) storeSearch(key string, hits []ports.ExternalGame) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.search) >= maxCache {
		clear(c.search)
	}
	c.search[key] = cached[[]ports.ExternalGame]{value: hits, expires: time.Now().Add(cacheTTL)}
}

func (c *Client) cachedThing(id int) (ports.ExternalGame, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	hit, ok := c.things[id]
	if !ok || time.Now().After(hit.expires) {
		return ports.ExternalGame{}, false
	}
	return hit.value, true
}

func (c *Client) storeThing(game ports.ExternalGame) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.things) >= maxCache {
		clear(c.things)
	}
	c.things[game.ID] = cached[ports.ExternalGame]{value: game, expires: time.Now().Add(cacheTTL)}
}
