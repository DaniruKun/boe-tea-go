package commands

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/VTGare/boe-tea-go/internal/embeds"
	"github.com/VTGare/boe-tea-go/internal/widget"
	"github.com/VTGare/boe-tea-go/utils"
	"github.com/VTGare/gumi"
	"github.com/VTGare/iqdbgo"
	"github.com/VTGare/sengoku"
	"github.com/bwmarrin/discordgo"
	log "github.com/sirupsen/logrus"
)

var (
	noSauceEmbed = embeds.NewBuilder().InfoTemplate("Sorry, Boe Tea couldnt find source or the image, if you haven't yet please consider using methods below").AddField("iqdb", "`bt!iqdb`", true).AddField("ascii2d", "[Click here desu~](https://ascii2d.net)").AddField("Google Image Search", "[Click here desu~](https://www.google.com/imghp?hl=EN)").Finalize()
)

func init() {
	groupName := "source"

	Commands = append(Commands, &gumi.Command{
		Name:        "sauce",
		Aliases:     []string{"saucenao", "snao"},
		Description: "Finds sauce using SauceNAO reverse search engine.",
		Group:       groupName,
		Usage:       "bt!sauce <image attachment, link, reply or in last 5 messages>",
		Example:     "bt!saucenao https://ram.moe/cuteanimegirl.png",
		Exec:        saucenao,
	})

	Commands = append(Commands, &gumi.Command{
		Name:        "iqdb",
		Description: "***WARNING. NSFW results!*** Finds sauce using iqdb reverse search engine.",
		Group:       groupName,
		Usage:       "bt!iqdb <image attachment, link, reply or in last 5 messages>",
		Example:     "bt!iqdb https://emilia.moe/cuteanimegirl.png",
		Exec:        iqdb,
		NSFW:        true,
	})
}

func saucenao(ctx *gumi.Ctx) error {
	var (
		s    = ctx.Session
		m    = ctx.Event
		args = strings.Fields(ctx.Args.Raw)
	)

	url, err := findImage(s, m, args)
	if err != nil {
		return err
	}

	if url == "" {
		return utils.ErrNotEnoughArguments
	}

	log.Infof("Searching source on SauceNAO. Image URL: %v", url)
	sauceEmbeds, err := saucenaoEmbeds(url, true)
	if err != nil {
		if err == sengoku.ErrRateLimitReached {
			eb := embeds.NewBuilder().InfoTemplate("Boe Tea's getting rate limited by SauceNAO. If you want to support me, so I can afford monthly SauceNAO subscription consider becoming a patron!")
			eb.AddField("Patreon", "[Click here desu~](https://www.patreon.com/vtgare)")

			s.ChannelMessageSendEmbed(m.ChannelID, eb.Finalize())
		}
		return err
	}

	w := widget.NewWidget(s, m.Author.ID, sauceEmbeds)
	err = w.Start(m.ChannelID)
	if err != nil {
		return err
	}

	return nil
}

func saucenaoEmbeds(link string, nosauce bool) ([]*discordgo.MessageEmbed, error) {
	res, err := sc.Search(link)
	if err != nil {
		return nil, err
	}

	filtered := make([]*sengoku.Sauce, 0)
	for _, r := range res {
		if err != nil {
			continue
		}

		if r.Similarity < 70.0 {
			continue
		}

		if !r.Pretty {
			continue
		}

		filtered = append(filtered, r)
	}

	l := len(filtered)
	if l == 0 {
		if nosauce {
			return []*discordgo.MessageEmbed{noSauceEmbed}, nil
		}
		return nil, nil
	}

	log.Infof("Found source. Results: %v", l)
	embeds := make([]*discordgo.MessageEmbed, l)
	for ind, source := range filtered {
		embed := saucenaoToEmbed(source, ind, l)
		embeds[ind] = embed
	}

	return embeds, nil
}

func saucenaoToEmbed(source *sengoku.Sauce, index, length int) *discordgo.MessageEmbed {
	title := ""
	if length > 1 {
		title = fmt.Sprintf("[%v/%v] %v", index+1, length, source.Title)
	} else {
		title = fmt.Sprintf("%v", source.Title)
	}

	eb := embeds.NewBuilder()
	eb.Title(title).Thumbnail(source.Thumbnail)
	if src := source.URLs.Source; src != "" {
		if _, err := url.Parse(src); err == nil {
			eb.URL(src)
		}

		if strings.Contains(src, "pximg.net") {
			last := strings.LastIndex(src, "/")
			if last != -1 {
				id := strings.Trim(src[last+1:], ".jpengif")

				src = "https://pixiv.net/en/artworks/" + id
			}
		}

		eb.AddField("Source", src)
	}
	eb.AddField("Similarity", fmt.Sprintf("%v", source.Similarity))

	if source.Author != nil {
		if source.Author.Name != "" {
			str := ""
			if source.Author.URL != "" {
				str = fmt.Sprintf("[%v](%v)", source.Author.Name, source.Author.URL)
			} else {
				str = source.Author.Name
			}
			eb.AddField("Author", str)
		}
	}

	if str := joinSauceURLs(source.URLs.ExternalURLs, " • "); str != "" {
		eb.AddField("Other URLs", str)
	}
	return eb.Finalize()
}

func iqdb(ctx *gumi.Ctx) error {
	var (
		s    = ctx.Session
		m    = ctx.Event
		args = strings.Fields(ctx.Args.Raw)
	)

	url, err := findImage(s, m, args)
	if err != nil {
		return err
	}

	if url == "" {
		return utils.ErrNotEnoughArguments
	}

	log.Infof("Searching source on iqdb. Image URL: %s", url)
	res, err := iqdbgo.Search(url)
	if err != nil {
		return err
	}

	var messageEmbeds = make([]*discordgo.MessageEmbed, 0)
	length := len(res.PossibleMatches)
	if res.BestMatch != nil {
		length++
	}

	if length == 0 {
		s.ChannelMessageSendEmbed(m.ChannelID, noSauceEmbed)
		return nil
	}

	if res.BestMatch != nil {
		messageEmbeds = append(messageEmbeds, iqdbEmbed(res.BestMatch, true, 0, length))
	}
	for i, s := range res.PossibleMatches {
		if res.BestMatch != nil {
			i++
		}
		messageEmbeds = append(messageEmbeds, iqdbEmbed(s, false, i, length))
	}

	if len(messageEmbeds) > 1 {
		w := widget.NewWidget(s, m.Author.ID, messageEmbeds)
		err := w.Start(m.ChannelID)
		if err != nil {
			return err
		}
	} else {
		_, err = s.ChannelMessageSendEmbed(m.ChannelID, messageEmbeds[0])
		if err != nil {
			return err
		}
	}

	return nil
}

func iqdbEmbed(source *iqdbgo.Match, best bool, index, length int) *discordgo.MessageEmbed {
	matchType := ""
	if best {
		matchType = "Best match"
	} else {
		matchType = "Possible match"
	}

	if strings.HasPrefix(source.URL, "http:") {
		source.URL = strings.Replace(source.URL, "http:", "https:", 1)
	}
	if !strings.HasPrefix(source.URL, "https:") {
		source.URL = "https:" + source.URL
	}

	title := ""
	if length > 1 {
		title = fmt.Sprintf("[%v/%v] %v", index+1, length, matchType)
	} else {
		title = fmt.Sprintf("%v", matchType)
	}

	eb := embeds.NewBuilder()

	eb.Title(title).Image(source.Thumbnail)

	if source.URL != "" {
		if _, err := url.Parse(source.URL); err == nil {
			eb.URL(source.URL)
		}
		eb.AddField("Source", source.URL)
	}

	if source.Tags != "" {
		eb.AddField("Info", fmt.Sprintf("%v", source.Tags))
	}

	eb.AddField("Similarity", strconv.Itoa(source.Similarity))

	return eb.Finalize()
}

func findImage(s *discordgo.Session, m *discordgo.MessageCreate, args []string) (string, error) {
	if len(args) > 0 {
		if ImageURLRegex.MatchString(args[0]) {
			return args[0], nil
		} else if url, err := findImageFromMessageLink(s, args[0]); err == nil && url != "" {
			return url, nil
		}
	}

	if len(m.Attachments) > 0 {
		url := m.Attachments[0].URL
		if ImageURLRegex.MatchString(url) {
			return url, nil
		}
	}

	if ref := m.MessageReference; ref != nil {
		url, err := findImageFromMessageLink(s, fmt.Sprintf("https://discord.com/channels/%s/%s/%s", ref.GuildID, ref.ChannelID, ref.MessageID))
		if err == nil && url != "" {
			return url, nil
		}
	}

	if len(m.Embeds) > 0 {
		if m.Embeds[0].Image != nil {
			url := m.Embeds[0].Image.URL
			if ImageURLRegex.MatchString(url) {
				return url, nil
			}
		}
	}

	messages, err := s.ChannelMessages(m.ChannelID, 5, m.ID, "", "")
	if err != nil {
		return "", err
	}
	if recent := findRecentImage(messages); recent != "" {
		return recent, nil
	}

	return "", nil
}

func findRecentImage(messages []*discordgo.Message) string {
	for _, msg := range messages {
		f := ImageURLRegex.FindString(msg.Content)
		switch {
		case f != "":
			return f
		case len(msg.Attachments) > 0:
			return msg.Attachments[0].URL
		case len(msg.Embeds) > 0:
			if msg.Embeds[0].Image != nil {
				return msg.Embeds[0].Image.URL
			}
		}
	}

	return ""
}

func findImageFromMessageLink(s *discordgo.Session, arg string) (string, error) {
	if matches := messageLinkRegex.FindStringSubmatch(arg); matches != nil {
		m, err := s.ChannelMessage(matches[1], matches[2])
		if err != nil {
			return "", err
		}
		if recent := findRecentImage([]*discordgo.Message{m}); recent != "" {
			return recent, nil
		}
	}

	return "", nil
}
