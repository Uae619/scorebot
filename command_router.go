package main

import (
	"sort"
	"strings"
)

type commandFunc func(*CommandHandler, *MessageContext) string

type CommandHandler struct {
	sender          ChatSender
	commands        map[string]commandFunc
	commandKeys     []string
	userdata        map[string]any
	qtProfileToken  string
	qtProfileData   map[string]any
	qtLastOperation string
}

var (
	commandRegistry = map[string]commandFunc{
		"/绑定账号": (*CommandHandler).bindAccount,
		"/绑定帐号": (*CommandHandler).bindAccount,
		"/取消绑定": (*CommandHandler).unbindAccount,
		"/解除绑定": (*CommandHandler).unbindAccount,
		"/获取快照": (*CommandHandler).getSnapshot,
		"/查询":   (*CommandHandler).searchExams,
		"/我的信息": (*CommandHandler).myInfo,
		"/考试详情": (*CommandHandler).examInfo,
		"/答题卡":  (*CommandHandler).getAnswerSheet,
		"/科目详情": (*CommandHandler).subjectInfo,
		"/错题详情": (*CommandHandler).subjectFalseQuestions,
		"/答题详情": (*CommandHandler).subjectAllQuestions,
		"/帮助":   (*CommandHandler).showHelp,
	}
	commandRegistryKeys = sortedCommandKeys(commandRegistry)
	optionalNormalizer  = strings.NewReplacer(
		"/", "",
		"|", "",
		"｜", "",
		"[", "",
		"]", "",
		"［", "",
		"］", "",
		"【", "",
		"】", "",
		"{", "",
		"}", "",
		"(", "",
		")", "",
		"（", "",
		"）", "",
	)
)

func sortedCommandKeys(commands map[string]commandFunc) []string {
	commandKeys := make([]string, 0, len(commands))
	for key := range commands {
		commandKeys = append(commandKeys, key)
	}
	sort.Slice(commandKeys, func(i, j int) bool { return len(commandKeys[i]) > len(commandKeys[j]) })
	return commandKeys
}

func newCommandHandler(sender ChatSender) *CommandHandler {
	return &CommandHandler{
		sender:      sender,
		commands:    commandRegistry,
		commandKeys: commandRegistryKeys,
		userdata:    map[string]any{},
	}
}

func (h *CommandHandler) requireAuth(ctx *MessageContext) (bool, string) {
	userdata := opView(ctx.UserID)
	if ok, _ := userdata["Return"].(bool); !ok {
		logCommandTrace(ctx, "auth.failed", map[string]any{
			"reason": "user_not_bound",
		})
		return false, messageNotBindedAccount
	}
	h.userdata = userdata
	return true, ""
}

func stringOrNA(v any) string {
	if s := asString(v); s != "" {
		return s
	}
	return "N/A"
}

func intString(v any) string {
	s := asString(v)
	return strings.TrimLeft(s, "0")
}

func normalizeOptional(value string) string {
	return optionalNormalizer.Replace(value)
}

func (h *CommandHandler) handle(ctx *MessageContext) {
	commandKey := ""
	for _, key := range h.commandKeys {
		if strings.HasPrefix(ctx.Content, key) {
			commandKey = key
			break
		}
	}

	command := (*CommandHandler).messageNone
	if commandKey != "" {
		args := strings.TrimSpace(ctx.Content[len(commandKey):])
		ctx.Content = strings.TrimSpace(commandKey + " " + args)
		command = h.commands[commandKey]
	}

	responseText := command(h, ctx)
	if strings.TrimSpace(responseText) != "" {
		h.sender.SendText(messageContext(ctx), ctx, responseText)
	}
}

func (h *CommandHandler) messageNone(ctx *MessageContext) string {
	// userdata := opView(ctx.UserID)
	// if ok, _ := userdata["Return"].(bool); ok {
	// 	return "* 本条消息未触发任何指令。"
	// }
	return "* 本条消息未触发任何指令。"
}

func (h *CommandHandler) showHelp(ctx *MessageContext) string {
	return "* 本机器人使用方法请参考README。"
}
