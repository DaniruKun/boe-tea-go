package commands

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/VTGare/boe-tea-go/internal/arrays"
	"github.com/VTGare/boe-tea-go/internal/dgoutils"
	"github.com/VTGare/boe-tea-go/pkg/bot"
	"github.com/VTGare/boe-tea-go/pkg/messages"
	"github.com/VTGare/embeds"
	"github.com/VTGare/gumi"
	"github.com/bwmarrin/discordgo"
	"go.mongodb.org/mongo-driver/mongo"
)

func generalGroup(b *bot.Bot) {
	group := "general"

	b.Router.RegisterCmd(&gumi.Command{
		Name:        "set",
		Group:       group,
		Aliases:     []string{"cfg", "config", "settings"},
		Description: "Shows or edits server settings.",
		Usage:       "bt!set <setting name> <new setting>",
		Example:     "bt!set pixiv false",
		Flags:       map[string]string{},
		GuildOnly:   true,
		NSFW:        false,
		AuthorOnly:  false,
		Permissions: 0,
		RateLimiter: gumi.NewRateLimiter(5 * time.Second),
		Exec:        set(b),
	})

	b.Router.RegisterCmd(&gumi.Command{
		Name:        "about",
		Group:       group,
		Aliases:     []string{"invite", "patreon", "support"},
		Description: "Bot's about page with the invite link and other useful stuff.",
		Usage:       "bt!about",
		Example:     "bt!about",
		RateLimiter: gumi.NewRateLimiter(5 * time.Second),
		Exec:        about(b),
	})

	b.Router.RegisterCmd(&gumi.Command{
		Name:        "ping",
		Group:       group,
		Description: "Checks bot's availabity and response time.",
		Usage:       "bt!ping",
		Example:     "bt!ping",
		RateLimiter: gumi.NewRateLimiter(5 * time.Second),
		Exec:        ping(b),
	})

	b.Router.RegisterCmd(&gumi.Command{
		Name:        "feedback",
		Group:       group,
		Description: "Sends feedback to bot's author.",
		Usage:       "bt!feedback <your wall of text here>",
		Example:     "bt!feedback Damn your bot sucks!",
		Exec:        feedback(b),
	})

	b.Router.RegisterCmd(&gumi.Command{
		Name:        "stats",
		Group:       group,
		Description: "Shows bot's runtime stats.",
		Usage:       "bt!stats",
		Example:     "bt!stats",
		RateLimiter: gumi.NewRateLimiter(5 * time.Second),
		Exec:        stats(b),
	})

	b.Router.RegisterCmd(&gumi.Command{
		Name:        "addchannel",
		Group:       group,
		Aliases:     []string{"artchannel"},
		Description: "Adds a new art channel to server settings.",
		Usage:       "bt!addchannel [channel ids/category id...]",
		Example:     "bt!addchannel #sfw #nsfw #basement",
		GuildOnly:   true,
		Permissions: discordgo.PermissionAdministrator | discordgo.PermissionManageServer,
		RateLimiter: gumi.NewRateLimiter(5 * time.Second),
		Exec:        addchannel(b),
	})

	b.Router.RegisterCmd(&gumi.Command{
		Name:        "removechannel",
		Group:       group,
		Description: "Removes an art channel from server settings.",
		Usage:       "bt!removechannel [channel ids/category id...]",
		Example:     "bt!removechannel #sfw #nsfw #basement",
		GuildOnly:   true,
		Permissions: discordgo.PermissionAdministrator | discordgo.PermissionManageServer,
		RateLimiter: gumi.NewRateLimiter(5 * time.Second),
		Exec:        removechannel(b),
	})
}

func ping(b *bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		eb := embeds.NewBuilder()

		return ctx.ReplyEmbed(
			eb.Title("🏓 Pong!").AddField(
				"Heartbeat latency",
				ctx.Session.HeartbeatLatency().Round(time.Millisecond).String(),
			).Finalize(),
		)
	}
}

func about(b *bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		locale := messages.AboutEmbed()

		eb := embeds.NewBuilder()
		eb.Title(locale.Title).Thumbnail(ctx.Session.State.User.AvatarURL(""))
		eb.Description(locale.Description)

		eb.AddField(
			locale.SupportServer,
			messages.ClickHere("https://discord.gg/hcxuHE7"),
			true,
		)

		eb.AddField(
			locale.InviteLink,
			messages.ClickHere(
				"https://discord.com/api/oauth2/authorize?client_id=636468907049353216&permissions=537259072&scope=bot",
			),
			true,
		)

		eb.AddField(
			locale.Patreon,
			messages.ClickHere("https://patreon.com/vtgare"),
			true,
		)

		ctx.ReplyEmbed(eb.Finalize())
		return nil
	}
}

func feedback(b *bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		if ctx.Args.Len() == 0 {
			return messages.ErrIncorrectCmd(ctx.Command)
		}

		eb := embeds.NewBuilder()
		eb.Author(
			fmt.Sprintf("Feedback from %v", ctx.Event.Author.String()),
			"",
			ctx.Event.Author.AvatarURL(""),
		).Description(
			ctx.Args.Raw,
		).AddField(
			"Author Mention",
			ctx.Event.Author.Mention(),
			true,
		).AddField(
			"Author ID",
			ctx.Event.Author.ID,
			true,
		)

		if ctx.Event.GuildID != "" {
			eb.AddField(
				"Guild", ctx.Event.GuildID, true,
			)
		}

		if len(ctx.Event.Attachments) > 0 {
			att := ctx.Event.Attachments[0]
			if strings.HasSuffix(att.Filename, "png") ||
				strings.HasSuffix(att.Filename, "jpg") ||
				strings.HasSuffix(att.Filename, "gif") {
				eb.Image(att.URL)
			}
		}

		ch, err := ctx.Session.UserChannelCreate(ctx.Router.AuthorID)
		if err != nil {
			return err
		}

		_, err = ctx.Session.ChannelMessageSendEmbed(ch.ID, eb.Finalize())
		if err != nil {
			return err
		}

		eb.Clear()
		ctx.ReplyEmbed(eb.SuccessTemplate("Feedback message has been sent.").Finalize())
		return nil
	}
}

func stats(b *bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		var (
			s   = ctx.Session
			mem runtime.MemStats
		)
		runtime.ReadMemStats(&mem)

		guilds := len(s.State.Guilds)
		channels := 0
		for _, g := range s.State.Guilds {
			channels += len(g.Channels)
		}

		latency := s.HeartbeatLatency().Round(1 * time.Millisecond)

		eb := embeds.NewBuilder()
		eb.Title("Bot stats")
		eb.AddField(
			"Guilds",
			strconv.Itoa(guilds),
			true,
		).AddField(
			"Channels",
			strconv.Itoa(channels),
			true,
		).AddField(
			"Latency",
			latency.String(),
			true,
		).AddField(
			"Shards",
			strconv.Itoa(s.ShardCount),
		).AddField(
			"RAM used",
			fmt.Sprintf("%v MB", mem.Alloc/1024/1024),
		)

		return ctx.ReplyEmbed(eb.Finalize())
	}
}

func set(b *bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		switch {
		case ctx.Args.Len() == 0:
			gd, err := ctx.Session.Guild(ctx.Event.GuildID)
			if err != nil {
				return messages.ErrGuildNotFound(err, ctx.Event.GuildID)
			}

			guild, err := b.Guilds.FindOne(context.Background(), gd.ID)
			if err != nil {
				switch {
				case errors.Is(err, mongo.ErrNoDocuments):
					return messages.ErrGuildNotFound(err, ctx.Event.GuildID)
				default:
					return err
				}
			}

			var (
				eb  = embeds.NewBuilder()
				msg = messages.Set()
			)

			eb.Title(msg.CurrentSettings).Description(fmt.Sprintf("**%v**", gd.Name))
			eb.Thumbnail(gd.IconURL())
			eb.Footer("Ebin message.", "")

			eb.AddField(
				msg.General.Title,
				fmt.Sprintf(
					"**%v**: %v | **%v**: %v",
					msg.General.Prefix, guild.Prefix,
					msg.General.NSFW, messages.FormatBool(guild.NSFW),
				),
			)

			eb.AddField(
				msg.Features.Title,
				fmt.Sprintf(
					"**%v**: %v | **%v**: %v | **%v**: %v",
					msg.Features.Repost, guild.Repost,
					msg.Features.Crosspost, messages.FormatBool(guild.Crosspost),
					msg.Features.Reactions, messages.FormatBool(guild.Reactions),
				),
			)

			eb.AddField(
				msg.PixivSettings.Title,
				fmt.Sprintf(
					"**%v**: %v | **%v**: %v",
					msg.PixivSettings.Enabled, messages.FormatBool(guild.Pixiv),
					msg.PixivSettings.Limit, strconv.Itoa(guild.Limit),
				),
			)

			eb.AddField(
				msg.TwitterSettings.Title,
				fmt.Sprintf(
					"**%v**: %v",
					msg.TwitterSettings.Enabled, messages.FormatBool(guild.Twitter),
				),
			)

			if len(guild.ArtChannels) > 0 {
				eb.AddField(
					msg.ArtChannels,
					strings.Join(arrays.MapString(guild.ArtChannels, func(s string) string {
						return fmt.Sprintf("<#%v> | `%v`", s, s)
					}), "\n"),
				)
			}

			ctx.ReplyEmbed(eb.Finalize())
			return nil
		case ctx.Args.Len() >= 2:
			perms, err := dgoutils.MemberHasPermission(
				ctx.Session,
				ctx.Event.GuildID,
				ctx.Event.Author.ID,
				discordgo.PermissionAdministrator|discordgo.PermissionManageServer,
			)
			if err != nil {
				return err
			}

			if !perms {
				return ctx.Router.OnNoPermissionsCallback(ctx)
			}

			guild, err := b.Guilds.FindOne(context.Background(), ctx.Event.GuildID)
			if err != nil {
				return err
			}

			var (
				settingName     = ctx.Args.Get(0)
				newSetting      = ctx.Args.Get(1)
				newSettingEmbed interface{}
				oldSettingEmbed interface{}
			)

			switch settingName.Raw {
			case "prefix":
				if unicode.IsLetter(rune(newSetting.Raw[len(newSetting.Raw)-1])) {
					newSetting.Raw += " "
				}

				if len(newSetting.Raw) > 5 {
					return messages.ErrPrefixTooLong(newSetting.Raw)
				}

				oldSettingEmbed = guild.Prefix
				newSettingEmbed = newSetting.Raw
				guild.Prefix = newSetting.Raw
			case "limit":
				limit, err := strconv.Atoi(newSetting.Raw)
				if err != nil {
					return messages.ErrParseInt(newSetting.Raw)
				}

				oldSettingEmbed = guild.Limit
				newSettingEmbed = limit
				guild.Limit = limit
			case "repost":
				if newSetting.Raw != "enabled" && newSetting.Raw != "disabled" && newSetting.Raw != "strict" {
					return messages.ErrUnknownRepostOption(newSetting.Raw)
				}

				oldSettingEmbed = guild.Repost
				newSettingEmbed = newSetting.Raw
				guild.Repost = newSetting.Raw
			case "nsfw":
				nsfw, err := parseBool(newSetting.Raw)
				if err != nil {
					return err
				}

				oldSettingEmbed = guild.NSFW
				newSettingEmbed = nsfw
				guild.NSFW = nsfw
			case "crosspost":
				crosspost, err := parseBool(newSetting.Raw)
				if err != nil {
					return err
				}

				oldSettingEmbed = guild.Crosspost
				newSettingEmbed = crosspost
				guild.Crosspost = crosspost
			case "reactions":
				new, err := parseBool(newSetting.Raw)
				if err != nil {
					return err
				}

				oldSettingEmbed = guild.Reactions
				newSettingEmbed = new
				guild.Reactions = new
			case "pixiv":
				new, err := parseBool(newSetting.Raw)
				if err != nil {
					return err
				}

				oldSettingEmbed = guild.Pixiv
				newSettingEmbed = new
				guild.Pixiv = new
			case "twitter":
				new, err := parseBool(newSetting.Raw)
				if err != nil {
					return err
				}

				oldSettingEmbed = guild.Twitter
				newSettingEmbed = new
				guild.Twitter = new
			default:
				return messages.ErrUnknownSetting(settingName.Raw)
			}

			_, err = b.Guilds.ReplaceOne(context.Background(), guild)
			if err != nil {
				return err
			}

			eb := embeds.NewBuilder()
			eb.InfoTemplate("Successfully changed setting.")
			eb.AddField("Setting name", settingName.Raw, true)
			eb.AddField("Old setting", fmt.Sprintf("%v", oldSettingEmbed), true)
			eb.AddField("New setting", fmt.Sprintf("%v", newSettingEmbed), true)

			ctx.ReplyEmbed(eb.Finalize())
			return nil
		default:
			return messages.ErrIncorrectCmd(ctx.Command)
		}
	}
}

func addchannel(b *bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		if ctx.Args.Len() == 0 {
			return messages.ErrIncorrectCmd(ctx.Command)
		}

		guild, err := b.Guilds.FindOne(context.Background(), ctx.Event.GuildID)
		if err != nil {
			return messages.ErrGuildNotFound(err, ctx.Event.GuildID)
		}

		channels := make([]string, 0)
		for _, arg := range ctx.Args.Arguments {
			ch, err := ctx.Session.Channel(strings.Trim(arg.Raw, "<#>"))
			if err != nil {
				return err
			}

			if ch.GuildID != guild.ID {
				return messages.ErrForeignChannel(ch.ID)
			}

			switch ch.Type {
			case discordgo.ChannelTypeGuildText:
				exists := false
				for _, channelID := range guild.ArtChannels {
					if channelID == ch.ID {
						exists = true
					}
				}

				if exists {
					return messages.ErrAlreadyArtChannel(ch.ID)
				}

				channels = append(channels, ch.ID)
			case discordgo.ChannelTypeGuildCategory:
				gcs, err := ctx.Session.GuildChannels(guild.ID)
				if err != nil {
					return err
				}

				for _, gc := range gcs {
					if gc.Type != discordgo.ChannelTypeGuildText {
						continue
					}

					if gc.ParentID == ch.ID {
						exists := false
						for _, channelID := range guild.ArtChannels {
							if channelID == gc.ID {
								exists = true
							}
						}

						if exists {
							return messages.ErrAlreadyArtChannel(ch.ID)
						}

						channels = append(channels, gc.ID)
					}
				}
			default:
				return nil
			}
		}

		_, err = b.Guilds.InsertArtChannels(
			context.Background(),
			guild.ID,
			channels,
		)

		if err != nil {
			return err
		}

		eb := embeds.NewBuilder()
		eb.SuccessTemplate(messages.AddArtChannelSuccess(channels))
		return ctx.ReplyEmbed(eb.Finalize())
	}
}

func removechannel(b *bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		if ctx.Args.Len() == 0 {
			return messages.ErrIncorrectCmd(ctx.Command)
		}

		guild, err := b.Guilds.FindOne(context.Background(), ctx.Event.GuildID)
		if err != nil {
			return messages.ErrGuildNotFound(err, ctx.Event.GuildID)
		}

		channels := make([]string, 0)
		for _, arg := range ctx.Args.Arguments {
			ch, err := ctx.Session.Channel(strings.Trim(arg.Raw, "<#>"))
			if err != nil {
				return messages.ErrChannelNotFound(err, arg.Raw)
			}

			if ch.GuildID != ctx.Event.GuildID {
				return messages.ErrForeignChannel(ch.ID)
			}

			switch ch.Type {
			case discordgo.ChannelTypeGuildText:
				channels = append(channels, ch.ID)
			case discordgo.ChannelTypeGuildCategory:
				gcs, err := ctx.Session.GuildChannels(guild.ID)
				if err != nil {
					return err
				}

				for _, gc := range gcs {
					if gc.Type != discordgo.ChannelTypeGuildText {
						continue
					}

					if gc.ParentID == ch.ID {
						channels = append(channels, gc.ID)
					}
				}
			default:
				return nil
			}
		}

		_, err = b.Guilds.DeleteArtChannels(
			context.Background(),
			guild.ID,
			channels,
		)

		if err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				return messages.RemoveArtChannelFail(channels)
			}

			return err
		}

		eb := embeds.NewBuilder()
		eb.SuccessTemplate(messages.RemoveArtChannelSuccess(channels))
		return ctx.ReplyEmbed(eb.Finalize())
	}
}

func parseBool(s string) (bool, error) {
	s = strings.ToLower(s)
	if s == "true" || s == "enabled" || s == "on" {
		return true, nil
	}

	if s == "false" || s == "disabled" || s == "off" {
		return false, nil
	}

	return false, messages.ErrParseBool(s)
}
