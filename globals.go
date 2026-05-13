package main

import (
	"log"
	"sync"
)

var (
	logger = log.Default()

	finishedList   = map[string]struct{}{}
	finishedListMu sync.Mutex

	errorTextList = []string{
		"用户权限校验失败，请联系咨询",
		"获取权限出错",
		"鉴权凭证已过期，请重新登陆",
		"必须带有身份标识token才能请求数据",
		"用户token已过期，请重新登录",
		"登录无效，请重新登录",
		"内部错误",
		"token已过期, 请重新登录",
		"未带权限标识，没法获取数据权限",
		"未能识别权限标识",
	}
	messageNotBindedAccount            = "你尚未绑定账号！"
	messageBFZUseFailedUse             = "* 错误: 百分智绑定不支持此功能。"
	messageHFSUseQTExamID              = "您当前绑定的账号为【好分数】。若需要查询【七天网络】平台的成绩，请先绑定【七天网络】账号。"
	messageQTPasswordChanged           = "您当前绑定的【七天网络】账号修改过密码，请重新进行绑定。"
	chatSender                         = NewQQChatSender(newQQBotAPI(appConfig.AppID, appConfig.ClientSecret))
	dataStore                DataStore = MySQLStore{}
)
