package mangadex

import (
	"image"

	"golang.org/x/text/language"
)

type ImageItem struct {
	Image image.Image

	// hidden
	imageID   int
	chapterID Identifier
	volumeID  Identifier
}

type PathItem struct {
	URL string

	// hidden
	imageID   int
	chapterID Identifier
	volumeID  Identifier
}

func (i PathItem) WithImage(img image.Image) ImageItem {
	return ImageItem{
		Image:     img,
		chapterID: i.chapterID,
		volumeID:  i.volumeID,
		imageID:   i.imageID,
	}
}

type ChapterInfo struct {
	Title      string
	Views      int
	Region     language.Region
	GroupNames []string
	Hash       string
	ID         int

	// hidden
	Identifier       Identifier
	VolumeIdentifier Identifier
}

type MangaInfo struct {
	Title    string
	Authors  []string
	Artists  []string
	IsHentai bool
	ID       int
}