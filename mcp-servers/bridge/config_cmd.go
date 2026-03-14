package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

var validReasoningEfforts = map[string]bool{"low": true, "medium": true, "high": true}

// defaultCategories는 code-routing.md에 정의된 8개 고정 카테고리다.
var defaultCategories = []string{
	"visual-engineering",
	"ultrabrain",
	"deep",
	"artistry",
	"quick",
	"writing",
	"unspecified-high",
	"unspecified-low",
}

type cliStatusKind int

const (
	cliBuiltin   cliStatusKind = iota // route == "claude"
	cliAvailable                      // CLI 설치됨
	cliMissing                        // CLI 미설치
)

type cliStatusInfo struct {
	Kind    cliStatusKind
	Command string // cliAvailable/cliMissing 일 때 사용
}

// cliStatusFor는 모델 이름에 대한 CLI 상태를 반환한다.
func cliStatusFor(modelName string, models map[string]ModelDef, clis map[string]bool) cliStatusInfo {
	if modelName == "claude" {
		return cliStatusInfo{Kind: cliBuiltin}
	}
	def, ok := models[modelName]
	if !ok {
		return cliStatusInfo{Kind: cliMissing, Command: "?"}
	}
	if clis[def.Command] {
		return cliStatusInfo{Kind: cliAvailable, Command: def.Command}
	}
	return cliStatusInfo{Kind: cliMissing, Command: def.Command}
}

// cliStatusString은 list 출력용 plain-text 문자열을 반환한다.
func cliStatusString(s cliStatusInfo) string {
	switch s.Kind {
	case cliBuiltin:
		return "—"
	case cliAvailable:
		return s.Command + " ✔"
	default:
		return s.Command + " ✗"
	}
}

type validationError struct {
	Rule    string
	Message string
	Warn    bool // true = warning only, not a hard error
}

// validateConfigRules는 config의 유효성을 검사하고 에러 목록을 반환한다.
func validateConfigRules(c Config) []validationError {
	var errs []validationError

	if c.Routes == nil {
		errs = append(errs, validationError{Rule: "routes 섹션 존재", Message: "routes 섹션이 없습니다"})
	}
	if c.Models == nil {
		errs = append(errs, validationError{Rule: "models 섹션 존재", Message: "models 섹션이 없습니다"})
	}
	if c.Routes == nil || c.Models == nil {
		return errs
	}

	for cat, model := range c.Routes {
		if model == "claude" {
			continue
		}
		if _, ok := c.Models[model]; !ok {
			errs = append(errs, validationError{
				Rule:    "route → model 존재",
				Message: fmt.Sprintf("%s → %s (models에 없음)", cat, model),
			})
		}
	}

	if c.DefaultRoute != "" && c.DefaultRoute != "claude" {
		if _, ok := c.Models[c.DefaultRoute]; !ok {
			errs = append(errs, validationError{
				Rule:    "default_route → model 존재",
				Message: fmt.Sprintf("default_route=%q: models에 없고 'claude'도 아님", c.DefaultRoute),
			})
		}
	}

	for cat, co := range c.CategoryOverrides {
		if co.ReasoningEffort != "" && !validReasoningEfforts[co.ReasoningEffort] {
			errs = append(errs, validationError{
				Rule:    "category_overrides reasoning_effort 값",
				Message: fmt.Sprintf("category_overrides[%s].reasoning_effort=%q: low/medium/high 중 하나여야 합니다", cat, co.ReasoningEffort),
			})
		}
		if _, ok := c.Routes[cat]; !ok {
			errs = append(errs, validationError{
				Rule:    "category_overrides → route 존재",
				Message: fmt.Sprintf("category_overrides[%s]: routes에 없는 카테고리 (override가 무시됩니다)", cat),
				Warn:    true,
			})
		}
	}

	return errs
}

// runConfigCommand는 config 서브커맨드를 처리한다.
func runConfigCommand(args []string) {
	sub := ""
	if len(args) > 0 {
		sub = args[0]
	}
	switch sub {
	case "list":
		printConfigTable()
	case "validate":
		runValidate()
	default:
		runConfigTUI()
	}
}

func printConfigTable() {
	if cfg.Routes == nil || cfg.Models == nil {
		fmt.Fprintf(os.Stderr, "error: config is missing routes or models section\n")
		os.Exit(1)
	}

	ordered := orderedCategories(cfg.Routes)

	fmt.Printf("%-22s %-20s %-14s %s\n", "Category", "Model", "CLI", "Overrides")
	fmt.Printf("%-22s %-20s %-14s %s\n", "──────────────────────", "────────────────────", "──────────────", "─────────────────────")
	for _, cat := range ordered {
		model := cfg.Routes[cat]
		status := cliStatusFor(model, cfg.Models, availableCLIs)
		override := ""
		if co, ok := cfg.CategoryOverrides[cat]; ok {
			var parts []string
			if co.ReasoningEffort != "" {
				parts = append(parts, "effort="+co.ReasoningEffort)
			}
			if co.PromptAppend != "" {
				parts = append(parts, "prompt_append")
			}
			override = strings.Join(parts, " ")
		}
		fmt.Printf("%-22s %-20s %-14s %s\n", cat, model, cliStatusString(status), override)
	}
}

type validateModel struct {
	spinner  spinner.Model
	checks   []checkResult
	done     bool
	hasError bool
}

type checkResult struct {
	label string
	pass  bool // true=✔, false=✗
	warn  bool // true=⚠ (경고, 오류 아님)
	msg   string
}

type validateDoneMsg struct {
	checks   []checkResult
	hasError bool
}

func (m validateModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, runChecksCmd())
}

func runChecksCmd() tea.Cmd {
	return func() tea.Msg {
		var checks []checkResult
		hasError := false

		checks = append(checks,
			checkResult{label: "routes 섹션 존재", pass: cfg.Routes != nil, msg: "routes 섹션이 없습니다"},
			checkResult{label: "models 섹션 존재", pass: cfg.Models != nil, msg: "models 섹션이 없습니다"},
		)

		missingCats := 0
		for _, cat := range defaultCategories {
			if _, ok := cfg.Routes[cat]; !ok {
				missingCats++
			}
		}
		checks = append(checks, checkResult{
			label: "8개 카테고리 필수 (모두 존재해야 함)",
			pass:  missingCats == 0,
			msg:   fmt.Sprintf("%d개 필수 카테고리 누락 (8개 모두 required)", missingCats),
		})

		if cfg.Routes != nil && cfg.Models != nil {
			errs := validateConfigRules(cfg)
			for _, e := range errs {
				if e.Warn {
					checks = append(checks, checkResult{
						label: e.Rule,
						pass:  false,
						warn:  true,
						msg:   e.Message,
					})
				} else {
					checks = append(checks, checkResult{
						label: e.Rule,
						pass:  false,
						msg:   e.Message,
					})
					hasError = true
				}
			}
		}

		for _, c := range checks {
			if !c.pass && !c.warn {
				hasError = true
			}
		}

		return validateDoneMsg{checks: checks, hasError: hasError}
	}
}

func (m validateModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case validateDoneMsg:
		m.checks = msg.checks
		m.hasError = msg.hasError
		m.done = true
		return m, tea.Quit
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m validateModel) View() string {
	if !m.done {
		return fmt.Sprintf("  %s Validating config...\n", m.spinner.View())
	}
	var b strings.Builder
	b.WriteString("  Validating config...\n\n")
	for _, c := range m.checks {
		switch {
		case c.pass:
			b.WriteString(styleOK.Render("  ✔  "+c.label) + "\n")
		case c.warn:
			b.WriteString(styleWarn.Render("  ⚠  "+c.msg) + "\n")
		default:
			b.WriteString(styleWarn.Render("  ✗  "+c.msg) + "\n")
		}
	}
	b.WriteString("\n")
	if m.hasError {
		b.WriteString(styleWarn.Render("  오류가 있습니다.") + "\n")
	} else {
		b.WriteString(styleOK.Render("  All checks passed.") + "\n")
	}
	return b.String()
}

func runValidate() {
	s := spinner.New()
	s.Spinner = spinner.Dot
	m := validateModel{spinner: s}
	p := tea.NewProgram(m)
	result, err := p.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "validate error: %v\n", err)
		os.Exit(1)
	}
	if result.(validateModel).hasError {
		os.Exit(1)
	}
}

// orderedCategories는 고정 카테고리 순서를 유지하며 추가 카테고리를 뒤에 붙인다.
func orderedCategories(routes map[string]string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, cat := range defaultCategories {
		if _, ok := routes[cat]; ok {
			result = append(result, cat)
			seen[cat] = true
		}
	}
	var extra []string
	for cat := range routes {
		if !seen[cat] {
			extra = append(extra, cat)
		}
	}
	sort.Strings(extra)
	return append(result, extra...)
}

// saveConfig는 config를 atomic write로 저장한다.
func saveConfig(c Config) error {
	path, err := getConfigPath()
	if err != nil {
		return err
	}
	return writeAtomicJSON(path, c, 0600)
}
