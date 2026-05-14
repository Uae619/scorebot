package main

import (
	"fmt"
	"regexp"
	"strings"
)

type bindArgs struct {
	accountType string
	username    string
	password    string
}

func parseBindArgs(content string) (bindArgs, string) {
	divMessage := strings.Fields(content)
	if len(divMessage) != 4 {
		return bindArgs{}, "* 错误: 格式不正确！\n格式：/绑定账号 [版本] [账号] [密码]\n版本：学生版为1，家长版为2，百分智为3，七天网络为4\n邮箱账号示例：/绑定账号 1 45678@qq点com 123456\n手机账号示例：/绑定账号 1 16688855554 123456"
	}
	switch divMessage[1] {
	case "1", "2", "3", "4":
	default:
		return bindArgs{}, "* 错误: 指令格式不正确！\n格式：/绑定账号 [版本] [账号] [密码]\n版本：学生版为1，家长版为2，百分智平台为3，七天网络为4\n邮箱账号示例：/绑定账号 1 45678@qq点com 123456\n手机账号示例：/绑定账号 1 16688886666 123456"
	}
	return bindArgs{
		accountType: divMessage[1],
		username:    normalizeOptional(divMessage[2]),
		password:    normalizeOptional(divMessage[3]),
	}, ""
}

func (h *CommandHandler) bindAccount(ctx *MessageContext) string {
	args, errMsg := parseBindArgs(ctx.Content)
	if errMsg != "" {
		return errMsg
	}

	switch args.accountType {
	case "1", "2":
		return h.bindHFSAccount(ctx, args)
	case "4":
		return h.bindQTAccount(ctx, args)
	default:
		return h.bindBFZAccount(ctx, args)
	}
}

func (h *CommandHandler) bindHFSAccount(ctx *MessageContext, args bindArgs) string {
	matched, _ := regexp.MatchString(`1[3-9]\d{9}`, args.username)
	if !matched && (!strings.Contains(args.username, "@") || !strings.Contains(args.username, ".")) {
		return "* 错误: 账号不正确，请确认你输入的账号为邮箱或手机号！\n邮箱账号示例：/绑定账号 1 45678@qq点com 123456\n手机账号示例：/绑定账号 1 16688886666 123456"
	}

	response := studentLoginWithContext(ctx, args.username, args.password, asInt(args.accountType))
	if len(response) == 1 {
		if strings.Contains(response[0], "简单") {
			return "* 错误: " + response[0]
		}
		return "* 错误: " + response[0]
	}

	response2 := studentSnapshotWithContext(ctx, response[1])
	if asString(response2["msg"]) != "信息获取成功" {
		return "* 错误: " + asString(response2["msg"])
	}
	hiddenConfig := studentGetHiddenConfigWithContext(ctx, response[1])
	if hiddenConfig["getSuccess"] != true {
		if isHFSRiskLocked(hiddenConfig) {
			return "* 错误: 账号被好分数风控机制命中，暂时无法查询。"
		}
		return "* 错误: " + defaultString(asString(hiddenConfig["msg"]), "未知错误")
	}
	if school := asString(response2["school"]); school == "wxyunxiaozb" || school == "" {
		return "* 错误: 账号尚未绑定学生，请在网页端/APP绑定学生后使用"
	}

	h.replaceExistingBinding(ctx)
	mode := "parent"
	if args.accountType == "1" {
		mode = "student"
	}
	h.persistBindingResult(ctx, map[string]any{
		"mode":   mode,
		"xuehao": response2["xuehao"],
		"zh":     args.username,
		"pw":     args.password,
		"id":     response2["studentid"],
		"school": response2["school"],
		"grade":  response2["grade"],
		"banji":  response2["class"],
		"name":   response2["name"],
		"token":  response[1],
	})
	return fmt.Sprintf(" * 绑定成功！\n用户基本信息：[%s]%s(%s)\n如需取消绑定，请回复[/取消绑定]，输入[/帮助]可阅读功能列表。\n若学生身份信息发生变更，请回复[/获取快照]以更新学生信息。\n注：出于功能实现的必要，机器人会在数据库中保存您的绑定信息（包括密码），故建议您使用最简单的密码，请知悉。相关信息会在您取消绑定时被清除。", asString(response2["school"]), asString(response2["name"]), asString(response2["xuehao"]))
}

func (h *CommandHandler) bindQTAccount(ctx *MessageContext, args bindArgs) string {
	response := qtStudentLoginWithContext(ctx, args.username, args.password)
	if response["isSuccess"] != true {
		return "* 错误: " + asString(response["msg"])
	}

	h.replaceExistingBinding(ctx)
	h.persistBindingResult(ctx, map[string]any{
		"mode":   "qt",
		"xuehao": nil,
		"zh":     args.username,
		"pw":     args.password,
		"name":   response["name"],
		"school": response["school"],
		"grade":  response["grade"],
		"banji":  nil,
		"token":  response["token"],
		"id":     nil,
	})
	return fmt.Sprintf(" * 绑定成功！（七天）\n用户基本信息：%s\n注：该绑定类型与APP登录状态会有所冲突。如需取消绑定，请回复[/取消绑定]，输入[/帮助]可阅读功能列表。\n注2：出于功能实现的必要，机器人会在数据库中保存您的绑定信息（包括密码），故建议您使用最简单的密码，请知悉。相关信息会在您取消绑定时被清除。", asString(response["name"]))
}

func (h *CommandHandler) bindBFZAccount(ctx *MessageContext, args bindArgs) string {
	response := bfzStudentLoginWithContext(messageContext(ctx), args.username, args.password)
	if response["isSuccess"] != true {
		return "* 错误: 百分智平台登录失败，请检查账号和密码。"
	}

	h.replaceExistingBinding(ctx)
	h.persistBindingResult(ctx, map[string]any{
		"mode":   "bfz",
		"xuehao": args.username,
		"zh":     args.username,
		"pw":     args.password,
		"name":   response["name"],
		"school": nil,
		"grade":  nil,
		"banji":  nil,
		"token":  nil,
		"id":     nil,
	})
	return fmt.Sprintf(" * 绑定成功！\n用户姓名：%s\n绑定类型：百分智平台", asString(response["name"]))
}

func (h *CommandHandler) replaceExistingBinding(ctx *MessageContext) {
	userdata := opView(ctx.UserID)
	if ok, _ := userdata["Return"].(bool); ok {
		opDeleteQTStudentExamCache(ctx.UserID)
		opDelete(ctx.UserID)
	}
}

func (h *CommandHandler) persistBindingResult(ctx *MessageContext, wdata map[string]any) {
	opNew(ctx.UserID)
	opWrite(ctx.UserID, wdata)
}

func (h *CommandHandler) myInfo(ctx *MessageContext) string {
	if ok, msg := h.requireAuth(ctx); !ok {
		return msg
	}
	modeLabel := "七天网络"
	switch asString(h.userdata["mode"]) {
	case "student":
		modeLabel = "学生版"
	case "parent":
		modeLabel = "家长版"
	case "bfz":
		modeLabel = "百分智"
	}
	returnInfo := fmt.Sprintf(
		"* UID %s\n* 绑定类型 %s\n* 绑定账号 %s\n* 绑定密码 %s\n* 绑定用户UID %s\n* 绑定用户学校 %s\n* 绑定用户学号 %s\n* 绑定用户名 %s\n* 绑定用户年级 %s\n* 绑定用户班级 %s\n* 最近查询考试 %s\n* token %s\n仅供您个人查阅，您在解除账号绑定后以上数据将会被清除。",
		stringOrNA(h.userdata["qqid"]),
		modeLabel,
		stringOrNA(h.userdata["zh"]),
		stringOrNA(h.userdata["pw"]),
		defaultString(intString(h.userdata["id"]), "N/A"),
		stringOrNA(h.userdata["school"]),
		stringOrNA(h.userdata["xuehao"]),
		stringOrNA(h.userdata["name"]),
		stringOrNA(h.userdata["grade"]),
		stringOrNA(h.userdata["banji"]),
		stringOrNA(h.userdata["exam"]),
		stringOrNA(h.userdata["token"]),
	)
	moonSendMsgWithContext(messageContext(ctx), appConfig.MoonGroupID, "【机器人服务推送】\n用户触发信息命令\n"+returnInfo)
	return returnInfo
}

func (h *CommandHandler) unbindAccount(ctx *MessageContext) string {
	if ok, msg := h.requireAuth(ctx); !ok {
		return msg
	}
	opDeleteQTStudentExamCache(ctx.UserID)
	opDelete(ctx.UserID)
	return "* 取消绑定完毕。"
}

func (h *CommandHandler) teacherLogin(ctx *MessageContext, school, username, password string) map[string]any {
	result := map[string]any{"loginSuccess": false, "message": ""}
	loginData := teacherLoginFenxiWithContext(messageContext(ctx), username, password)
	if loginData["loginSuccess"] != true {
		moonSendMsgWithContext(messageContext(ctx), appConfig.MoonGroupID, fmt.Sprintf("【机器人服务推送】\n登录失败 【%s】\n提示信息：%s\n使用账号：%s\n使用密码：%s", school, asString(loginData["msg"]), username, password))
		if asString(loginData["msg"]) == "codeError:该用户不存在或密码错误" {
			result["message"] = fmt.Sprintf("登录报错(%s)\n错误信息已推送给管理员，请耐心等待处理。", school)
		} else {
			result["message"] = "本功能暂时无法使用，还请谅解~\n本条报错信息已发送给管理员。"
		}
		return result
	}
	unifySID := asString(loginData["unify_sid"])
	if unifySID == "" {
		moonSendMsgWithContext(messageContext(ctx), appConfig.MoonGroupID, fmt.Sprintf("【机器人服务推送】\n登录失败/unify_sid获取 【%s】\n提示信息：%s\n使用账号：%s\n使用密码：%s", school, asString(loginData["msg"]), username, password))
		result["message"] = fmt.Sprintf("登录报错(%s): get unify_sid error\n错误信息已推送给管理员，请耐心等待处理。", school)
		return result
	}
	opWriteTeacher(school, map[string]any{"cookie": unifySID})
	result["loginSuccess"] = true
	return result
}

func (h *CommandHandler) getSnapshot(ctx *MessageContext) string {
	if ok, msg := h.requireAuth(ctx); !ok {
		return msg
	}
	if asString(h.userdata["mode"]) == "bfz" {
		return messageBFZUseFailedUse
	}
	if asString(h.userdata["mode"]) == "qt" {
		return h.qtGetSnapshot(ctx)
	}
	response := studentLoginWithContext(ctx, asString(h.userdata["zh"]), asString(h.userdata["pw"]), hfsAccountType(asString(h.userdata["mode"])))
	if len(response) == 1 {
		return "* 错误: " + response[0]
	}
	response2 := studentSnapshotWithContext(ctx, response[1])
	if asString(response2["msg"]) != "信息获取成功" {
		return "* 错误: " + defaultString(asString(response2["msg"]), "未知错误")
	}
	hiddenConfig := studentGetHiddenConfigWithContext(ctx, response[1])
	if hiddenConfig["getSuccess"] != true {
		if isHFSRiskLocked(hiddenConfig) {
			return "* 错误: 账号被好分数风控机制命中，暂时无法查询。"
		}
		return "* 错误: " + defaultString(asString(hiddenConfig["msg"]), "未知错误")
	}
	if school := asString(response2["school"]); school == "" || school == "wxyunxiaozb" {
		return "* 错误: 账号尚未绑定学生，请在网页端/APP绑定学生后使用"
	}

	opWrite(ctx.UserID, map[string]any{
		"xuehao": response2["xuehao"],
		"id":     response2["studentid"],
		"school": response2["school"],
		"grade":  response2["grade"],
		"banji":  response2["class"],
		"name":   response2["name"],
		"token":  response[1],
	})
	return fmt.Sprintf("* 获取快照成功！\n[%s]%s(%s)", asString(response2["school"]), asString(response2["name"]), asString(response2["xuehao"]))
}
