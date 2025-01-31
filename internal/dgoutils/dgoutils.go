package dgoutils

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

var (
	widgetControls = map[string]bool{"⏮": true, "⏪": true, "◀": true, "⏹": true, "▶": true, "⏩": true, "⏭": true}
	ErrNotRange    = errors.New("not range")
	ErrRangeSyntax = errors.New("range low is higher than range high")
)

func MemberHasPermission(s *discordgo.Session, guildID string, userID string, permission int64) (bool, error) {
	member, err := s.State.Member(guildID, userID)
	if err != nil {
		if member, err = s.GuildMember(guildID, userID); err != nil {
			return false, err
		}
	}

	for _, roleID := range member.Roles {
		role, err := s.State.Role(guildID, roleID)
		if err != nil {
			return false, err
		}

		if role.Permissions&permission != 0 {
			return true, nil
		}
	}

	g, err := s.Guild(guildID)
	if err != nil {
		return false, fmt.Errorf("failed to get guild: %w", err)
	}

	if g.OwnerID == userID {
		return true, nil
	}

	return false, nil
}

type Range struct {
	Low  int
	High int
}

func NewRange(s string) (*Range, error) {
	hyphen := strings.IndexByte(s, '-')
	if hyphen == -1 {
		return nil, ErrNotRange
	}
	lowStr := s[:hyphen]
	highStr := s[hyphen+1:]

	low, err := strconv.Atoi(lowStr)
	if err != nil {
		return nil, err
	}

	high, err := strconv.Atoi(highStr)
	if err != nil {
		return nil, err
	}

	if low > high {
		return nil, ErrRangeSyntax
	}

	return &Range{
		Low:  low,
		High: high,
	}, nil
}

func (r *Range) Array() []int {
	arr := make([]int, 0)
	for i := r.Low; i <= r.High; i++ {
		arr = append(arr, i)
	}

	return arr
}

func (r *Range) Map() map[int]struct{} {
	m := make(map[int]struct{}, 0)
	for i := r.Low; i <= r.High; i++ {
		m[i] = struct{}{}
	}
	return m
}

//EmbedWidget is an interactive DiscordGo widget interface
type EmbedWidget struct {
	s           *discordgo.Session
	m           *discordgo.Message
	author      string
	currentPage int
	Pages       []*discordgo.MessageEmbed

	callback func(action WidgetAction, page int) error
}

type WidgetAction int

const (
	WidgetActionFirstPage WidgetAction = iota
	WidgetActionFiveDown
	WidgetActionPreviousPage
	WidgetActionStop
	WidgetActionNextPage
	WidgetActionFiveUp
	WidgetActionLastPage
)

var (
	actionMap = map[string]WidgetAction{
		"⏮": WidgetActionFirstPage,
		"⏪": WidgetActionFiveDown,
		"◀": WidgetActionPreviousPage,
		"⏹": WidgetActionStop,
		"▶": WidgetActionNextPage,
		"⏩": WidgetActionFiveUp,
		"⏭": WidgetActionLastPage,
	}
)

func (a WidgetAction) String() string {
	return []string{"⏮", "⏪", "◀", "⏹", "▶", "⏩", "⏭"}[a]
}

func NewWidget(s *discordgo.Session, author string, embeds []*discordgo.MessageEmbed) *EmbedWidget {
	return &EmbedWidget{s, nil, author, 0, embeds, nil}
}

func (w *EmbedWidget) WithCallback(fn func(WidgetAction, int) error) {
	w.callback = fn
}

func (w *EmbedWidget) Start(channelID string) error {
	if len(w.Pages) == 0 {
		return nil
	}

	m, err := w.s.ChannelMessageSendEmbed(channelID, w.Pages[0])
	if err != nil {
		return err
	}
	w.m = m

	if w.len() == 1 {
		return nil
	}

	if w.len() > 5 {
		w.s.MessageReactionAdd(m.ChannelID, m.ID, "⏮")
		w.s.MessageReactionAdd(m.ChannelID, m.ID, "⏪")
	}

	w.s.MessageReactionAdd(m.ChannelID, m.ID, "◀")
	w.s.MessageReactionAdd(m.ChannelID, m.ID, "⏹")
	w.s.MessageReactionAdd(m.ChannelID, m.ID, "▶")

	if w.len() > 5 {
		w.s.MessageReactionAdd(m.ChannelID, m.ID, "⏩")
		w.s.MessageReactionAdd(m.ChannelID, m.ID, "⏭")
	}

	var reaction *discordgo.MessageReaction
	for {
		select {
		case k := <-nextMessageReactionAdd(w.s):
			reaction = k.MessageReaction
		case <-time.After(2 * time.Minute):
			return nil
		}

		r := reaction.Emoji.APIName()
		_, ok := widgetControls[r]
		if !ok {
			continue
		}

		if reaction.MessageID != w.m.ID || w.s.State.User.ID == reaction.UserID || reaction.UserID != w.author {
			continue
		}

		switch reaction.Emoji.APIName() {
		case "⏮":
			w.firstPage()
		case "⏪":
			w.fivePagesDown()
		case "◀":
			w.pageDown()
		case "▶":
			w.pageUp()
		case "⏩":
			w.fivePagesUp()
		case "⏭":
			w.lastPage()
		case "⏹":
			w.s.MessageReactionsRemoveAll(w.m.ChannelID, w.m.ID)
			return nil
		}

		if w.callback != nil {
			err := w.callback(actionMap[reaction.Emoji.APIName()], w.currentPage)
			if err != nil {
				return err
			}
		}

		_, err := w.s.ChannelMessageEditEmbed(w.m.ChannelID, w.m.ID, w.Pages[w.currentPage])
		if err != nil {
			return err
		}

		w.s.MessageReactionRemove(w.m.ChannelID, w.m.ID, reaction.Emoji.APIName(), reaction.UserID)
	}
}

func (w *EmbedWidget) pageUp() {
	if w.currentPage == w.len()-1 || w.len() <= 1 {
		return
	}

	w.currentPage++
}

func (w *EmbedWidget) fivePagesUp() {
	if w.currentPage == w.len()-1 || w.len() <= 1 {
		return
	}

	w.currentPage += 5
	if w.currentPage >= w.len() {
		w.currentPage = w.len() - 1
	}
}

func (w *EmbedWidget) pageDown() {
	if w.currentPage == 0 || w.len() <= 1 {
		return
	}

	w.currentPage--
}

func (w *EmbedWidget) fivePagesDown() {
	if w.currentPage == 0 || w.len() <= 1 {
		return
	}

	w.currentPage -= 5
	if w.currentPage < 0 {
		w.currentPage = 0
	}
}

func (w *EmbedWidget) lastPage() {
	if w.currentPage == w.len()-1 || w.len() <= 1 {
		return
	}

	w.currentPage = w.len() - 1
}

func (w *EmbedWidget) firstPage() {
	if w.currentPage == 0 || w.len() <= 1 {
		return
	}

	w.currentPage = 0
}

func (w *EmbedWidget) len() int {
	return len(w.Pages)
}

func nextMessageReactionAdd(s *discordgo.Session) chan *discordgo.MessageReactionAdd {
	out := make(chan *discordgo.MessageReactionAdd)
	s.AddHandlerOnce(func(_ *discordgo.Session, e *discordgo.MessageReactionAdd) {
		out <- e
	})
	return out
}
