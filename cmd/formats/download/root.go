package download

import (
	"context"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"time"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/leotaku/kojirou/cmd/formats"
	md "github.com/leotaku/kojirou/mangadex"
	"golang.org/x/sync/errgroup"
)

const (
	maxJobsChapter = 8
	maxJobsImage   = 16
)

var (
	httpClient     *http.Client
	mangadexClient *md.Client
)

func init() {
	retry := retryablehttp.NewClient()
	retry.Logger = nil
	retry.RetryWaitMin = time.Second * 5
	retry.Backoff = retryablehttp.LinearJitterBackoff
	httpClient = retry.StandardClient()
	mangadexClient = md.NewClient().WithHTTPClient(httpClient)
}

func MangadexSkeleton(mangaID string) (*md.Manga, error) {
	return mangadexClient.FetchManga(context.TODO(), mangaID)
}

func MangadexChapters(mangaID string) (md.ChapterList, error) {
	return mangadexClient.FetchChapters(context.TODO(), mangaID)
}

func MangadexCovers(manga *md.Manga, p formats.Progress) (md.ImageList, error) {
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	covers, err := mangadexClient.FetchCovers(ctx, manga.Info.ID)
	if err != nil {
		return nil, err
	}

	coverPaths := make(chan md.Path)
	go func() {
		for _, path := range covers {
			coverPaths <- path
			p.Increase(1)
		}
		close(coverPaths)
	}()

	coverImages, eg := pathsToImages(coverPaths, ctx, cancel)

	results := make(md.ImageList, len(covers))
	for coverImage := range coverImages {
		p.Add(1)
		results = append(results, coverImage)
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	} else {
		return results, nil
	}
}

func MangadexPages(chapterList md.ChapterList, p formats.Progress) (md.ImageList, error) {
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	eg, ctx := errgroup.WithContext(ctx)

	chapters := make(chan md.Chapter)
	go func() {
		for _, chapter := range chapterList {
			chapters <- chapter
			p.Increase(1)
		}
		close(chapters)
	}()

	paths, childEg := chaptersToPaths(chapters, ctx, cancel, p)
	eg.Go(childEg.Wait)

	images, childEg := pathsToImages(paths, ctx, cancel)
	eg.Go(childEg.Wait)

	results := make(md.ImageList, 0)
	for image := range images {
		p.Add(1)
		results = append(results, image)
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	} else {
		return results, nil
	}
}

func chaptersToPaths(
	chapters <-chan md.Chapter,
	ctx context.Context,
	cancel context.CancelFunc,
	p formats.Progress,
) (<-chan md.Path, *errgroup.Group) {
	ch := make(chan md.Path)
	eg, ctx := errgroup.WithContext(ctx)
	eg.SetLimit(maxJobsChapter + 1)

	eg.Go(func() error {
		for {
			select {
			case <-ctx.Done():
				return fmt.Errorf("canceled")
			case chapter, ok := <-chapters:
				if !ok {
					return nil
				}
				eg.Go(func() error {
					paths, err := mangadexClient.FetchPaths(ctx, &chapter)
					if err != nil {
						defer cancel()
						return fmt.Errorf("chapter %v: paths: %w", chapter.Info.Identifier, err)
					} else {
						p.Add(1)
						for _, path := range paths {
							select {
							case <-ctx.Done():
								return fmt.Errorf("canceled")
							case ch <- path:
								p.Increase(1)
							}
						}
						return nil
					}
				})
			}
		}
	})

	go func() {
		eg.Wait() //nolint:errcheck
		close(ch)
	}()

	return ch, eg
}

func pathsToImages(
	paths <-chan md.Path,
	ctx context.Context,
	cancel context.CancelFunc,
) (<-chan md.Image, *errgroup.Group) {
	ch := make(chan md.Image)
	eg, ctx := errgroup.WithContext(ctx)
	eg.SetLimit(maxJobsImage + 1)

	eg.Go(func() error {
		for {
			select {
			case <-ctx.Done():
				return fmt.Errorf("canceled")
			case path, ok := <-paths:
				if !ok {
					return nil
				}
				eg.Go(func() error {
					image, err := getImage(httpClient, ctx, path.URL, 0)
					if err != nil {
						defer cancel()
						return fmt.Errorf("chapter %v: image %v: %w", path.ChapterIdentifier, path.ImageIdentifier, err)
					} else {
						select {
						case <-ctx.Done():
							return fmt.Errorf("canceled")
						case ch <- path.WithImage(image):
							return nil
						}
					}
				})
			}
		}
	})

	go func() {
		eg.Wait() //nolint:errcheck
		close(ch)
	}()

	return ch, eg
}

func getImage(client *http.Client, ctx context.Context, url string, try uint) (image.Image, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("prepare: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status: %v", resp.Status)
	}

	img, _, err := image.Decode(resp.Body)
	// Hack to fix broken images.
	if img == nil && try <= 10 {
		return getImage(client, ctx, url, try+1)
	}
	if err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	return img, nil
}
