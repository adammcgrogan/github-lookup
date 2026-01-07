package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// --- STYLES ---
var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7D56F4")).MarginBottom(1)
	subStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#767676"))
	infoStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
	repoStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#04B575"))
	descStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#DDDDDD"))
	errStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000")).Bold(true)
)

// --- DATA ---
type CombinedData struct {
	User struct {
		Login, Name, Bio, Location string
		PublicRepos, Followers     int
	}
	Repos []struct {
		Name, Description string
		Stars             int `json:"stargazers_count"`
	}
}

// --- MODEL ---
type model struct {
	input   textinput.Model
	data    *CombinedData
	loading bool
	err     error
	width   int
}

func initialModel() model {
	ti := textinput.New()
	ti.Placeholder, ti.CharLimit, ti.Width = "GitHub username...", 156, 30
	ti.Focus()
	return model{input: ti, width: 80}
}

func (m model) Init() tea.Cmd { return textinput.Blink }

// --- HELPERS & COMMANDS ---
func fetch(url string, target interface{}) error {
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "BubbleTea-CLI")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("API error: %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(target)
}

func searchGithub(user string) tea.Cmd {
	return func() tea.Msg {
		var data CombinedData
		if err := fetch(fmt.Sprintf("https://api.github.com/users/%s", user), &data.User); err != nil {
			return err
		}
		fetch(fmt.Sprintf("https://api.github.com/users/%s/repos?sort=updated&per_page=10", user), &data.Repos)
		return data
	}
}

// --- UPDATE ---
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// 1. GLOBAL QUIT CHECK (Ctrl+C only)
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		if keyMsg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	}

	// 2. Normal Event Handling
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width

	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			// If we have results, Enter resets the search
			if m.data != nil {
				m.data = nil
				m.input.SetValue("")
				return m, nil
			}
			// Otherwise, perform search
			m.loading, m.err = true, nil
			return m, searchGithub(m.input.Value())
		}

	case CombinedData:
		m.loading, m.data = false, &msg

	case error:
		m.loading, m.err = false, msg
	}

	// 3. Text Input Handling
	// Only update text input if we are in search mode (data is nil)
	if m.data == nil {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}

	return m, nil
}

// --- VIEW ---
func (m model) View() string {
	if m.loading {
		return fmt.Sprintf("\nSearching for %s...\n", m.input.Value())
	}
	if m.err != nil {
		return fmt.Sprintf("\n%s\n(Enter to try again)\n", errStyle.Render(m.err.Error()))
	}

	// Result View
	if d := m.data; d != nil {
		kv := func(k, v string) string {
			return fmt.Sprintf("%s %s\n", subStyle.Render(k+":"), infoStyle.Render(v))
		}
		var b strings.Builder
		b.WriteString("\n" + titleStyle.Render("USER PROFILE") + "\n")
		b.WriteString(kv("Name", d.User.Name))
		b.WriteString(kv("Github", "@"+d.User.Login))
		if d.User.Location != "" {
			b.WriteString(kv("Location", d.User.Location))
		}
		b.WriteString(kv("Stats", fmt.Sprintf("%d Repos, %d Followers", d.User.PublicRepos, d.User.Followers)))

		if d.User.Bio != "" {
			b.WriteString(subStyle.Render("Bio:") + "\n")
			b.WriteString(lipgloss.NewStyle().Width(m.width-4).Render(d.User.Bio) + "\n")
		}

		b.WriteString("\n" + titleStyle.Render("LATEST REPOSITORIES") + "\n")
		for _, r := range d.Repos {
			b.WriteString(repoStyle.Render(fmt.Sprintf("%s (★ %d)", r.Name, r.Stars)) + "\n")
			desc := r.Description
			if desc == "" {
				desc = "No description."
			}
			b.WriteString(descStyle.Width(m.width-4).Render(desc) + "\n\n")
		}
		b.WriteString(subStyle.Render("(Enter to search again, Ctrl+C to quit)"))
		return b.String()
	}

	// Search Input View
	return fmt.Sprintf("\n%s\n\n%s\n\n(Enter to search, Ctrl+C to quit)\n",
		titleStyle.Render("GITHUB SEARCH"),
		m.input.View())
}

func main() {
	if _, err := tea.NewProgram(initialModel()).Run(); err != nil {
		fmt.Println("Error:", err)
	}
}
