package ui

import (
	"fmt"

	"github.com/acmacalister/tssh"
	components "github.com/acmacalister/tssh/ui/components"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	sshclient "github.com/helloyi/go-sshclient"
	"github.com/tailscale/tailscale-client-go/tailscale"
	"golang.org/x/crypto/ssh"
)

type (
	success interface {
		[]tailscale.Device
	}

	Result[T success] struct {
		Success T
		Error   error
	}

	mainModel struct {
		loading    spinner.Model
		deviceList *components.ListModel
		mainMenu   *components.ListModel
		state      state
		err        error
		ts         tssh.TailscaleService
	}

	state int
)

const (
	stateMenu state = iota
	stateLoading
	stateFailure
	stateDevice
)

var (
	textStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("69")).Render
	spinnerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("69"))
)

func (m *mainModel) Init() tea.Cmd {
	return tea.Batch(m.loading.Tick)
}

func (m *mainModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m.handleWindow(msg)
	case tea.KeyMsg:
		return m.handleKeyPress(msg)
	case spinner.TickMsg:
		return m.handleTick(msg)
	case Result[[]tailscale.Device]:
		return m.handleResult(msg)
	case components.ListItem:
		return m.handleAction(msg)
	default:
		return m.handleDefault(msg)
	}
}

func (m *mainModel) handleKeyPress(msg tea.KeyMsg) (*mainModel, tea.Cmd) {
	var cmd tea.Cmd
	keypress := msg.String()
	switch keypress {
	case "ctrl+c", "q":
		return m, tea.Quit
	}

	if m.state == stateMenu {
		m.mainMenu, cmd = m.mainMenu.Update(msg)
		return m, cmd
	}

	if m.state == stateDevice {
		m.deviceList, cmd = m.deviceList.Update(msg)
		return m, cmd
	}

	return m, cmd
}

func (m *mainModel) handleWindow(msg tea.WindowSizeMsg) (*mainModel, tea.Cmd) {
	var cmd tea.Cmd
	m.deviceList, cmd = m.deviceList.Update(msg)
	return m, cmd
}

func (m *mainModel) handleTick(msg spinner.TickMsg) (*mainModel, tea.Cmd) {
	var cmd tea.Cmd
	m.loading, cmd = m.loading.Update(msg)
	return m, cmd
}

func (m *mainModel) handleResult(result Result[[]tailscale.Device]) (*mainModel, tea.Cmd) {
	var cmd tea.Cmd
	if result.Error != nil {
		m.state = stateFailure
		m.err = result.Error
		return m, cmd
	}

	listItems := make([]components.ListItem, 0, len(result.Success))
	for _, device := range result.Success {
		for _, tag := range device.Tags {
			if tag == "tag:e2e" {
				listItems = append(listItems, components.ListItem{Name: device.Hostname, Info: device.User, Action: tssh.ActionDeviceSSH})
			}
		}
	}
	cmd = m.deviceList.SetItems(listItems...)
	m.state = stateDevice

	return m, cmd
}

func (m *mainModel) handleAction(item components.ListItem) (*mainModel, tea.Cmd) {
	switch item.Action {
	case tssh.ActionSSH:
		m.state = stateLoading
		go m.fetchDevices()
	case tssh.ActionDeviceSSH:
		m.state = stateLoading
		if err := m.sshDevice(item.Name); err != nil {
			m.err = err
			m.state = stateFailure
			return m, nil
		}
		m.state = stateMenu
		m.Update(nil)
	}
	return m, nil
}

func (m *mainModel) handleDefault(msg tea.Msg) (*mainModel, tea.Cmd) {
	var cmd tea.Cmd
	switch m.state {
	case stateMenu:
		m.mainMenu, cmd = m.mainMenu.Update(msg)
	case stateDevice:
		m.deviceList, cmd = m.deviceList.Update(msg)
	case stateLoading:
		m.loading, cmd = m.loading.Update(msg)
	}

	return m, cmd
}

func (m mainModel) View() string {
	switch m.state {
	case stateMenu:
		return m.mainMenu.View()
	case stateLoading:
		return lipgloss.JoinHorizontal(lipgloss.Top, m.loading.View(), textStyle(" Fetching Devices..."))
	case stateDevice:
		return m.deviceList.View()
	case stateFailure:
		return lipgloss.JoinHorizontal(lipgloss.Top, textStyle(fmt.Sprintf("Failure: %s", m.err.Error())))

	}

	return ""
}

func (m *mainModel) fetchDevices() {
	devices, err := m.ts.Devices()
	m.Update(Result[[]tailscale.Device]{Success: devices, Error: err})
}

func (m *mainModel) sshDevice(hostname string) error {
	config := &ssh.ClientConfig{
		User:            "ubuntu",
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	client, err := sshclient.Dial("tcp", hostname+":22", config)
	if err != nil {
		return err
	}
	defer client.Close()

	termConfig := &sshclient.TerminalConfig{
		Term:   "xterm",
		Height: 40,
		Weight: 80,
	}

	if err := client.Terminal(termConfig).Start(); err != nil {
		return err
	}

	return nil
}

func New(ts tssh.TailscaleService) error {
	mm := components.NewList("What do you want to do?", components.ListItem{Name: "SSH to Tailscale Device", Info: "Jump on a device", Action: tssh.ActionSSH})

	m := mainModel{state: stateMenu,
		mainMenu:   mm,
		deviceList: components.NewList("Devices"),
		loading:    spinner.New(spinner.WithSpinner(spinner.Points), spinner.WithStyle(spinnerStyle)),
		ts:         ts}

	p := tea.NewProgram(&m)
	if _, err := p.Run(); err != nil {
		return err
	}

	return nil
}
