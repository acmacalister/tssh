package ui

import (
	"github.com/acmacalister/tssh"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	bullet   = "•"
	ellipsis = "…"
)

var (
	appStyle = lipgloss.NewStyle().Padding(1, 2)

	statusMessageStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#04B575", Dark: "#04B575"}).
				Render

	delegateKeys = &listKeyMap{
		choose: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "choose"),
		),
	}
)

type (
	listKeyMap struct {
		choose key.Binding
	}

	ListItem struct {
		Name   string
		Info   string
		Action tssh.Action
	}
)

func (i ListItem) Title() string       { return i.Name }
func (i ListItem) Description() string { return i.Info }
func (i ListItem) FilterValue() string { return i.Name }

type ListModel struct {
	list list.Model
}

func (m *ListModel) Init() tea.Cmd {
	return nil
}

func (m *ListModel) Update(msg tea.Msg) (*ListModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m.handleWindow(msg)
	case tea.KeyMsg:
		return m.handleKeyPress(msg)
	default:
		return m.handleDefault(msg)
	}
}

func (m *ListModel) View() string {
	return m.list.View()
}

func (m *ListModel) handleWindow(msg tea.WindowSizeMsg) (*ListModel, tea.Cmd) {
	var cmd tea.Cmd
	h, v := appStyle.GetFrameSize()
	m.list.SetSize(msg.Width-h, msg.Height-v)
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m *ListModel) handleKeyPress(msg tea.KeyMsg) (*ListModel, tea.Cmd) {
	var cmd tea.Cmd
	switch {
	case key.Matches(msg, delegateKeys.choose):
		return m.handleChoose(msg)
	default:
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	}
}

func (m *ListModel) handleChoose(msg tea.KeyMsg) (*ListModel, tea.Cmd) {
	var cmd tea.Cmd
	i, ok := m.list.SelectedItem().(ListItem)
	if !ok {
		return m, nil
	}
	m.list, cmd = m.list.Update(msg)
	return m, tea.Batch(cmd, m.list.NewStatusMessage(statusMessageStyle("You chose "+i.Title())), m.selectedCmd)
}

func (m *ListModel) handleDefault(msg tea.Msg) (*ListModel, tea.Cmd) {
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m *ListModel) selectedCmd() tea.Msg {
	i, ok := m.list.SelectedItem().(ListItem)
	if !ok {
		return nil
	}

	return i
}

func (m *ListModel) SetItems(items ...ListItem) tea.Cmd {
	listItems := make([]list.Item, 0, len(items))
	for _, item := range items {
		listItems = append(listItems, ListItem{Name: item.Name, Info: item.Info, Action: item.Action})
	}

	return m.list.SetItems(listItems)
}

func itemStyles() (s list.DefaultItemStyles) {
	s.NormalTitle = lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#1a1a1a", Dark: "#dddddd"}).
		Padding(0, 0, 0, 2)

	s.NormalDesc = s.NormalTitle.Copy().
		Foreground(lipgloss.AdaptiveColor{Light: "#A49FA5", Dark: "#777777"})

	s.SelectedTitle = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(lipgloss.AdaptiveColor{Light: "69", Dark: "69"}).
		Foreground(lipgloss.AdaptiveColor{Light: "69", Dark: "69"}).
		Padding(0, 0, 0, 1)

	s.SelectedDesc = s.SelectedTitle.Copy().
		Foreground(lipgloss.AdaptiveColor{Light: "69", Dark: "69"})

	s.DimmedTitle = lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#A49FA5", Dark: "#777777"}).
		Padding(0, 0, 0, 2)

	s.DimmedDesc = s.DimmedTitle.Copy().
		Foreground(lipgloss.AdaptiveColor{Light: "#C2B8C2", Dark: "#4D4D4D"})

	s.FilterMatch = lipgloss.NewStyle().Underline(true)

	return s
}

func styles() (s list.Styles) {
	verySubduedColor := lipgloss.AdaptiveColor{Light: "#DDDADA", Dark: "#3C3C3C"}
	subduedColor := lipgloss.AdaptiveColor{Light: "#9B9B9B", Dark: "#5C5C5C"}

	s.TitleBar = lipgloss.NewStyle().Padding(0, 0, 1, 2)

	s.Title = lipgloss.NewStyle().
		//Background(lipgloss.Color("62")).
		Foreground(lipgloss.Color("230")).
		Padding(0, 1)

	s.Spinner = lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#8E8E8E", Dark: "#747373"})

	s.FilterPrompt = lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "69", Dark: "69"})

	s.FilterCursor = lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "69", Dark: "69"})

	s.DefaultFilterCharacterMatch = lipgloss.NewStyle().Underline(true)

	s.StatusBar = lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#A49FA5", Dark: "#777777"}).
		Padding(0, 0, 1, 2)

	s.StatusEmpty = lipgloss.NewStyle().Foreground(subduedColor)

	s.StatusBarActiveFilter = lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#1a1a1a", Dark: "#dddddd"})

	s.StatusBarFilterCount = lipgloss.NewStyle().Foreground(verySubduedColor)

	s.NoItems = lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#909090", Dark: "#626262"})

	s.ArabicPagination = lipgloss.NewStyle().Foreground(subduedColor)

	s.PaginationStyle = lipgloss.NewStyle().PaddingLeft(2) //nolint:gomnd

	s.HelpStyle = lipgloss.NewStyle().Padding(1, 0, 0, 2)

	s.ActivePaginationDot = lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#847A85", Dark: "#979797"}).
		SetString(bullet)

	s.InactivePaginationDot = lipgloss.NewStyle().
		Foreground(verySubduedColor).
		SetString(bullet)

	s.DividerDot = lipgloss.NewStyle().
		Foreground(verySubduedColor).
		SetString(" " + bullet + " ")

	return s
}

func newDelegate() list.DefaultDelegate {
	d := list.DefaultDelegate{
		ShowDescription: true,
		Styles:          itemStyles(),
	}
	d.SetHeight(2)
	d.SetSpacing(1)
	return d
}

func NewList(title string, items ...ListItem) *ListModel {
	listItems := make([]list.Item, 0)
	if len(items) > 0 {
		listItems = make([]list.Item, 0, len(items))
		for _, item := range items {
			listItems = append(listItems, ListItem{Name: item.Name, Info: item.Info, Action: item.Action})
		}
	}

	d := newDelegate()
	l := list.New(listItems, d, 40, 20)
	l.Title = title
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.ShowFilter()
	l.Styles = styles()
	return &ListModel{list: l}
}
