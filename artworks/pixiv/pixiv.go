package pixiv

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/VTGare/boe-tea-go/artworks"
	"github.com/VTGare/boe-tea-go/internal/arrays"
	"github.com/VTGare/boe-tea-go/messages"
	"github.com/VTGare/boe-tea-go/store"
	"github.com/VTGare/embeds"
	"github.com/bwmarrin/discordgo"
	"github.com/everpcpc/pixiv"
)

var (
	regex = regexp.MustCompile(
		`(?i)http(?:s)?:\/\/(?:www\.)?pixiv\.net\/(?:en\/)?(?:artworks\/|member_illust\.php\?)(?:mode=medium\&)?(?:illust_id=)?([0-9]+)`,
	)
)

type Pixiv struct {
	app *pixiv.AppPixivAPI
}

type Artwork struct {
	ID     string
	Type   string
	Author string
	Title  string
	Likes  int
	Pages  int
	Tags   []string
	Images []*Image
	NSFW   bool
	url    string
}

type Image struct {
	Preview  string
	Original string
}

func New(authToken, refreshToken string) (artworks.Provider, error) {
	_, err := pixiv.LoadAuth(authToken, refreshToken, time.Now())
	if err != nil {
		return nil, err
	}

	return &Pixiv{app: pixiv.NewApp()}, nil
}

func (p *Pixiv) Match(s string) (string, bool) {
	res := regex.FindStringSubmatch(s)
	if res == nil {
		return "", false
	}

	return res[1], true
}

func (p *Pixiv) Find(id string) (artworks.Artwork, error) {
	i, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		return nil, err
	}

	illust, err := p.app.IllustDetail(i)
	if err != nil {
		return nil, err
	}

	author := ""
	if illust.User != nil {
		author = illust.User.Name
	} else {
		author = "Unknown"
	}

	tags := make([]string, 0)
	nsfw := false
	for _, tag := range illust.Tags {
		if tag.Name == "R-18" {
			nsfw = true
		}

		tags = append(tags, tag.Name)
	}

	images := make([]*Image, 0, illust.PageCount)
	if page := illust.MetaSinglePage; page != nil {
		if page.OriginalImageURL != "" {
			img := &Image{
				Original: page.OriginalImageURL,
				Preview:  illust.Images.Large,
			}

			images = append(images, img)
		}
	}

	for _, page := range illust.MetaPages {
		img := &Image{
			Original: page.Images.Original,
			Preview:  page.Images.Large,
		}

		images = append(images, img)
	}

	artwork := &Artwork{
		ID:     id,
		url:    "https://pixiv.net/en/artworks/" + id,
		Title:  illust.Title,
		Author: author,
		Tags:   tags,
		Images: images,
		NSFW:   nsfw,
		Type:   illust.Type,
		Pages:  illust.PageCount,
		Likes:  illust.TotalBookmarks,
	}

	return artwork, nil
}

func (p *Pixiv) Enabled(g *store.Guild) bool {
	return g.Pixiv
}

func (a *Artwork) StoreArtwork() *store.Artwork {
	return &store.Artwork{
		Title:  a.Title,
		Author: a.Author,
		URL:    a.url,
		Images: a.imageURLs(),
	}
}

func (a *Artwork) MessageSends(footer string, hasTags bool) ([]*discordgo.MessageSend, error) {
	var (
		length = len(a.Images)
		pages  = make([]*discordgo.MessageSend, 0, length)
		eb     = embeds.NewBuilder()
	)

	if length == 0 {
		eb.Title("❎ An error has occured.")
		eb.Description("Pixiv artwork has been deleted or the ID does not exist.")
		eb.Footer(footer, "")

		return []*discordgo.MessageSend{
			{Embeds: []*discordgo.MessageEmbed{eb.Finalize()}},
		}, nil
	}

	if length > 1 {
		eb.Title(fmt.Sprintf("%v by %v | Page %v / %v", a.Title, a.Author, 1, length))
	} else {
		eb.Title(fmt.Sprintf("%v by %v", a.Title, a.Author))
	}

	if hasTags {
		tags := arrays.Map(a.Tags, func(s string) string {
			return fmt.Sprintf("[%v](https://pixiv.net/en/tags/%v/artworks)", s, s)
		})

		eb.Description(fmt.Sprintf("**Tags**\n%v", strings.Join(tags, " • ")))
	}

	eb.URL(a.url).
		AddField("Likes", strconv.Itoa(a.Likes), true).
		AddField("Original quality", messages.ClickHere(a.Images[0].originalProxy()), true).
		Timestamp(time.Now())

	if footer != "" {
		eb.Footer(footer, "")
	}

	eb.Image(a.Images[0].previewProxy())
	pages = append(pages, &discordgo.MessageSend{Embeds: []*discordgo.MessageEmbed{eb.Finalize()}})
	if length > 1 {
		for ind, image := range a.Images[1:] {
			eb := embeds.NewBuilder()

			eb.Title(fmt.Sprintf("%v by %v | Page %v / %v", a.Title, a.Author, ind+2, length))
			eb.Image(image.previewProxy())
			eb.URL(a.url).Timestamp(time.Now())

			if footer != "" {
				eb.Footer(footer, "")
			}

			eb.AddField("Likes", strconv.Itoa(a.Likes), true)
			eb.AddField("Original quality", messages.ClickHere(image.originalProxy()), true)

			pages = append(pages, &discordgo.MessageSend{Embeds: []*discordgo.MessageEmbed{eb.Finalize()}})
		}
	}

	return pages, nil
}

func (a *Artwork) URL() string {
	return a.url
}

func (a *Artwork) Len() int {
	return a.Pages
}

func (a *Artwork) imageURLs() []string {
	urls := make([]string, 0, len(a.Images))

	for _, img := range a.Images {
		urls = append(urls, img.originalProxy())
	}

	return urls
}

func (i Image) originalProxy() string {
	return strings.Replace(i.Original, "https://i.pximg.net", "https://boe-tea-pximg.herokuapp.com", 1)
}

func (i Image) previewProxy() string {
	return strings.Replace(i.Preview, "https://i.pximg.net", "https://boe-tea-pximg.herokuapp.com", 1)
}
