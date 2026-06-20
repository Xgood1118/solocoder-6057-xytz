package subscriptionlist

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/xdagiz/xytz/internal/config"
	"github.com/xdagiz/xytz/internal/styles"
	"github.com/xdagiz/xytz/internal/tui/models"
	"github.com/xdagiz/xytz/internal/types"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	zone "github.com/lrstanley/bubblezone/v2"
)

type Model struct {
	Width         int
	Height        int
	List          list.Model
	Subscriptions []types.Subscription
	ErrMsg        string
	prefix        string
	Renaming      bool
	RenameInput   textinput.Model
	renameID      string
}

type subscriptionListItem struct {
	sub types.Subscription
}

func (i subscriptionListItem) Title() string {
	name := i.sub.DisplayName
	if name == "" {
		name = i.sub.OriginalID
	}

	prefix := ""
	if i.sub.IsPaused {
		prefix = "⏸ "
	}

	return prefix + name
}

func (i subscriptionListItem) Description() string {
	typeLabel := "Channel"
	if i.sub.Type == types.SubscriptionTypePlaylist {
		typeLabel = "Playlist"
	}

	lastFetched := "never"
	if !i.sub.LastFetched.IsZero() {
		lastFetched = formatTime(i.sub.LastFetched)
	}

	return fmt.Sprintf("%s • last fetched: %s", typeLabel, lastFetched)
}

func (i subscriptionListItem) FilterValue() string {
	name := i.sub.DisplayName
	if name == "" {
		name = i.sub.OriginalID
	}
	return name
}

func formatTime(t time.Time) string {
	now := time.Now()
	diff := now.Sub(t)

	if diff < time.Minute {
		return "just now"
	}
	if diff < time.Hour {
		return fmt.Sprintf("%d min ago", int(diff.Minutes()))
	}
	if diff < 24*time.Hour {
		return fmt.Sprintf("%d hours ago", int(diff.Hours()))
	}
	if diff < 7*24*time.Hour {
		return fmt.Sprintf("%d days ago", int(diff.Hours()/24))
	}
	return t.Format("2006-01-02")
}

func NewModel() Model {
	s := textinput.DefaultStyles(true)
	prefix := zone.NewPrefix()
	dl := styles.NewClickableDelegate(prefix, styles.NewListDelegate())
	li := list.New([]list.Item{}, dl, 0, 0)
	li.SetShowStatusBar(false)
	li.SetShowTitle(false)
	li.SetShowHelp(false)
	li.SetStatusBarItemName("subscription", "subscriptions")
	s.Cursor.Color = styles.AccentPrimaryColor
	s.Focused.Prompt = lipgloss.NewStyle().Foreground(styles.TextPrimaryColor)
	li.FilterInput.SetStyles(s)

	renameInput := textinput.New()
	renameInput.Placeholder = "Enter new name..."
	renameInput.CharLimit = 100
	renameInput.Width = 50

	return Model{
		List:          li,
		Subscriptions: []types.Subscription{},
		ErrMsg:        "",
		prefix:        prefix,
		Renaming:      false,
		RenameInput:   renameInput,
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
		unpausedCount := 0
		for _, sub := range m.Subscriptions {
			if !sub.IsPaused {
				unpausedCount++
			}
		}
		headerText = fmt.Sprintf("Subscriptions (%d active / %d total)", unpausedCount, len(m.Subscriptions))
		headerStyle = styles.SectionHeaderStyle
	}

	s.WriteString(headerStyle.Render(headerText))
	s.WriteRune('\n')

	if m.Renaming {
		s.WriteString(styles.SectionHeaderStyle.Render("Rename subscription:"))
		s.WriteRune('\n')
		s.WriteString(m.RenameInput.View())
		s.WriteRune('\n')
		s.WriteString(styles.MutedStyle.Render("Enter to confirm, Esc to cancel"))
		s.WriteRune('\n')
	}

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

	if m.Renaming {
		switch msg := msg.(type) {
		case tea.KeyPressMsg:
			switch {
			case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
				newName := strings.TrimSpace(m.RenameInput.Value())
				if newName != "" {
					cmd = func() tea.Msg {
						return types.RenameSubscriptionMsg{
							ID:          m.renameID,
							DisplayName: newName,
						}
					}
				}
				m.Renaming = false
				m.renameID = ""
				m.RenameInput.SetValue("")
				return m, cmd

			case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
				m.Renaming = false
				m.renameID = ""
				m.RenameInput.SetValue("")
				return m, nil
			}
		}
		m.RenameInput, cmd = m.RenameInput.Update(msg)
		return m, cmd
	}

	switch msg := msg.(type) {
	case tea.MouseReleaseMsg:
		if msg.Button == tea.MouseLeft && !m.List.SettingFilter() {
			for i := range m.List.Items() {
				if zone.Get(m.prefix + strconv.Itoa(i)).InBounds(msg) {
					if i != m.List.Index() {
						m.List.Select(i)
						return m, nil
					}

					cmd = m.openSelectedSubscription()
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
		case key.Matches(msg, models.SubscriptionListModelKeys.Enter):
			if m.List.FilterState() == list.Filtering {
				m.List.SetFilterState(list.FilterApplied)
				return m, nil
			}

			if len(m.List.Items()) == 0 {
				return m, nil
			}

			cmd = m.openSelectedSubscription()
			return m, cmd

		case key.Matches(msg, models.SubscriptionListModelKeys.TogglePause):
			if !m.List.SettingFilter() && len(m.Subscriptions) > 0 {
				if sub, ok := m.SelectedSubscription(); ok {
					cmd = func() tea.Msg {
						return types.ToggleSubscriptionPauseMsg{ID: sub.ID}
					}
				}
			}

		case key.Matches(msg, models.SubscriptionListModelKeys.Delete):
			if !m.List.SettingFilter() && len(m.Subscriptions) > 0 {
				if sub, ok := m.SelectedSubscription(); ok {
					cmd = func() tea.Msg {
						return types.RemoveSubscriptionMsg{ID: sub.ID}
					}
				}
			}

		case key.Matches(msg, models.SubscriptionListModelKeys.Rename):
			if !m.List.SettingFilter() && len(m.Subscriptions) > 0 {
				if sub, ok := m.SelectedSubscription(); ok {
					m.Renaming = true
					m.renameID = sub.ID
					currentName := sub.DisplayName
					if currentName == "" {
						currentName = sub.OriginalID
					}
					m.RenameInput.SetValue(currentName)
					m.RenameInput.CursorEnd()
					return m, textinput.Blink
				}
			}

		case key.Matches(msg, models.SubscriptionListModelKeys.Refresh):
			if !m.List.SettingFilter() {
				cmd = func() tea.Msg {
					return types.FetchSubscriptionsMsg{Count: 0}
				}
			}
		}
	}

	m.List, listCmd = m.List.Update(msg)
	return m, tea.Batch(cmd, listCmd)
}

func (m Model) SelectedSubscription() (types.Subscription, bool) {
	selectedItem := m.List.SelectedItem()
	if item, ok := selectedItem.(subscriptionListItem); ok {
		return item.sub, true
	}

	return types.Subscription{}, false
}

func (m *Model) SetItems(subs []types.Subscription) {
	m.Subscriptions = subs
	items := make([]list.Item, len(subs))
	for i, sub := range subs {
		items[i] = subscriptionListItem{sub: sub}
	}
	m.List.SetItems(items)
}

func (m Model) openSelectedSubscription() tea.Cmd {
	sub, ok := m.SelectedSubscription()
	if !ok || sub.ID == "" {
		return nil
	}

	switch sub.Type {
	case types.SubscriptionTypeChannel:
		return func() tea.Msg {
			return types.ChannelSelectedMsg{
				Channel: types.ChannelItem{
					ID:   sub.OriginalID,
					Name: sub.DisplayName,
				},
			}
		}
	case types.SubscriptionTypePlaylist:
		return func() tea.Msg {
			return types.PlaylistSelectedMsg{
				Playlist: types.PlaylistItem{
					ID:        sub.OriginalID,
					TitleText: sub.DisplayName,
					URL:       sub.URL,
				},
			}
		}
	}
	return nil
}
