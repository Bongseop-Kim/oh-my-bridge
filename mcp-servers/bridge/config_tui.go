package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// diffEntry는 변경된 카테고리 한 행을 나타낸다.
type diffEntry struct {
	Category string
	From     string
	To       string
}

// computeDiff는 original과 current를 비교해 변경된 항목만 반환한다.
func computeDiff(original, current map[string]string) []diffEntry {
	var result []diffEntry
	for cat, to := range current {
		from := original[cat]
		if from != to {
			result = append(result, diffEntry{Category: cat, From: from, To: to})
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Category < result[j].Category
	})
	return result
}

// buildDropdownOptions는 "claude"를 첫 번째로 고정하고 models 키를 정렬해 반환한다.
func buildDropdownOptions(models map[string]ModelDef) []string {
	opts := []string{"claude"}
	var modelNames []string
	for name := range models {
		modelNames = append(modelNames, name)
	}
	sort.Strings(modelNames)
	return append(opts, modelNames...)
}

// --- TUI State ---

type tuiScreen int

const (
	screenList        tuiScreen = iota // 메인 카테고리 목록
	screenDropdown                     // 모델 드롭다운
	screenDiff                         // diff 미리보기
	screenQuitConfirm                  // 미저장 종료 확인
)

type tuiModel struct {
	screen     tuiScreen
	categories []string          // 표시 순서 고정
	original   map[string]string // 로드 시점 원본
	current    map[string]string // 편집 중인 상태
	models     map[string]ModelDef
	clis       map[string]bool

	cursor     int      // screenList: 선택된 행, screenDropdown: 선택된 모델
	listCursor int      // 드롭다운 진입 전 카테고리 인덱스 보존
	dropdown   []string // buildDropdownOptions 결과

	errMsg string // 저장 실패 메시지 (비어있으면 정상)
}

func newTUIModel() tuiModel {
	cats := orderedCategories(cfg.Routes)
	current := make(map[string]string, len(cfg.Routes))
	for k, v := range cfg.Routes {
		current[k] = v
	}
	original := make(map[string]string, len(cfg.Routes))
	for k, v := range cfg.Routes {
		original[k] = v
	}
	return tuiModel{
		screen:     screenList,
		categories: cats,
		original:   original,
		current:    current,
		models:     cfg.Models,
		clis:       availableCLIs,
	}
}

func (m tuiModel) Init() tea.Cmd { return nil }

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	switch m.screen {
	case screenList:
		return m.updateList(key)
	case screenDropdown:
		return m.updateDropdown(key)
	case screenDiff:
		return m.updateDiff(key)
	case screenQuitConfirm:
		return m.updateQuitConfirm(key)
	}
	return m, nil
}

func (m tuiModel) updateList(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.categories)-1 {
			m.cursor++
		}
	case "enter":
		if len(m.categories) == 0 {
			return m, nil
		}
		cat := m.categories[m.cursor]
		m.listCursor = m.cursor
		m.dropdown = buildDropdownOptions(m.models)
		if len(m.dropdown) == 0 {
			return m, nil
		}
		m.cursor = 0
		for i, opt := range m.dropdown {
			if opt == m.current[cat] {
				m.cursor = i
				break
			}
		}
		m.screen = screenDropdown
	case "s":
		diff := computeDiff(m.original, m.current)
		if len(diff) == 0 {
			fmt.Println("변경 없음.")
			return m, tea.Quit
		}
		m.cursor = 0
		m.screen = screenDiff
	case "q", "ctrl+c":
		if len(computeDiff(m.original, m.current)) > 0 {
			m.screen = screenQuitConfirm
		} else {
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m tuiModel) updateDropdown(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.dropdown)-1 {
			m.cursor++
		}
	case "enter":
		if len(m.categories) == 0 || len(m.dropdown) == 0 {
			return m, nil
		}
		listCursor := m.listCursor
		if listCursor >= len(m.categories) {
			listCursor = len(m.categories) - 1
		}
		cursor := m.cursor
		if cursor >= len(m.dropdown) {
			cursor = len(m.dropdown) - 1
		}
		cat := m.categories[listCursor]
		m.current[cat] = m.dropdown[cursor]
		m.cursor = m.listCursor
		m.screen = screenList
	case "esc":
		m.cursor = m.listCursor
		m.screen = screenList
	}
	return m, nil
}

func (m tuiModel) updateDiff(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "enter":
		newCfg := cfg
		newCfg.Routes = m.current
		if err := saveConfig(newCfg); err != nil {
			m.errMsg = fmt.Sprintf("저장 실패: %v", err)
			return m, nil
		}
		return m, tea.Quit
	case "esc":
		m.cursor = 0
		m.screen = screenList
	}
	return m, nil
}

func (m tuiModel) updateQuitConfirm(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "s":
		diff := computeDiff(m.original, m.current)
		if len(diff) > 0 {
			m.screen = screenDiff
		} else {
			return m, tea.Quit
		}
	case "q":
		return m, tea.Quit
	case "esc":
		m.screen = screenList
	}
	return m, nil
}

// --- Styles ---

var (
	styleSelected = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	styleOK       = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	styleWarn     = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	styleDim      = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	styleHeader   = lipgloss.NewStyle().Bold(true).Underline(true)
	styleBorder   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
)

func (m tuiModel) View() string {
	switch m.screen {
	case screenList:
		return m.viewList()
	case screenDropdown:
		return m.viewDropdown()
	case screenDiff:
		return m.viewDiff()
	case screenQuitConfirm:
		return m.viewQuitConfirm()
	}
	return ""
}

func (m tuiModel) viewList() string {
	var b strings.Builder
	b.WriteString(styleHeader.Render("oh-my-bridge config") + "\n\n")
	fmt.Fprintf(&b, "  %-22s %-22s %s\n", "Category", "Model", "CLI")
	fmt.Fprintf(&b, "  %-22s %-22s %s\n", strings.Repeat("─", 22), strings.Repeat("─", 22), strings.Repeat("─", 12))

	for i, cat := range m.categories {
		model := m.current[cat]
		status := cliStatusFor(model, m.models, m.clis)
		cliStr := renderCLIStatus(status)

		var line string
		if i == m.cursor {
			line = styleSelected.Render(fmt.Sprintf("> %-22s %-22s %s", cat, model+" [▼]", cliStr))
		} else {
			line = fmt.Sprintf("  %-22s %-22s %s", cat, model+" [▼]", cliStr)
		}
		b.WriteString(line + "\n")
	}

	b.WriteString("\n  " + styleDim.Render("[↑↓] 이동  [Enter] 모델 변경  [s] 저장  [q] 종료"))
	return styleBorder.Render(b.String())
}

func (m tuiModel) viewDropdown() string {
	cat := m.categories[m.listCursor]
	var b strings.Builder
	fmt.Fprintf(&b, "  %s 모델 선택:\n", cat)
	b.WriteString("  " + strings.Repeat("─", 30) + "\n")
	for i, opt := range m.dropdown {
		if i == m.cursor {
			b.WriteString(styleSelected.Render("  ● "+opt) + "\n")
		} else {
			b.WriteString("    " + opt + "\n")
		}
	}
	b.WriteString("\n  " + styleDim.Render("[↑↓] 이동  [Enter] 선택  [Esc] 취소"))
	return styleBorder.Render(b.String())
}

func (m tuiModel) viewDiff() string {
	diff := computeDiff(m.original, m.current)
	var b strings.Builder
	b.WriteString("  변경사항 확인:\n")
	b.WriteString("  " + strings.Repeat("─", 50) + "\n")
	for _, d := range diff {
		status := cliStatusFor(d.To, m.models, m.clis)
		cliStr := renderCLIStatus(status)
		line := fmt.Sprintf("  %-22s %-18s → %-18s %s", d.Category, d.From, d.To, cliStr)
		if status.Kind == cliMissing {
			line = styleWarn.Render(line)
		}
		b.WriteString(line + "\n")
	}
	if m.errMsg != "" {
		b.WriteString("\n  " + styleWarn.Render(m.errMsg))
	}
	b.WriteString("\n  " + styleDim.Render("[Enter] 저장  [Esc] 취소"))
	return styleBorder.Render(b.String())
}

func (m tuiModel) viewQuitConfirm() string {
	var b strings.Builder
	b.WriteString("  저장하지 않은 변경사항이 있습니다.\n\n")
	b.WriteString("  " + styleDim.Render("[s] 저장 후 종료  [q] 버리고 종료  [Esc] 취소"))
	return styleBorder.Render(b.String())
}

func renderCLIStatus(s cliStatusInfo) string {
	switch s.Kind {
	case cliBuiltin:
		return styleDim.Render("─ built-in")
	case cliAvailable:
		return styleOK.Render("● " + s.Command + " ✔")
	default:
		return styleWarn.Render("✗ " + s.Command + " 없음")
	}
}

// runConfigTUI는 TUI를 기동한다.
func runConfigTUI() {
	m := newTUIModel()
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "TUI error: %v\n", err)
		os.Exit(1)
	}
}
