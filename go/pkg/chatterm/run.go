package chatterm

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/andriyg76/simple-chat/go/pkg/chatclient"
	"github.com/charmbracelet/glamour"
	"github.com/chzyer/readline"
)

type instance struct {
	BaseURL    string    `json:"baseUrl"`
	Token      string    `json:"token"`
	ExpiresAt  time.Time `json:"expiresAt"`
	LastUsedAt time.Time `json:"lastUsedAt,omitempty"`
}

type fileConfig struct {
	DefaultProfile string              `json:"defaultProfile"`
	Profiles       map[string]instance `json:"profiles"`
}

type conversation struct {
	Code          string    `json:"code"`
	Title         string    `json:"title"`
	LastMessageAt time.Time `json:"lastMessageAt"`
	ReviewSummary string    `json:"reviewSummary"`
}

type openRoom struct {
	code  string
	title string
}

// Main runs the terminal client with os.Args and DefaultOptions.
func Main() error {
	return Run(os.Args[1:], DefaultOptions())
}

// Run parses flags and starts an interactive session. Embedders pass custom Options (AppName, IO).
func Run(args []string, opt Options) error {
	opt.applyDefaults()
	fs := flag.NewFlagSet(opt.AppName, flag.ContinueOnError)
	profile := fs.String("profile", "", "named server profile")
	server := fs.String("server", "", "server URL for enrollment")
	enroll := fs.String("enroll", "", "one-time enrollment code")
	plain := fs.Bool("plain", opt.PlainChat, "use sync plain send (/messages/plain) instead of async")
	if err := fs.Parse(args); err != nil {
		return err
	}
	opt.PlainChat = *plain

	cfg, path, err := loadConfig(opt.AppName)
	if err != nil {
		return err
	}
	name := *profile
	if name == "" {
		name, err = selectProfile(cfg, opt.Stdin, opt.Stdout)
		if err != nil {
			return err
		}
	}
	if name == "" {
		name = "default"
	}
	current := cfg.Profiles[name]
	if *server != "" || *enroll != "" {
		if *server == "" || *enroll == "" {
			return errors.New("--server and --enroll are required together")
		}
		if err := validateServer(*server); err != nil {
			return err
		}
		current, err = exchange(opt.HTTP, *server, *enroll)
		if err != nil {
			return err
		}
		cfg.Profiles[name] = current
		cfg.DefaultProfile = name
		if err := saveConfig(path, cfg); err != nil {
			return err
		}
	}
	if current.Token == "" {
		return fmt.Errorf("no credentials: run %s --profile NAME --server URL --enroll CODE", opt.AppName)
	}
	current.LastUsedAt = time.Now()
	cfg.Profiles[name] = current
	cfg.DefaultProfile = name
	if err := saveConfig(path, cfg); err != nil {
		return err
	}
	return runSession(opt, name, current)
}

func selectProfile(cfg fileConfig, input io.Reader, output io.Writer) (string, error) {
	if len(cfg.Profiles) == 0 {
		return cfg.DefaultProfile, nil
	}
	if len(cfg.Profiles) == 1 {
		for name := range cfg.Profiles {
			return name, nil
		}
	}
	names := make([]string, 0, len(cfg.Profiles))
	for name := range cfg.Profiles {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		a, b := cfg.Profiles[names[i]].LastUsedAt, cfg.Profiles[names[j]].LastUsedAt
		if a.Equal(b) {
			return names[i] < names[j]
		}
		return a.After(b)
	})
	fmt.Fprintln(output, "Profiles:")
	for i, name := range names {
		last := "never"
		if value := cfg.Profiles[name].LastUsedAt; !value.IsZero() {
			last = value.Format("2006-01-02 15:04")
		}
		fmt.Fprintf(output, "  %d. %s  %s  last used: %s\n", i+1, name, cfg.Profiles[name].BaseURL, last)
	}
	fmt.Fprint(output, "Select profile number: ")
	line, err := bufio.NewReader(input).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	var selected int
	if _, err := fmt.Sscan(strings.TrimSpace(line), &selected); err != nil || selected < 1 || selected > len(names) {
		return "", errors.New("invalid profile selection")
	}
	return names[selected-1], nil
}

func runSession(opt Options, profile string, inst instance) error {
	items, err := conversations(opt.HTTP, inst)
	if err != nil {
		return err
	}
	reader := bufio.NewReader(opt.Stdin)
	if len(items) > 0 {
		fmt.Fprintln(opt.Stdout, "Conversations:")
		for i, c := range items {
			fmt.Fprintf(opt.Stdout, "  %d. %s  %s\n", i+1, c.Title, c.Code)
		}
	}
	fmt.Fprint(opt.Stdout, "Open numbers (comma-separated, e.g. 1,2) or Enter for new room: ")
	line, _ := reader.ReadString('\n')
	indices := parseConversationSelection(strings.TrimSpace(line), len(items))

	client := chatclient.Client{BaseURL: inst.BaseURL, Token: inst.Token, HTTP: opt.HTTP}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	open := make([]openRoom, 0, len(indices))
	titleByCode := map[string]string{}
	if len(indices) == 0 {
		sess, err := client.Create(context.Background())
		if err != nil {
			return err
		}
		open = append(open, openRoom{code: sess.Code, title: sess.Title})
		titleByCode[sess.Code] = sess.Title
		fmt.Fprintln(opt.Stdout, "Started a new conversation.")
	} else {
		for _, i := range indices {
			c := items[i-1]
			open = append(open, openRoom{code: c.Code, title: c.Title})
			titleByCode[c.Code] = c.Title
		}
	}

	activeCode := open[0].code
	watched := make(map[string]bool)
	var subsMu sync.Mutex
	var outputMu sync.Mutex
	progressByRoom := map[string]*progressDisplay{}
	inboxStarted := false

	historyFile, err := prepareHistoryFile(opt.AppName, profile)
	if err != nil {
		return err
	}
	lineEditor, err := readline.NewEx(&readline.Config{
		Prompt:                 "> ",
		HistoryFile:            historyFile,
		HistoryLimit:           1_000,
		DisableAutoSaveHistory: false,
	})
	if err != nil {
		return fmt.Errorf("readline: %w", err)
	}
	defer lineEditor.Close()
	renderer, err := glamour.NewTermRenderer(glamour.WithAutoStyle())
	if err != nil {
		return fmt.Errorf("create Markdown renderer: %w", err)
	}

	startInbox := func() {
		subsMu.Lock()
		if inboxStarted {
			subsMu.Unlock()
			return
		}
		inboxStarted = true
		subsMu.Unlock()
		inboxCursor, err := client.InboxCursor(context.Background())
		if err != nil {
			inboxCursor = 0
		}
		go func() {
			err := client.SubscribeInbox(ctx, inboxCursor, func(event chatclient.InboxEvent) error {
				code := event.SessionID
				subsMu.Lock()
				if !watched[code] {
					subsMu.Unlock()
					return nil
				}
				subsMu.Unlock()
				outputMu.Lock()
				defer outputMu.Unlock()
				progress := progressByRoom[code]
				if progress == nil {
					return nil
				}
				label := titleByCode[code]
				if label == "" {
					label = code
				}
				switch event.Kind {
				case "progress":
					progress.update(event.Text)
				case "message", "reply":
					if event.Sender == "client" {
						return nil
					}
					progress.clear()
					fmt.Fprintf(opt.Stdout, "[%s] %s", label, renderMarkdown(renderer, event.Text))
				case "error":
					progress.clear()
					fmt.Fprintf(opt.Stderr, "[%s] error: %s\n", label, event.Text)
				}
				return nil
			})
			if err != nil && ctx.Err() == nil {
				outputMu.Lock()
				fmt.Fprintf(opt.Stderr, "inbox delivery error: %v\n", err)
				outputMu.Unlock()
			}
		}()
	}
	startWatch := func(code string) {
		subsMu.Lock()
		watched[code] = true
		subsMu.Unlock()
		progressByRoom[code] = &progressDisplay{output: opt.Stdout}
		startInbox()
	}
	for _, room := range open {
		startWatch(room.code)
	}

	fmt.Fprintf(opt.Stdout, "Active room: %s. Commands: /title NAME, /use N, /watch N, /add ID, /remove ID, /members, /exit\n", titleByCode[activeCode])
	for {
		line, err := lineEditor.Readline()
		if errors.Is(err, readline.ErrInterrupt) {
			continue
		}
		if err != nil && !errors.Is(err, io.EOF) {
			return err
		}
		command := strings.TrimSpace(line)
		if command == "/exit" || command == "/logout" || (err != nil && line == "") {
			return nil
		}
		if command == "" {
			continue
		}
		if handled, err := handleSlashCommand(client, ctx, command, items, &open, &activeCode, titleByCode, startWatch); handled {
			if err != nil {
				fmt.Fprintln(opt.Stderr, "error:", err)
			}
			continue
		}
		outputMu.Lock()
		fmt.Fprint(opt.Stdout, "…\n")
		outputMu.Unlock()
		if opt.PlainChat {
			_, err = client.PostPlain(ctx, activeCode, command)
		} else {
			_, err = client.Post(ctx, activeCode, command)
		}
		if err != nil {
			fmt.Fprintln(opt.Stderr, "error:", err)
		}
	}
}

func parseConversationSelection(line string, count int) []int {
	if line == "" || count == 0 {
		return nil
	}
	var out []int
	for part := range strings.SplitSeq(line, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		var n int
		if _, err := fmt.Sscan(part, &n); err != nil || n < 1 || n > count {
			continue
		}
		out = append(out, n)
	}
	return out
}

func handleSlashCommand(
	client chatclient.Client,
	ctx context.Context,
	command string,
	items []conversation,
	open *[]openRoom,
	activeCode *string,
	titleByCode map[string]string,
	startWatch func(string),
) (bool, error) {
	if strings.HasPrefix(command, "/title ") {
		title := strings.TrimSpace(command[len("/title "):])
		sess, err := client.UpdateTitle(ctx, *activeCode, title)
		if err != nil {
			return true, err
		}
		titleByCode[*activeCode] = sess.Title
		return true, nil
	}
	if strings.HasPrefix(command, "/add ") {
		id := strings.TrimSpace(command[len("/add "):])
		if err := client.AddMember(ctx, *activeCode, id); err != nil {
			return true, err
		}
		return true, nil
	}
	if strings.HasPrefix(command, "/remove ") {
		id := strings.TrimSpace(command[len("/remove "):])
		if err := client.RemoveMember(ctx, *activeCode, id); err != nil {
			return true, err
		}
		return true, nil
	}
	if command == "/members" {
		members, err := client.ListMembers(ctx, *activeCode)
		if err != nil {
			return true, err
		}
		fmt.Println("members:", strings.Join(members, ", "))
		return true, nil
	}
	if strings.HasPrefix(command, "/use ") {
		arg := strings.TrimSpace(command[len("/use "):])
		if n, err := strconv.Atoi(arg); err == nil && n > 0 && n <= len(*open) {
			*activeCode = (*open)[n-1].code
			return true, nil
		}
		for _, room := range *open {
			if room.code == arg || strings.HasPrefix(room.code, arg) {
				*activeCode = room.code
				return true, nil
			}
		}
		return true, errors.New("unknown room; use /use N")
	}
	if strings.HasPrefix(command, "/watch ") {
		arg := strings.TrimSpace(command[len("/watch "):])
		var code string
		if n, err := strconv.Atoi(arg); err == nil && n > 0 && n <= len(items) {
			code = items[n-1].Code
			titleByCode[code] = items[n-1].Title
		} else {
			for _, c := range items {
				if c.Code == arg || strings.HasPrefix(c.Code, arg) {
					code = c.Code
					titleByCode[code] = c.Title
					break
				}
			}
		}
		if code == "" {
			return true, errors.New("unknown conversation")
		}
		startWatch(code)
		*open = append(*open, openRoom{code: code, title: titleByCode[code]})
		return true, nil
	}
	if command == "/help" {
		fmt.Println("Commands: /title NAME, /use N, /watch N, /add PROFILE, /remove PROFILE, /members, /exit")
		return true, nil
	}
	return false, nil
}

func renderMarkdown(renderer *glamour.TermRenderer, input string) string {
	rendered, err := renderer.Render(input)
	if err != nil {
		return input + "\n"
	}
	return rendered
}

func prepareHistoryFile(appName, profile string) (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	historyDir := filepath.Join(dir, appName, "history")
	if err := os.MkdirAll(historyDir, 0700); err != nil {
		return "", err
	}
	name := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.' {
			return r
		}
		return '_'
	}, profile)
	path := filepath.Join(historyDir, name+".history")
	f, err := os.OpenFile(path, os.O_CREATE, 0600)
	if err != nil {
		return "", err
	}
	return path, f.Close()
}

func exchange(httpClient *http.Client, server, code string) (instance, error) {
	token, duration, err := chatclient.Exchange(context.Background(), server, code, httpClient)
	if err != nil {
		return instance{}, err
	}
	return instance{BaseURL: strings.TrimRight(server, "/"), Token: token, ExpiresAt: time.Now().Add(duration)}, nil
}

func conversations(httpClient *http.Client, i instance) ([]conversation, error) {
	items, err := (chatclient.Client{BaseURL: i.BaseURL, Token: i.Token, HTTP: httpClient}).List(context.Background())
	result := make([]conversation, len(items))
	for n, item := range items {
		result[n] = conversation{Code: item.Code, Title: item.Title, LastMessageAt: item.LastMessageAt, ReviewSummary: item.ReviewSummary}
	}
	return result, err
}

func createConversation(httpClient *http.Client, i instance) (string, error) {
	out, err := (chatclient.Client{BaseURL: i.BaseURL, Token: i.Token, HTTP: httpClient}).Create(context.Background())
	return out.Code, err
}

type progressDisplay struct {
	output io.Writer
	lines  int
}

func (d *progressDisplay) update(text string) {
	d.clear()
	if text == "" {
		return
	}
	fmt.Fprint(d.output, text)
	d.lines = strings.Count(text, "\n") + 1
}

func (d *progressDisplay) clear() {
	if d.lines == 0 {
		return
	}
	fmt.Fprint(d.output, "\r\033[2K")
	for line := 1; line < d.lines; line++ {
		fmt.Fprint(d.output, "\033[1A\r\033[2K")
	}
	d.lines = 0
}

func loadConfig(appName string) (fileConfig, string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return fileConfig{}, "", err
	}
	path := filepath.Join(dir, appName, "chat.json")
	cfg := fileConfig{Profiles: map[string]instance{}}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, path, nil
	}
	if err != nil {
		return cfg, path, err
	}
	return cfg, path, json.Unmarshal(raw, &cfg)
}

func saveConfig(path string, cfg fileConfig) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0600)
}

func validateServer(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return fmt.Errorf("invalid server URL")
	}
	if u.Scheme == "https" {
		return nil
	}
	host := strings.Split(u.Host, ":")[0]
	if u.Scheme == "http" && (host == "localhost" || host == "127.0.0.1" || host == "::1") {
		return nil
	}
	return fmt.Errorf("remote server URL must use HTTPS")
}

// Exit prints a chat error and exits the process (for thin cmd/main wrappers).
func Exit(err error) {
	if err == nil {
		return
	}
	fmt.Fprintln(os.Stderr, "chat:", err)
	os.Exit(1)
}
