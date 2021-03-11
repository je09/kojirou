package mangadex

import (
	"fmt"
	"net/http"

	"github.com/leotaku/kojirou/mangadex/api"
)

type Client struct {
	base api.Client
}

func NewClient() *Client {
	return &Client{
		base: *api.NewClient(),
	}
}

func (c *Client) WithHTTPClient(http http.Client) *Client {
	c.base.WithClient(http)
	return c
}

func (c *Client) FetchManga(mangaID int) (*Manga, error) {
	b, err := c.base.FetchBase(mangaID)
	if err != nil {
		return nil, fmt.Errorf("fetch manga: %w", err)
	}

	return &Manga{
		Info:    convertBase(b.Data),
		Volumes: make(map[Identifier]Volume),
	}, nil
}

func (c *Client) FetchChapters(mangaID int) (ChapterList, error) {
	ca, err := c.base.FetchChapters(mangaID)
	if err != nil {
		return nil, fmt.Errorf("fetch chapters: %w", err)
	}

	chapters := convertChapters(ca.Data)
	return chapters, nil
}

func (c *Client) FetchCovers(mangaID int) (PathList, error) {
	co, err := c.base.FetchCovers(mangaID)
	if err != nil {
		return nil, fmt.Errorf("fetch covers: %w", err)
	}

	covers := convertCovers(co.Data)
	return covers, nil
}

func (c *Client) FetchChapter(ci ChapterInfo) (PathList, error) {
	chap, err := c.base.FetchChapter(ci.ID)
	if err != nil {
		return nil, fmt.Errorf("fetch chapter: %w", err)
	}

	paths := convertChapter(chap.Data, ci.Identifier, ci.VolumeIdentifier)
	return paths, nil
}
