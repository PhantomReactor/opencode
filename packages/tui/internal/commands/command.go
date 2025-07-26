package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/google/uuid"
	"github.com/sst/opencode-sdk-go"
	"github.com/sst/opencode/internal/attachment"
)

type ExecuteCommandMsg Command
type ExecuteCommandsMsg []Command
type CommandExecutedMsg Command

type Keybinding struct {
	RequiresLeader bool
	Key            string
}

func (k Keybinding) Matches(msg tea.KeyPressMsg, leader bool) bool {
	key := k.Key
	key = strings.TrimSpace(key)
	return key == msg.String() && (k.RequiresLeader == leader)
}

type CommandName string
type Command struct {
	Name        CommandName
	Description string
	Keybindings []Keybinding
	Trigger     []string
	Prompt      string
	Variables   []string
	IsCustom    bool
}

func (c Command) Keys() []string {
	var keys []string
	for _, k := range c.Keybindings {
		keys = append(keys, k.Key)
	}
	return keys
}

func (c Command) HasTrigger() bool {
	return len(c.Trigger) > 0
}

func (c Command) PrimaryTrigger() string {
	if len(c.Trigger) > 0 {
		return c.Trigger[0]
	}
	return ""
}

func (c Command) MatchesTrigger(trigger string) bool {
	return slices.Contains(c.Trigger, trigger)
}

func (c Command) HasVariables() bool {
	return len(c.Variables) > 0
}

func (c Command) ReplaceVariables(args []string) string {
	if !c.IsCustom {
		return c.Prompt
	}

	result := c.Prompt
	for i, variable := range c.Variables {
		placeholder := "$" + variable
		replacement := ""
		if i < len(args) {
			replacement = args[i]
		}
		result = strings.ReplaceAll(result, placeholder, replacement)
	}
	return result
}

func (c Command) ProcessWithTerminalCommands(args []string, client *opencode.Client) (string, error) {
	if !c.IsCustom {
		return c.Prompt, nil
	}

	if !strings.Contains(c.Prompt, "!`") {
		return c.ReplaceVariables(args), nil
	}

	variables := make(map[string]string)
	for i, variable := range c.Variables {
		if i < len(args) {
			variables[variable] = args[i]
		} else {
			variables[variable] = ""
		}
	}

	ctx := context.Background()
	response, err := client.Config.ExecuteCommand(ctx, opencode.ConfigExecuteCommandParams{
		Prompt:    c.Prompt,
		Variables: variables,
	})
	if err != nil {
		return "", err
	}

	return response.ProcessedPrompt, nil
}

func ProcessAttachmentsInText(text string, cwd string) (string, []*attachment.Attachment) {
	var attachments []*attachment.Attachment
	var result strings.Builder

	i := 0
	for i < len(text) {
		if text[i] == '@' {
			start := i + 1
			end := start

			for end < len(text) && text[end] != ' ' && text[end] != '\t' && text[end] != '\n' && text[end] != '\r' {
				end++
			}

			if end > start {
				filePath := text[start:end]

				fullPath := filePath
				if !filepath.IsAbs(filePath) {
					fullPath = filepath.Join(cwd, filePath)
				}

				if _, err := os.Stat(fullPath); err == nil {
					displayText := "@" + filepath.Base(filePath)
					startIndex := result.Len()
					endIndex := startIndex + len(displayText)

					attachment := &attachment.Attachment{
						ID:         uuid.NewString(),
						Type:       "file",
						Display:    displayText,
						URL:        fmt.Sprintf("file://%s", fullPath),
						Filename:   filepath.Base(filePath),
						MediaType:  "text/plain",
						StartIndex: startIndex,
						EndIndex:   endIndex,
						Source: &attachment.FileSource{
							Path: filePath,
						},
					}
					attachments = append(attachments, attachment)

					result.WriteString(displayText)
					i = end
					continue
				}
			}
		}

		result.WriteByte(text[i])
		i++
	}

	return result.String(), attachments
}

type CommandRegistry map[CommandName]Command

func (r CommandRegistry) Sorted() []Command {
	var commands []Command
	for _, command := range r {
		commands = append(commands, command)
	}
	slices.SortFunc(commands, func(a, b Command) int {
		if a.IsCustom && !b.IsCustom {
			return 1
		}
		if !a.IsCustom && b.IsCustom {
			return -1
		}

		if a.IsCustom && b.IsCustom {
			return strings.Compare(string(a.Name), string(b.Name))
		}

		priorityOrder := map[CommandName]int{
			SessionNewCommand:   0,
			AppHelpCommand:      1,
			SessionShareCommand: 2,
			ModelListCommand:    3,
		}

		aPriority, aHasPriority := priorityOrder[a.Name]
		bPriority, bHasPriority := priorityOrder[b.Name]

		if aHasPriority && bHasPriority {
			return aPriority - bPriority
		}
		if aHasPriority {
			return -1
		}
		if bHasPriority {
			return 1
		}
		if a.Name == AppExitCommand {
			return 1
		}
		if b.Name == AppExitCommand {
			return -1
		}

		return strings.Compare(string(a.Name), string(b.Name))
	})
	return commands
}
func (r CommandRegistry) Matches(msg tea.KeyPressMsg, leader bool) []Command {
	var matched []Command
	for _, command := range r.Sorted() {
		if command.Matches(msg, leader) {
			matched = append(matched, command)
		}
	}
	return matched
}

const (
	AppHelpCommand              CommandName = "app_help"
	SwitchModeCommand           CommandName = "switch_mode"
	SwitchModeReverseCommand    CommandName = "switch_mode_reverse"
	EditorOpenCommand           CommandName = "editor_open"
	SessionNewCommand           CommandName = "session_new"
	SessionListCommand          CommandName = "session_list"
	SessionShareCommand         CommandName = "session_share"
	SessionUnshareCommand       CommandName = "session_unshare"
	SessionInterruptCommand     CommandName = "session_interrupt"
	SessionCompactCommand       CommandName = "session_compact"
	SessionExportCommand        CommandName = "session_export"
	ToolDetailsCommand          CommandName = "tool_details"
	ModelListCommand            CommandName = "model_list"
	ThemeListCommand            CommandName = "theme_list"
	FileListCommand             CommandName = "file_list"
	FileCloseCommand            CommandName = "file_close"
	FileSearchCommand           CommandName = "file_search"
	FileDiffToggleCommand       CommandName = "file_diff_toggle"
	ProjectInitCommand          CommandName = "project_init"
	InputClearCommand           CommandName = "input_clear"
	InputPasteCommand           CommandName = "input_paste"
	InputSubmitCommand          CommandName = "input_submit"
	InputNewlineCommand         CommandName = "input_newline"
	MessagesPageUpCommand       CommandName = "messages_page_up"
	MessagesPageDownCommand     CommandName = "messages_page_down"
	MessagesHalfPageUpCommand   CommandName = "messages_half_page_up"
	MessagesHalfPageDownCommand CommandName = "messages_half_page_down"
	MessagesPreviousCommand     CommandName = "messages_previous"
	MessagesNextCommand         CommandName = "messages_next"
	MessagesFirstCommand        CommandName = "messages_first"
	MessagesLastCommand         CommandName = "messages_last"
	MessagesLayoutToggleCommand CommandName = "messages_layout_toggle"
	MessagesCopyCommand         CommandName = "messages_copy"
	MessagesUndoCommand         CommandName = "messages_undo"
	MessagesRedoCommand         CommandName = "messages_redo"
	AppExitCommand              CommandName = "app_exit"
)

func (k Command) Matches(msg tea.KeyPressMsg, leader bool) bool {
	for _, binding := range k.Keybindings {
		if binding.Matches(msg, leader) {
			return true
		}
	}
	return false
}

func parseBindings(bindings ...string) []Keybinding {
	var parsedBindings []Keybinding
	for _, binding := range bindings {
		for p := range strings.SplitSeq(binding, ",") {
			requireLeader := strings.HasPrefix(p, "<leader>")
			keybinding := strings.ReplaceAll(p, "<leader>", "")
			keybinding = strings.TrimSpace(keybinding)
			parsedBindings = append(parsedBindings, Keybinding{
				RequiresLeader: requireLeader,
				Key:            keybinding,
			})
		}
	}
	return parsedBindings
}

func loadCustomCommands(client *opencode.Client, defaultCommands []Command) []Command {
	var customCommands []Command

	if client == nil {
		return customCommands
	}

	ctx := context.Background()
	commandsResponse, err := client.Config.Commands(ctx)
	if err != nil {
		return customCommands
	}

	for _, cmdResp := range commandsResponse {
		isDupliacateCommand := false
		for _, defaultCommand := range defaultCommands {
			if slices.Contains(defaultCommand.Trigger, cmdResp.Name) {
				isDupliacateCommand = true
				break
			}
		}
		if isDupliacateCommand {
			continue
		}
		customCommands = append(customCommands, Command{
			Name:        CommandName(cmdResp.Name),
			Description: cmdResp.Description,
			Trigger:     []string{cmdResp.Name},
			Prompt:      cmdResp.Prompt,
			Variables:   cmdResp.Variables,
			IsCustom:    true,
		})
	}

	return customCommands
}
func LoadFromConfig(config *opencode.Config, client *opencode.Client) CommandRegistry {
	defaults := []Command{
		{
			Name:        AppHelpCommand,
			Description: "show help",
			Keybindings: parseBindings("<leader>h"),
			Trigger:     []string{"help"},
		},
		{
			Name:        SwitchModeCommand,
			Description: "next mode",
			Keybindings: parseBindings("tab"),
		},
		{
			Name:        SwitchModeReverseCommand,
			Description: "previous mode",
			Keybindings: parseBindings("shift+tab"),
		},
		{
			Name:        EditorOpenCommand,
			Description: "open editor",
			Keybindings: parseBindings("<leader>e"),
			Trigger:     []string{"editor"},
		},
		{
			Name:        SessionExportCommand,
			Description: "export conversation",
			Keybindings: parseBindings("<leader>x"),
			Trigger:     []string{"export"},
		},
		{
			Name:        SessionNewCommand,
			Description: "new session",
			Keybindings: parseBindings("<leader>n"),
			Trigger:     []string{"new", "clear"},
		},
		{
			Name:        SessionListCommand,
			Description: "list sessions",
			Keybindings: parseBindings("<leader>l"),
			Trigger:     []string{"sessions", "resume", "continue"},
		},
		{
			Name:        SessionShareCommand,
			Description: "share session",
			Keybindings: parseBindings("<leader>s"),
			Trigger:     []string{"share"},
		},
		{
			Name:        SessionUnshareCommand,
			Description: "unshare session",
			Keybindings: parseBindings("<leader>u"),
			Trigger:     []string{"unshare"},
		},
		{
			Name:        SessionInterruptCommand,
			Description: "interrupt session",
			Keybindings: parseBindings("esc"),
		},
		{
			Name:        SessionCompactCommand,
			Description: "compact the session",
			Keybindings: parseBindings("<leader>c"),
			Trigger:     []string{"compact", "summarize"},
		},
		{
			Name:        ToolDetailsCommand,
			Description: "toggle tool details",
			Keybindings: parseBindings("<leader>d"),
			Trigger:     []string{"details"},
		},
		{
			Name:        ModelListCommand,
			Description: "list models",
			Keybindings: parseBindings("<leader>m"),
			Trigger:     []string{"models"},
		},
		{
			Name:        ThemeListCommand,
			Description: "list themes",
			Keybindings: parseBindings("<leader>t"),
			Trigger:     []string{"themes"},
		},
		// {
		// 	Name:        FileListCommand,
		// 	Description: "list files",
		// 	Keybindings: parseBindings("<leader>f"),
		// 	Trigger:     []string{"files"},
		// },
		{
			Name:        FileCloseCommand,
			Description: "close file",
			Keybindings: parseBindings("esc"),
		},
		{
			Name:        FileSearchCommand,
			Description: "search file",
			Keybindings: parseBindings("<leader>/"),
		},
		{
			Name:        FileDiffToggleCommand,
			Description: "split/unified diff",
			Keybindings: parseBindings("<leader>v"),
		},
		{
			Name:        ProjectInitCommand,
			Description: "create/update AGENTS.md",
			Keybindings: parseBindings("<leader>i"),
			Trigger:     []string{"init"},
		},
		{
			Name:        InputClearCommand,
			Description: "clear input",
			Keybindings: parseBindings("ctrl+c"),
		},
		{
			Name:        InputPasteCommand,
			Description: "paste content",
			Keybindings: parseBindings("ctrl+v", "super+v"),
		},
		{
			Name:        InputSubmitCommand,
			Description: "submit message",
			Keybindings: parseBindings("enter"),
		},
		{
			Name:        InputNewlineCommand,
			Description: "insert newline",
			Keybindings: parseBindings("shift+enter", "ctrl+j"),
		},
		{
			Name:        MessagesPageUpCommand,
			Description: "page up",
			Keybindings: parseBindings("pgup"),
		},
		{
			Name:        MessagesPageDownCommand,
			Description: "page down",
			Keybindings: parseBindings("pgdown"),
		},
		{
			Name:        MessagesHalfPageUpCommand,
			Description: "half page up",
			Keybindings: parseBindings("ctrl+alt+u"),
		},
		{
			Name:        MessagesHalfPageDownCommand,
			Description: "half page down",
			Keybindings: parseBindings("ctrl+alt+d"),
		},
		{
			Name:        MessagesPreviousCommand,
			Description: "previous message",
			Keybindings: parseBindings("ctrl+up"),
		},
		{
			Name:        MessagesNextCommand,
			Description: "next message",
			Keybindings: parseBindings("ctrl+down"),
		},
		{
			Name:        MessagesFirstCommand,
			Description: "first message",
			Keybindings: parseBindings("ctrl+g"),
		},
		{
			Name:        MessagesLastCommand,
			Description: "last message",
			Keybindings: parseBindings("ctrl+alt+g"),
		},
		{
			Name:        MessagesLayoutToggleCommand,
			Description: "toggle layout",
			Keybindings: parseBindings("<leader>p"),
		},
		{
			Name:        MessagesCopyCommand,
			Description: "copy message",
			Keybindings: parseBindings("<leader>y"),
		},
		{
			Name:        MessagesUndoCommand,
			Description: "undo last message",
			Keybindings: parseBindings("<leader>u"),
			Trigger:     []string{"undo"},
		},
		{
			Name:        MessagesRedoCommand,
			Description: "redo message",
			Keybindings: parseBindings("<leader>r"),
			Trigger:     []string{"redo"},
		},
		{
			Name:        AppExitCommand,
			Description: "exit the app",
			Keybindings: parseBindings("ctrl+c", "<leader>q"),
			Trigger:     []string{"exit", "quit", "q"},
		},
	}

	registry := make(CommandRegistry)
	keybinds := map[string]string{}
	marshalled, _ := json.Marshal(config.Keybinds)
	json.Unmarshal(marshalled, &keybinds)

	for _, command := range defaults {
		// Remove share/unshare commands if sharing is disabled
		if config.Share == opencode.ConfigShareDisabled &&
			(command.Name == SessionShareCommand || command.Name == SessionUnshareCommand) {
			continue
		}
		if keybind, ok := keybinds[string(command.Name)]; ok && keybind != "" {
			if keybind == "none" {
				continue
			}
			command.Keybindings = parseBindings(keybind)
		}
		registry[command.Name] = command
	}

	customCommands := loadCustomCommands(client, defaults)
	for _, command := range customCommands {
		if keybind, ok := keybinds[string(command.Name)]; ok && keybind != "" {
			if keybind == "none" {
				continue
			}
			command.Keybindings = parseBindings(keybind)
		}
		registry[command.Name] = command
	}
	return registry
}
