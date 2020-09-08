package commands

import (
	"fmt"
	"strings"

	"github.com/VTGare/boe-tea-go/internal/database"
	"github.com/VTGare/boe-tea-go/utils"
	"github.com/VTGare/gumi"
	"github.com/bwmarrin/discordgo"
)

func init() {
	cp := CommandFramework.AddGroup("crosspost", gumi.GroupDescription("Cross-posting feature settings"))
	cr := cp.AddCommand("create", createGroup, gumi.CommandDescription("Creates a new cross-post group."))
	cr.Help.AddField("Usage", "bt!create <group name> [channel IDs or mentions]", false)

	dl := cp.AddCommand("delete", deleteGroup, gumi.CommandDescription("Deletes a cross-post group."))
	dl.Help.AddField("Usage", "bt!delete <group name>", false)

	cp.AddCommand("groups", groups, gumi.CommandDescription("Lists all your cross-post groups."), gumi.WithAliases("list", "allgroups", "ls"))

	pop := cp.AddCommand("pop", removeFromGroup, gumi.CommandDescription("Removes channels from a cross-post group."), gumi.WithAliases("remove"))
	pop.Help.AddField("Usage", "bt!pop <group name> [channel IDs or mentions]", false)

	push := cp.AddCommand("push", addToGroup, gumi.CommandDescription("Adds channels to a cross-post group."), gumi.WithAliases("add"))
	push.Help.AddField("Usage", "bt!push <group name> [channel IDs or mentions]", false)

	copyc := cp.AddCommand("copy", copyGroup, gumi.CommandDescription("Copies a cross-post group."), gumi.WithAliases("cp", "clone"))
	copyc.Help.AddField("Usage", "bt!copy <source group name> <destination group name> <parent ID>", false)
}

func groups(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	user := database.DB.FindUser(m.Author.ID)
	if user == nil {
		return fmt.Errorf("user settings not found, create create a group first with the following command: ``bt!create <group name> <parent ID>``")
	}

	embed := &discordgo.MessageEmbed{
		Title:     fmt.Sprintf("%v's cross-post groups", m.Author.Username),
		Color:     utils.EmbedColor,
		Timestamp: utils.EmbedTimestamp(),
		Thumbnail: &discordgo.MessageEmbedThumbnail{URL: m.Author.AvatarURL("")},
	}

	for _, g := range user.ChannelGroups {
		if len(g.Children) > 0 {
			children := utils.Map(g.Children, func(str string) string {
				return fmt.Sprintf("<#%v>", str)
			})
			embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{Name: g.Name, Value: fmt.Sprintf("**Parent:** [<#%v>]\n**Children:** %v", g.Parent, children)})
		} else {
			embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{Name: g.Name, Value: fmt.Sprintf("**Parent:** [<#%v>]\n**Children:** -", g.Parent)})
		}
	}

	if len(embed.Fields) == 0 {
		embed.Description = ":gun:🤠 *This town ain't big enough for the both of us!*\n"
		embed.Image = &discordgo.MessageEmbedImage{URL: "https://thumbs.gfycat.com/InconsequentialPerfumedGadwall-size_restricted.gif"}
	}

	s.ChannelMessageSendEmbed(m.ChannelID, embed)
	return nil
}

func createGroup(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("``bt!create`` requires two arguments. Example: ``bt!create touhou #lewdtouhouart``")
	}

	user := database.DB.FindUser(m.Author.ID)
	if user == nil {
		user = database.NewUserSettings(m.Author.ID)
		err := database.DB.InsertOneUser(user)
		if err != nil {
			return fmt.Errorf("Fatal database error: %v", err)
		}
	}

	var (
		groupName = args[0]
		ch        = args[1]
	)

	if strings.HasPrefix(ch, "<#") {
		ch = strings.Trim(ch, "<#>")
	}
	if _, err := s.State.Channel(ch); err != nil {
		return fmt.Errorf("unable to find channel ``%v``. Make sure Boe Tea is present on the server and able to read the channel", ch)
	}

	err := database.DB.CreateGroup(m.Author.ID, groupName, ch)
	if err != nil {
		return fmt.Errorf("Fatal database error: %v", err)
	}

	s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
		Title:     "✅ Sucessfully created a cross-post group!",
		Color:     utils.EmbedColor,
		Timestamp: utils.EmbedTimestamp(),
		Thumbnail: &discordgo.MessageEmbedThumbnail{URL: utils.DefaultEmbedImage},
		Fields:    []*discordgo.MessageEmbedField{{Name: "Name", Value: groupName}, {Name: "Parent channel", Value: fmt.Sprintf("<#%v>", ch)}},
	})
	return nil
}

func deleteGroup(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("``bt!delete`` requires at least one argument.\n**Usage:** ``bt!delete ntr``")
	}

	user := database.DB.FindUser(m.Author.ID)
	if user == nil {
		s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
			Title:     "❎ Failed to delete a cross-post group!",
			Color:     utils.EmbedColor,
			Timestamp: utils.EmbedTimestamp(),
			Thumbnail: &discordgo.MessageEmbedThumbnail{URL: utils.DefaultEmbedImage},
			Fields:    []*discordgo.MessageEmbedField{{Name: "Reason", Value: "You have no cross-post groups yet."}},
		})
		return nil
	}

	err := database.DB.DeleteGroup(m.Author.ID, args[0])
	if err != nil {
		return fmt.Errorf("Fatal database error: %v", err)
	}

	s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
		Title:     "✅ Sucessfully deleted a cross-post group!",
		Color:     utils.EmbedColor,
		Timestamp: utils.EmbedTimestamp(),
		Thumbnail: &discordgo.MessageEmbedThumbnail{URL: utils.DefaultEmbedImage},
		Fields:    []*discordgo.MessageEmbedField{{Name: "Name", Value: args[0]}},
	})

	return nil
}

func removeFromGroup(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("``bt!remove`` requires at least two arguments.\n**Usage:** ``bt!remove nudes #nsfw``")
	}

	user := database.DB.FindUser(m.Author.ID)
	if user == nil {
		s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
			Title:     "❎ Failed to remove from a cross-post group!",
			Color:     utils.EmbedColor,
			Timestamp: utils.EmbedTimestamp(),
			Thumbnail: &discordgo.MessageEmbedThumbnail{URL: utils.DefaultEmbedImage},
			Fields:    []*discordgo.MessageEmbedField{{Name: "Reason", Value: "You have no cross-post groups yet."}},
		})
		return nil
	}

	ids := utils.Map(args[1:], func(s string) string {
		return strings.Trim(s, "<#>")
	})

	found, err := database.DB.RemoveFromGroup(m.Author.ID, args[0], ids...)
	if err != nil {
		return fmt.Errorf("Fatal database error: %v", err)
	}

	if len(found) > 0 {
		s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
			Title:     "✅ Sucessfully removed channels from a cross-post group!",
			Color:     utils.EmbedColor,
			Timestamp: utils.EmbedTimestamp(),
			Thumbnail: &discordgo.MessageEmbedThumbnail{URL: utils.DefaultEmbedImage},
			Fields: []*discordgo.MessageEmbedField{{Name: "Group name", Value: args[0]}, {Name: "Channels", Value: strings.Join(utils.Map(found, func(s string) string {
				return fmt.Sprintf("<#%v>", s)
			}), " ")}},
		})
	} else {
		s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
			Title:     "❎ Failed to remove channels from a cross-post group!",
			Color:     utils.EmbedColor,
			Timestamp: utils.EmbedTimestamp(),
			Thumbnail: &discordgo.MessageEmbedThumbnail{URL: utils.DefaultEmbedImage},
			Fields:    []*discordgo.MessageEmbedField{{Name: "Group name", Value: args[0]}, {Name: "Reason", Value: "No valid channels were found"}},
		})
	}

	return nil
}

func addToGroup(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("``bt!push`` requires at least two arguments.\n**Usage:** ``bt!push hololive #marine-booty``")
	}

	user := database.DB.FindUser(m.Author.ID)
	if user == nil {
		s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
			Title:     "❎ Failed to add to a cross-post group!",
			Color:     utils.EmbedColor,
			Timestamp: utils.EmbedTimestamp(),
			Thumbnail: &discordgo.MessageEmbedThumbnail{URL: utils.DefaultEmbedImage},
			Fields:    []*discordgo.MessageEmbedField{{Name: "Reason", Value: "You have no cross-post groups yet."}},
		})
		return nil
	}

	groupName := args[0]
	channelsMap := make(map[string]bool, 0)
	for _, id := range args[1:] {
		channelsMap[id] = true
	}

	channels := make([]string, 0)
	for ch := range channelsMap {
		if strings.HasPrefix(ch, "<#") {
			ch = strings.Trim(ch, "<#>")
		}

		if _, err := s.State.Channel(ch); err != nil {
			return fmt.Errorf("unable to find channel ``%v``. Make sure Boe Tea is present on the server and able to read the channel", ch)
		}

		channels = append(channels, ch)
	}

	added, err := database.DB.AddToGroup(m.Author.ID, groupName, channels...)
	if err != nil {
		return fmt.Errorf("Fatal database error: %v", err)
	}

	if len(added) > 0 {
		s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
			Title:     "✅ Sucessfully added channels to a cross-post group!",
			Color:     utils.EmbedColor,
			Timestamp: utils.EmbedTimestamp(),
			Thumbnail: &discordgo.MessageEmbedThumbnail{URL: utils.DefaultEmbedImage},
			Fields: []*discordgo.MessageEmbedField{{Name: "Name", Value: args[0]}, {Name: "Channels", Value: strings.Join(utils.Map(added, func(s string) string {
				return fmt.Sprintf("<#%v>", s)
			}), " ")}},
		})
	} else {
		s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
			Title:     "❎ Failed to add channels to a cross-post group!",
			Color:     utils.EmbedColor,
			Timestamp: utils.EmbedTimestamp(),
			Thumbnail: &discordgo.MessageEmbedThumbnail{URL: utils.DefaultEmbedImage},
			Fields:    []*discordgo.MessageEmbedField{{Name: "Group name", Value: args[0]}, {Name: "Reason", Value: "No valid channels were found"}},
		})
	}

	return nil
}

func copyGroup(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	if len(args) < 3 {
		return fmt.Errorf("``bt!copy`` requires at least three arguments.\n**Usage:** ``bt!copy <source> <destination> <new parent channel>``")
	}

	user := database.DB.FindUser(m.Author.ID)
	if user == nil {
		s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
			Title:     "❎ Failed to copy a cross-post group!",
			Color:     utils.EmbedColor,
			Timestamp: utils.EmbedTimestamp(),
			Thumbnail: &discordgo.MessageEmbedThumbnail{URL: utils.DefaultEmbedImage},
			Fields:    []*discordgo.MessageEmbedField{{Name: "Reason", Value: "You have no cross-post groups yet."}},
		})
		return nil
	}

	var (
		group    *database.Group
		src      = args[0]
		dest     = args[1]
		exists   bool
		parent   = strings.Trim(args[2], "<#>")
		isParent bool
	)

	if _, err := s.State.Channel(parent); err != nil {
		return fmt.Errorf("unable to find channel ``%v``. Make sure Boe Tea is present on the server and able to read the channel", parent)
	}

	for _, g := range user.ChannelGroups {
		if g.Name == src {
			group = g
		}

		if g.Name == dest {
			exists = true
		}

		if g.Parent == parent {
			isParent = true
		}
	}

	if group == nil {
		s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
			Title:     "❎ Failed to copy a cross-post group!",
			Color:     utils.EmbedColor,
			Timestamp: utils.EmbedTimestamp(),
			Thumbnail: &discordgo.MessageEmbedThumbnail{URL: utils.DefaultEmbedImage},
			Fields:    []*discordgo.MessageEmbedField{{Name: "Reason", Value: "Couldn't find a source group ``" + src + "``"}},
		})
		return nil
	}

	if isParent {
		s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
			Title:     "❎ Failed to copy a cross-post group!",
			Color:     utils.EmbedColor,
			Timestamp: utils.EmbedTimestamp(),
			Thumbnail: &discordgo.MessageEmbedThumbnail{URL: utils.DefaultEmbedImage},
			Fields:    []*discordgo.MessageEmbedField{{Name: "Reason", Value: fmt.Sprintf("Channel <#%v> is already a parent channel", parent)}},
		})
		return nil
	}

	if exists {
		s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
			Title:     "❎ Failed to copy a cross-post group!",
			Color:     utils.EmbedColor,
			Timestamp: utils.EmbedTimestamp(),
			Thumbnail: &discordgo.MessageEmbedThumbnail{URL: utils.DefaultEmbedImage},
			Fields:    []*discordgo.MessageEmbedField{{Name: "Reason", Value: fmt.Sprintf("Group name %v is already taken", dest)}},
		})
		return nil
	}

	new := &database.Group{
		Name:     dest,
		Parent:   parent,
		Children: make([]string, len(group.Children)),
	}

	copy(new.Children, group.Children)
	for ind, c := range new.Children {
		if c == parent {
			new.Children[ind] = group.Parent
		}
	}

	err := database.DB.PushGroup(m.Author.ID, new)
	if err != nil {
		return fmt.Errorf("Fatal database error: %v", err)
	}

	s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
		Title:     "✅ Sucessfully copied a cross-post group!",
		Color:     utils.EmbedColor,
		Timestamp: utils.EmbedTimestamp(),
		Thumbnail: &discordgo.MessageEmbedThumbnail{URL: utils.DefaultEmbedImage},
		Fields: []*discordgo.MessageEmbedField{{Name: "Name", Value: new.Name}, {Name: "Parent", Value: fmt.Sprintf("<#%v>", new.Parent)}, {Name: "Channels", Value: strings.Join(utils.Map(new.Children, func(s string) string {
			return fmt.Sprintf("<#%v>", s)
		}), " ")}},
	})

	return nil
}
