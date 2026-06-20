package updates

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/xdagiz/xytz/internal/config"
	"github.com/xdagiz/xytz/internal/styles"
	"github.com/xdagiz/xytz/internal/tui/models"
	"github.com/xdagiz/xytz/internal/types"
	"github.com/xdagiz/xytz/internal/utils"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	zone "github.com/lrstanley/bubblezone/v2"
)

type Model struct {
	Width        int
	Height       int
	List         list.Model
	Videos       []types.SubscriptionVideo
	ErrMsg       string
	prefix       string
	UnreadCount  int
}

type updateListItem struct {
	video types.SubscriptionVideo
}

func (i updateListItem) Title() string {
	title := i.video.VideoTitle
	if !i.video.IsRead {
		title = "● " + title
	} else {
		title = "  " + title
	}
	return title
}

func (i updateListItem) Description() string {
	channel := i.video.SubscriptionName
	if channel == "" {
		channel = i.video.Channel
	}
	return fmt.Sprintf("%s • %s", channel, formatUploadDate(i.video.UploadDate))
}

func (i updateListItem) FilterValue() string {
	return i.video.VideoTitle + " " + i.video.SubscriptionName
}

func formatUploadDate(dateStr string) string {
	if dateStr == "" {
		return ""
	}
	return dateStr
}

func NewModel() Model {
	s := textinput.DefaultStyles(true)
	prefix := zone.NewPrefix()
	dl := styles.NewClickableDelegate(prefix, styles.NewListDelegate())
	li := list.New([]list.Item{}, dl, 0, 0)
	li.SetShowStatusBar(false)
	li.SetShowTitle(false)
	li.SetShowHelp(false)
	li.SetStatusBarItemName("video", "videos")
	s.Cursor.Color = styles.AccentPrimaryColor
	s.Focused.Prompt = lipgloss.NewStyle().Foreground(styles.TextPrimaryColor)
	li.FilterInput.SetStyles(s)

	return Model{
		List:       li,
		Videos:     []types.SubscriptionVideo{},
		ErrMsg:     "",
		prefix:     prefix,
		UnreadCount: 0,
	}
}

func (m *Model) ApplyTheme() {
	m.List.SetDelegate(styles.NewClickableDelegate(m.prefix, styles.NewListDelegate()))
	s := textinput.DefaultStyles(true)
	s.Focused.Prompt = lipgloss.NewStyle().Foreground(styles.TextPrimaryColor)
	s.Cursor.Color = styles.AccentPrimaryColor
	m.List.FilterInput.SetStyles(s)
}

func (m *Model) ApplyConfig(cfg *config.Config) {
	if cfg.ListCompactMode {
		m.List.SetDelegate(styles.NewClickableDelegate(m.prefix, styles.NewCompactDelegate()))
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) View() string {
	var (
		s           strings.Builder
		headerText  string
		headerStyle lipgloss.Style
	)

	if m.ErrMsg != "" {
		headerStyle = styles.ErrorMessageStyle.PaddingTop(1)
		headerText = fmt.Sprintf("Error: %s", m.ErrMsg)
	} else {
		headerText = fmt.Sprintf("Updates (%d unread / %d total)", m.UnreadCount, len(m.Videos))
		headerStyle = styles.SectionHeaderStyle
	}

	s.WriteString(headerStyle.Render(headerText))
	s.WriteRune('\n')
	s.WriteString(styles.ListContainer.Render(m.List.View()))

	return s.String()
}

func (m Model) HandleResize(w, h int) Model {
	m.Width = w
	m.Height = h
	m.List.SetSize(w, h-5)
	return m
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var (
		cmd     tea.Cmd
		listCmd tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.MouseReleaseMsg:
		if msg.Button == tea.MouseLeft && !m.List.SettingFilter() {
			for i := range m.List.Items() {
				if zone.Get(m.prefix + strconv.Itoa(i)).InBounds(msg) {
					if i != m.List.Index() {
						m.List.Select(i)
						return m, nil
					}

					video, ok := m.SelectedVideo()
					if ok && video.ID != "" {
						cmd = func() tea.Msg {
							return types.StartFormatMsg{
								URL:           utils.ResolveVideoItemURL(video.VideoItem),
								SelectedVideo: video.VideoItem,
							}
						}
					}

					return m, cmd
				}
			}
		}

	case tea.MouseWheelMsg:
		switch msg.Button {
		case tea.MouseWheelUp:
			m.List.CursorUp()
		case tea.MouseWheelDown:
			m.List.CursorDown()
		}
		return m, nil

	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, models.UpdatesModelKeys.Enter):
			if m.List.FilterState() == list.Filtering {
				m.List.SetFilterState(list.FilterApplied)
				return m, nil
			}

			if len(m.List.Items()) == 0 {
				return m, nil
			}

			if video, ok := m.SelectedVideo(); ok {
				cmd = func() tea.Msg {
					return types.StartFormatMsg{
						URL:           utils.ResolveVideoItemURL(video.VideoItem),
						SelectedVideo: video.VideoItem,
					}
				}
				return m, cmd
			}

		case key.Matches(msg, models.UpdatesModelKeys.Download):
			if !m.List.SettingFilter() && len(m.Videos) > 0 {
				if video, ok := m.SelectedVideo(); ok {
					cmd = func() tea.Msg {
						return types.MarkVideoReadMsg{VideoID: video.ID}
					}
					cmd2 := func() tea.Msg {
						return types.StartFormatMsg{
							URL:           utils.ResolveVideoItemURL(video.VideoItem),
							SelectedVideo: video.VideoItem,
						}
					}
					return m, tea.Batch(cmd, cmd2)
				}
			}

		case key.Matches(msg, models.UpdatesModelKeys.BatchDownload):
			if !m.List.SettingFilter() && m.UnreadCount > 0 {
				cmd = func() tea.Msg {
					return types.UpdatesBatchDownloadMsg{}
				}
				return m, cmd
			}

		case key.Matches(msg, models.UpdatesModelKeys.ToggleRead):
			if !m.List.SettingFilter() && len(m.Videos) > 0 {
				if video, ok := m.SelectedVideo(); ok {
					cmd = func() tea.Msg {
						return types.MarkVideoReadMsg{VideoID: video.ID}
					}
					return m, cmd
				}
			}

		case key.Matches(msg, models.UpdatesModelKeys.MarkAllRead):
			if !m.List.SettingFilter() && m.UnreadCount > 0 {
				cmd = func() tea.Msg {
					return types.MarkAllVideosReadMsg{SubscriptionID: ""}
				}
				return m, cmd
			}

		case key.Matches(msg, models.UpdatesModelKeys.GoToChannel):
			if !m.List.SettingFilter() && len(m.Videos) > 0 {
				if video, ok := m.SelectedVideo(); ok {
					channelName := video.Channel
					if channelName == "" && video.ChannelURL != "" {
						channelName = video.ChannelURL
					}
					if channelName != "" {
						cmd = func() tea.Msg {
							return types.StartChannelURLMsg{
								URL:         video.ChannelURL,
								ChannelName: channelName,
							}
						}
						return m, cmd
					}
				}
			}
		}
	}

	m.List, listCmd = m.List.Update(msg)
	return m, tea.Batch(cmd, listCmd)
}

func (m Model) SelectedVideo() (types.SubscriptionVideo, bool) {
	selectedItem := m.List.SelectedItem()
	if item, ok := selectedItem.(updateListItem); ok {
		return item.video, true
	}

	return types.SubscriptionVideo{}, false
}

func (m *Model) SetItems(videos []types.SubscriptionVideo) {
	m.Videos = videos

	sort.SliceStable(videos, func(i, j int) bool {
		return videos[i].UploadDate > videos[j].UploadDate
	})

	unread := 0
	for _, v := range videos {
		if !v.IsRead {
			unread++
		}
	}
	m.UnreadCount = unread

	items := make([]list.Item, len(videos))
	for i, v := range videos {
		items[i] = updateListItem{video: v}
	}
	m.List.SetItems(items)
}

func (m *Model) GetUnreadVideos() []types.SubscriptionVideo {
	var unread []types.SubscriptionVideo
	for _, v := range m.Videos {
		if !v.IsRead {
			unread = append(unread, v)
		}
	}
	return unread
}
