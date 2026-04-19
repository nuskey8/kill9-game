package game

import (
	"fmt"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var cmdNames = []string{
	// dummy commands
	"go run main.go",
	"go test ./...",
	"python worker.py",
	"node api.js",
	"ffmpeg -i input.mp4",
	"yt-dlp stream-url",
	"cargo build --release",
	"java -jar app.jar",
	"ruby crawl.rb",
	"php artisan queue:work",
	"bun dev",
	"deno run bot.ts",
	"tail -f server.log",
	"grep -R TODO src",
	"sleep 9999",
	"yes > /dev/null",
	"nginx -g daemon off;",
	"redis-server",
	"postgres -D data",
	"npm install",

	// Oh... (DO NOT try this on a real terminal)
	"rm -rf / --no-preserve-root",

	// fork bombs! (DO NOT try these on a real terminal)
	":(){ :|:& };:",
	"python -c \"while True: __import__('os').fork()\"",
	"perl -e \"fork while 1\"",

	// :)
	"cat /dev/urandom | hexdump -C",
	"sl",
	"fortune",
	"cowsay \"Hello, World!\"",
	"telnet towel.blinkenlights.nl",
	"cmatrix",
	"nuskey8 \"Have fun!\"",
}

type Process struct {
	PID           int
	CreatedAt     time.Time
	Cmd           string
	NotResponding bool
}

type State struct {
	rng             *rand.Rand
	processes       []Process
	score           int
	ramPercent      int
	gameOver        bool
	quitRequested   bool
	killAllRemain   int
	lastSpawn       time.Time
	lastBurst       time.Time
	spawnInterval   time.Duration
	lastMessage     string
	lastMessageType string
	startedAt       time.Time
	history         []string
	historyIndex    int

	input  textinput.Model
	width  int
	height int
}

func New() *State {
	seed := time.Now().UnixNano()
	in := textinput.New()
	in.Prompt = "$ "
	in.Focus()
	in.CharLimit = 64
	in.Width = 44

	return &State{
		rng:             rand.New(rand.NewSource(seed)),
		processes:       make([]Process, 0, 64),
		killAllRemain:   3,
		spawnInterval:   3 * time.Second,
		lastMessage:     "",
		lastMessageType: "info",
		startedAt:       time.Now(),
		history:         make([]string, 0, 64),
		historyIndex:    0,
		input:           in,
	}
}

func (s *State) Run() error {
	s.spawn(3)
	s.recalculateRAM()
	p := tea.NewProgram(s, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

type tickMsg time.Time

func tickCmd() tea.Cmd {
	return tea.Tick(180*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (s *State) Init() tea.Cmd {
	return tickCmd()
}

func (s *State) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.width = msg.Width
		s.height = msg.Height
		return s, nil

	case tickMsg:
		if !s.gameOver {
			s.tick(time.Time(msg))
			s.recalculateRAM()
			if s.ramPercent >= 100 {
				s.gameOver = true
				s.lastMessage = "GAME OVER"
				s.lastMessageType = "error"
			}
		}
		return s, tickCmd()

	case tea.KeyMsg:
		if msg.String() == "q" && s.gameOver {
			return s, tea.Quit
		}

		switch msg.String() {
		case "ctrl+c":
			return s, tea.Quit
		case "enter":
			line := strings.TrimSpace(s.input.Value())
			s.input.SetValue("")
			s.addHistory(line)
			s.handleInput(line)
			if s.quitRequested {
				return s, tea.Quit
			}
			s.recalculateRAM()
			if s.ramPercent >= 100 {
				s.gameOver = true
				s.lastMessage = "RAM reached 100%. Press q to quit."
				s.lastMessageType = "error"
			}
			return s, nil
		case "up":
			if !s.gameOver {
				s.historyPrev()
				return s, nil
			}
		case "down":
			if !s.gameOver {
				s.historyNext()
				return s, nil
			}
		}
	}

	if !s.gameOver {
		var cmd tea.Cmd
		s.input, cmd = s.input.Update(msg)
		return s, cmd
	}

	return s, nil
}

func (s *State) tick(now time.Time) {
	if s.lastSpawn.IsZero() {
		s.lastSpawn = now
		s.lastBurst = now
		return
	}

	elapsed := now.Sub(s.startedAt)
	difficulty := 1.0 + float64(elapsed/time.Second)/40.0
	if difficulty > 3.0 {
		difficulty = 3.0
	}

	if now.Sub(s.lastSpawn) >= s.spawnInterval {
		s.spawn(1)
		s.lastSpawn = now
	}

	if s.score >= 100 && now.Sub(s.lastBurst) >= 1*time.Second && s.rng.Float64() < 0.00666 {
		burst := 2 + s.rng.Intn(4)
		s.spawn(burst)
		s.lastMessage = fmt.Sprintf("burst detected: +%d processes", burst)
		s.lastMessageType = "warn"
	}
}

func (s *State) spawn(n int) {
	now := time.Now()
	for range n {
		p := Process{
			PID:           s.randomPID(),
			CreatedAt:     now,
			Cmd:           cmdNames[s.rng.Intn(len(cmdNames))],
			NotResponding: s.rng.Float64() < 0.10,
		}
		s.processes = append(s.processes, p)
	}
}

func (s *State) randomPID() int {
	minPID, maxPID := s.currentPIDRange()

	for range 64 {
		pid := minPID + s.rng.Intn(maxPID-minPID+1)
		if !s.pidExists(pid) {
			return pid
		}
	}

	for pid := minPID; pid <= maxPID; pid++ {
		if !s.pidExists(pid) {
			return pid
		}
	}

	const (
		globalMinPID = 10
		globalMaxPID = 999999
	)

	for pid := globalMinPID; pid <= globalMaxPID; pid++ {
		if !s.pidExists(pid) {
			return pid
		}
	}

	return globalMinPID
}

func (s *State) currentPIDRange() (minPID int, maxPID int) {
	score := s.score
	if score < 100 {
		return 100, 999
	} else if score < 300 {
		return 1000, 9999
	} else if score < 600 {
		return 10000, 99999
	} else {
		return 100000, 999999
	}
}

func (s *State) pidExists(pid int) bool {
	for _, p := range s.processes {
		if p.PID == pid {
			return true
		}
	}
	return false
}

func (s *State) recalculateRAM() {
	count := len(s.processes)
	if count == 0 {
		s.ramPercent = 0
		return
	}

	nrBonus := 0
	for _, p := range s.processes {
		if p.NotResponding {
			nrBonus++
		}
	}

	ram := min(count*5+nrBonus*3, 100)
	s.ramPercent = ram
}

func (s *State) handleInput(raw string) {
	line := strings.TrimSpace(raw)
	if line == "" {
		s.lastMessageType = "warn"
		s.lastMessage = "empty input"
		return
	}

	parts := strings.Fields(line)
	if len(parts) == 0 {
		return
	}

	if parts[0] == "q" {
		s.quitRequested = true
		return
	}

	if parts[0] == "killall" {
		s.handleKillAll(parts)
		return
	}

	if s.gameOver {
		s.lastMessageType = "error"
		s.lastMessage = "game over. press q to quit"
		return
	}

	if parts[0] != "kill" {
		s.lastMessageType = "error"
		s.lastMessage = fmt.Sprintf("sh: command not found: %s", parts[0])
		return
	}

	pid, force, ok := parseKill(parts)
	if !ok {
		s.lastMessageType = "error"
		s.lastMessage = "usage: kill <pid> | kill -9 <pid> | kill <pid> -9 | kill -s SIGKILL <pid> | kill -SIGKILL <pid>"
		return
	}

	idx := s.findProcess(pid)
	if idx == -1 {
		s.lastMessageType = "error"
		s.lastMessage = fmt.Sprintf("kill: (%d) - No such process", pid)
		return
	}

	target := s.processes[idx]
	if target.NotResponding && !force {
		s.lastMessageType = "warn"
		s.lastMessage = fmt.Sprintf("kill: (%d) requires -9", pid)
		return
	}

	bonus := 10
	if target.NotResponding {
		bonus = 25
	} else if force {
		bonus = -5
	}

	s.processes = append(s.processes[:idx], s.processes[idx+1:]...)
	s.score += bonus
	s.lastMessageType = "ok"
	if bonus >= 0 {
		s.lastMessage = fmt.Sprintf("killed %d (+%d)", pid, bonus)
	} else {
		s.lastMessageType = "warn"
		s.lastMessage = fmt.Sprintf("killed %d (%d, unnecessary `-9` option)", pid, bonus)
	}
}

func (s *State) handleKillAll(parts []string) {
	if len(parts) > 1 {
		s.lastMessageType = "error"
		s.lastMessage = "usage: killall"
		return
	}

	if s.killAllRemain <= 0 {
		s.lastMessageType = "error"
		s.lastMessage = "killall: no remaining uses"
		return
	}

	if len(s.processes) == 0 {
		s.lastMessageType = "warn"
		s.lastMessage = "killall: no processes"
		return
	}

	force := true
	kept := make([]Process, 0, len(s.processes))
	killed := 0
	gained := 0

	for _, p := range s.processes {
		if p.NotResponding && !force {
			kept = append(kept, p)
			continue
		}
		killed++
		if p.NotResponding {
			gained += 25
		} else {
			gained += 10
		}
	}

	s.processes = kept
	s.score += gained
	s.killAllRemain--
	s.lastMessageType = "ok"
	s.lastMessage = fmt.Sprintf("killall removed %d processes (+%d) [remaining: %d]", killed, gained, s.killAllRemain)
}

func (s *State) addHistory(line string) {
	if line == "" {
		s.historyIndex = len(s.history)
		return
	}

	if len(s.history) == 0 || s.history[len(s.history)-1] != line {
		s.history = append(s.history, line)
	}
	s.historyIndex = len(s.history)
}

func (s *State) historyPrev() {
	if len(s.history) == 0 {
		return
	}
	if s.historyIndex > 0 {
		s.historyIndex--
	}
	s.input.SetValue(s.history[s.historyIndex])
	s.input.CursorEnd()
}

func (s *State) historyNext() {
	if len(s.history) == 0 {
		return
	}
	if s.historyIndex < len(s.history)-1 {
		s.historyIndex++
		s.input.SetValue(s.history[s.historyIndex])
		s.input.CursorEnd()
		return
	}
	s.historyIndex = len(s.history)
	s.input.SetValue("")
}

func parseKill(parts []string) (pid int, force bool, ok bool) {
	if len(parts) < 2 {
		return 0, false, false
	}

	pidSet := false
	args := parts[1:]

	for i := 0; i < len(args); i++ {
		arg := args[i]

		switch {
		case arg == "-s":
			if i+1 >= len(args) {
				return 0, false, false
			}
			next := args[i+1]
			if !isKillSignal(next) {
				return 0, false, false
			}
			force = true
			i++

		case strings.HasPrefix(arg, "-s") && len(arg) > 2:
			sig := strings.TrimPrefix(arg, "-s")
			if !isKillSignal(sig) {
				return 0, false, false
			}
			force = true

		case strings.HasPrefix(arg, "-"):
			sig := strings.TrimPrefix(arg, "-")
			if !isKillSignal(sig) {
				return 0, false, false
			}
			force = true

		default:
			if pidSet {
				return 0, false, false
			}

			parsedPID, err := strconv.Atoi(arg)
			if err != nil {
				return 0, false, false
			}

			pid = parsedPID
			pidSet = true
		}
	}

	if !pidSet {
		return 0, false, false
	}

	return pid, force, true
}

func isKillSignal(sig string) bool {
	normalized := strings.ToUpper(strings.TrimSpace(sig))
	switch normalized {
	case "9", "KILL", "SIGKILL":
		return true
	default:
		return false
	}
}

func (s *State) findProcess(pid int) int {
	for i, p := range s.processes {
		if p.PID == pid {
			return i
		}
	}
	return -1
}

func (s *State) View() string {
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("45")).Render("kill -9")
	score := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("51")).Render(fmt.Sprintf("SCORE: %d", s.score))

	bar, barStyle := renderRAMBar(s.ramPercent)
	ramLine := fmt.Sprintf("RAM: %s %d%%", barStyle.Render(bar), s.ramPercent)

	header := lipgloss.NewStyle().Foreground(lipgloss.Color("33")).Bold(true).Render(fmt.Sprintf("%-8s %-6s %s", "PID", "TIME", "CMD"))

	clone := make([]Process, len(s.processes))
	copy(clone, s.processes)
	sort.Slice(clone, func(i, j int) bool {
		return clone[i].CreatedAt.Before(clone[j].CreatedAt)
	})

	var b strings.Builder
	b.WriteString(title)
	b.WriteString("\n")
	b.WriteString(score)
	b.WriteString("\n")
	b.WriteString(ramLine)
	b.WriteString("\n\n")
	b.WriteString(header)
	b.WriteString("\n")

	if len(clone) == 0 {
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Render("(no processes)"))
		b.WriteString("\n")
	} else {
		for _, p := range clone {
			line := fmt.Sprintf("%-8d %-6s %s", p.PID, formatAge(time.Since(p.CreatedAt)), p.Cmd)
			if p.NotResponding {
				line += " [Not Responding]"
				line = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render(line)
			}
			b.WriteString(line)
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	if s.lastMessage != "" {
		msgStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
		switch s.lastMessageType {
		case "ok":
			msgStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
		case "warn":
			msgStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
		case "error":
			msgStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
		}
		b.WriteString(msgStyle.Render(s.lastMessage))
		b.WriteString("\n")
	}

	if s.gameOver {
		b.WriteString("q: quit")
		return b.String()
	}

	b.WriteString(s.input.View())
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(fmt.Sprintf("killall: %d | q: quit", s.killAllRemain)))

	return b.String()
}

func renderRAMBar(percent int) (bar string, style lipgloss.Style) {
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}

	width := 20
	filled := max(min(percent*width/100, width), 0)

	bar = strings.Repeat("█", filled) + strings.Repeat("░", width-filled)

	switch {
	case percent >= 75:
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	case percent >= 45:
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
	default:
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	}

	return bar, style
}
