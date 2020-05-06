package utils

import (
	"errors"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/VTGare/boe-tea-go/database"
	"github.com/VTGare/boe-tea-go/services"
	"github.com/bwmarrin/discordgo"
)

//ActionFunc is a function type alias for prompt actions
type ActionFunc = func() bool

//PromptOptions is a struct that defines prompt's behaviour.
type PromptOptions struct {
	Actions map[string]ActionFunc
	Message string
	Timeout time.Duration
}

type PixivOptions struct {
	ProcPrompt bool
	Indexes    map[int]bool
	Exclude    bool
}

var (
	EmojiRegex            = regexp.MustCompile(`(\x{00a9}|\x{00ae}|[\x{2000}-\x{3300}]|\x{d83c}[\x{d000}-\x{dfff}]|\x{d83d}[\x{d000}-\x{dfff}]|\x{d83e}[\x{d000}-\x{dfff}])`)
	NumRegex              = regexp.MustCompile(`([0-9]+)`)
	EmbedColor            = 0x439ef1
	AuthorID              = "244208152776540160"
	PixivRegex            = regexp.MustCompile(`http(?:s)?:\/\/(?:www\.)?pixiv\.net\/(?:en\/)?(?:artworks\/|member_illust\.php\?illust_id=)([0-9]+)`)
	ErrNotEnoughArguments = errors.New("not enough arguments")
	ErrParsingArgument    = errors.New("error parsing arguments, please make sure all arguments are integers")
)

func EmbedTimestamp() string {
	return time.Now().Format(time.RFC3339)
}

//FindAuthor is a SauceNAO helper function that finds original source author string.
func FindAuthor(sauce services.Sauce) string {
	if sauce.Data.MemberName != "" {
		return sauce.Data.MemberName
	} else if sauce.Data.Author != "" {
		return sauce.Data.Author
	} else if creator, ok := sauce.Data.Creator.(string); ok {
		return creator
	}

	return "-"
}

//PostPixiv reposts Pixiv images from a link to a discord channel
func PostPixiv(s *discordgo.Session, m *discordgo.MessageCreate, pixivIDs []string, opts ...PixivOptions) error {
	if opts == nil {
		opts = []PixivOptions{
			{
				ProcPrompt: true,
				Indexes:    map[int]bool{},
				Exclude:    true,
			},
		}
	}

	var ask bool
	var links bool

	guild := database.GuildCache[m.GuildID]
	switch guild.Repost {
	case "ask":
		ask = true
	case "links":
		ask = false
		links = true
	case "embeds":
		ask = false
		links = false
	}

	if ask {
		prompt := CreatePrompt(s, m, &PromptOptions{
			Message: "Send images as links (✅) or embeds (❎)?",
			Actions: map[string]ActionFunc{
				"✅": func() bool {
					return true
				},
				"❎": func() bool {
					return false
				},
			},
			Timeout: time.Second * 15,
		})
		if prompt == nil {
			return nil
		}
		links = prompt()
	}

	aggregatedPosts := make([]interface{}, 0)
	for _, link := range pixivIDs {
		post, err := services.GetPixivPost(link)
		if err != nil {
			return err
		}

		for ind, image := range post.Images {
			title := ""
			if len(post.Images) == 1 {
				title = fmt.Sprintf("%v by %v", post.Title, post.Author)
			} else {
				title = fmt.Sprintf("%v by %v. Page %v/%v", post.Title, post.Author, ind+1, len(post.Images))
			}

			if links {
				title += fmt.Sprintf("\n%v\n♥ %v", image, post.Likes)
				aggregatedPosts = append(aggregatedPosts, title)
			} else {
				embedWarning := fmt.Sprintf("If embed is empty follow this link to see the image: %v", image)
				aggregatedPosts = append(aggregatedPosts, discordgo.MessageEmbed{
					Title:     title,
					URL:       image,
					Color:     EmbedColor,
					Timestamp: time.Now().Format(time.RFC3339),
					Fields: []*discordgo.MessageEmbedField{
						{
							Name:   "Likes",
							Value:  strconv.Itoa(post.Likes),
							Inline: true,
						},
						{
							Name:   "Tags",
							Value:  strings.Join(post.Tags, " • "),
							Inline: true,
						},
					},
					Image: &discordgo.MessageEmbedImage{
						URL: image,
					},
					Footer: &discordgo.MessageEmbedFooter{
						Text: embedWarning,
					},
				})
			}
		}
	}

	if len(aggregatedPosts) >= guild.Limit {
		return errors.New("hold your horses, too many images")
	}

	flag := true
	if opts[0].ProcPrompt {
		if len(aggregatedPosts) >= guild.LargeSet {
			message := ""
			if len(aggregatedPosts) >= 3 {
				message = fmt.Sprintf("Large set of images (%v), do you want me to send each image individually?", len(aggregatedPosts))
			} else {
				message = "Do you want me to send each image individually?"
			}

			prompt := CreatePrompt(s, m, &PromptOptions{
				Message: message,
				Actions: map[string]ActionFunc{
					"👌": func() bool {
						return true
					},
				},
				Timeout: time.Second * 15,
			})
			if prompt == nil {
				return nil
			}
			flag = prompt()
		}
	}

	if flag {
		log.Println(fmt.Sprintf("Successfully reposting %v images in %v", len(aggregatedPosts), guild.GuildID))
		for ind, post := range aggregatedPosts {
			if _, ok := opts[0].Indexes[ind+1]; ok {
				continue
			}

			if p, isEmbed := post.(discordgo.MessageEmbed); isEmbed {
				_, err := s.ChannelMessageSendEmbed(m.ChannelID, &p)
				if err != nil {
					return err
				}
			} else {
				s.ChannelMessageSend(m.ChannelID, post.(string))
			}
		}
	}
	return nil
}

//CreatePrompt sends a prompt message to a discord channel
func CreatePrompt(s *discordgo.Session, m *discordgo.MessageCreate, opts *PromptOptions) ActionFunc {
	prompt, _ := s.ChannelMessageSend(m.ChannelID, opts.Message)
	for emoji := range opts.Actions {
		s.MessageReactionAdd(m.ChannelID, prompt.ID, emoji)
	}

	var reaction *discordgo.MessageReaction
	for {
		select {
		case k := <-nextMessageReactionAdd(s):
			reaction = k.MessageReaction
		case <-time.After(opts.Timeout):
			s.ChannelMessageDelete(prompt.ChannelID, prompt.ID)
			return nil
		}

		if _, ok := opts.Actions[reaction.Emoji.APIName()]; !ok {
			continue
		}

		if reaction.MessageID != prompt.ID || s.State.User.ID == reaction.UserID || reaction.UserID != m.Author.ID {
			continue
		}

		s.ChannelMessageDelete(prompt.ChannelID, prompt.ID)
		return opts.Actions[reaction.Emoji.APIName()]
	}
}

func nextMessageReactionAdd(s *discordgo.Session) chan *discordgo.MessageReactionAdd {
	out := make(chan *discordgo.MessageReactionAdd)
	s.AddHandlerOnce(func(_ *discordgo.Session, e *discordgo.MessageReactionAdd) {
		out <- e
	})
	return out
}

func FormatBool(b bool) string {
	if b {
		return "enabled"
	}
	return "disabled"
}

func CreateDB(eventGuilds []*discordgo.Guild) error {
	allGuilds := database.AllGuilds()
	for _, guild := range *allGuilds {
		database.GuildCache[guild.GuildID] = guild
	}

	newGuilds := make([]interface{}, 0)
	for _, guild := range eventGuilds {
		if _, ok := database.GuildCache[guild.ID]; !ok {
			log.Println(guild.ID, "not found in database. Adding...")
			g := database.DefaultGuildSettings(guild.ID)
			newGuilds = append(newGuilds, g)
			database.GuildCache[g.GuildID] = *g
		}
	}

	if len(newGuilds) > 0 {
		err := database.InsertManyGuilds(newGuilds)
		if err != nil {
			return err
		} else {
			log.Println("Successfully inserted all current guilds.")
		}
	}

	log.Println(fmt.Sprintf("Connected to %v guilds", len(eventGuilds)))
	return nil
}

func GetEmoji(s *discordgo.Session, guildID, e string) (string, error) {
	if EmojiRegex.MatchString(e) || e == "👌" {
		return e, nil
	}

	emojiID := NumRegex.FindString(e)
	emoji, err := s.State.Emoji(guildID, emojiID)
	if err != nil {
		return "", err
	}
	return emoji.APIName(), nil
}
