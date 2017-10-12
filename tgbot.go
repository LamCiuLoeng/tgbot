package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"gopkg.in/telegram-bot-api.v4"
)

func toBuildJob(bot *tgbotapi.BotAPI, update tgbotapi.Update, j Jenkins) error {
	words := strings.Fields(update.Message.CommandArguments())
	var job, sha1 string
	if len(words) > 1 {
		job = words[0]
		sha1 = words[1]
	} else if len(words) == 1 {
		job = words[0]
	}

	msg := tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprintf("准备触发任务[%s]", job))
	msg.ReplyToMessageID = update.Message.MessageID
	bot.Send(msg)
	path, err := j.SubmitJob(job, sha1)
	if err != nil {
		msg = tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprintf("提交任务失败:%s", err))
		msg.ReplyToMessageID = update.Message.MessageID
		bot.Send(msg)
		return err
	}

	var lastBuildNumber string
	for {
		lastBuildNumber, err = j.GetBuildNumber(path)
		if err != nil {
			time.Sleep(5 * time.Second)
			log.Printf("重试去获取build number")
		} else {
			break
		}
	}

	log.Printf("准备查询任务[%s]的build[%s]是否完成\n", job, lastBuildNumber)
	var result, building bool
	for {
		result, building, err = j.CheckResult(job, lastBuildNumber)
		if building == true && err == nil {
			time.Sleep(5 * time.Second)
		} else {
			if err == nil {
				break
			}
			return err
		}
	}

	if result {
		msg = tgbotapi.NewMessage(update.Message.Chat.ID,
			fmt.Sprintf("😁👍 成功发布job[%s],build[%s]", job, lastBuildNumber))
		msg.ReplyToMessageID = update.Message.MessageID
		bot.Send(msg)
	} else {
		logText, err := j.GetLog(job, lastBuildNumber)
		if err != nil {
			msg = tgbotapi.NewMessage(update.Message.Chat.ID,
				fmt.Sprintf("检查任务[%s]的build[%s]输出结果失败:%s", job, lastBuildNumber, err))
			msg.ReplyToMessageID = update.Message.MessageID
			bot.Send(msg)
			return err
		}
		msg = tgbotapi.NewMessage(update.Message.Chat.ID,
			fmt.Sprintf("😱😱😱 发布job[%s]失败,日志最后内容如果下:\n%s", job, logText))
		msg.ReplyToMessageID = update.Message.MessageID
		bot.Send(msg)
	}
	return nil
}

func toListJob(bot *tgbotapi.BotAPI, update tgbotapi.Update, j Jenkins) error {
	jobNames, err := j.ListJobs()
	var content string
	if err != nil {
		content = fmt.Sprintf("😵 查询失败\n%s", err)
	} else {
		content = strings.Join(jobNames, "\n")
	}
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, content)
	bot.Send(msg)
	return nil
}

func toHelp(bot *tgbotapi.BotAPI, update tgbotapi.Update) {
	content := "我提供以下服务:\n/deploy {job} [sha1]  发布项目\n/jobs 显示所有项目\n/ping 回复pong，证明我活着"
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, content)
	bot.Send(msg)
}

func main() {
	botToken := os.Getenv("BOT_TOKEN")
	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Panic(err)
	}

	url := os.Getenv("JENKINS_URL")
	user := os.Getenv("JENKINS_USER")
	userToken := os.Getenv("JENKINS_USER_TOKEN")
	jobToken := os.Getenv("JENKINS_JOB_TOKEN")
	j := NewJenkins(url, user, userToken, jobToken)

	// bot.Debug = true
	log.Printf("Authorized on account %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}
		if update.Message.IsCommand() {
			switch update.Message.Command() {
			case "deploy":
				toBuildJob(bot, update, j)
			case "jobs":
				toListJob(bot, update, j)
			case "ping":
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "pong")
				bot.Send(msg)
			default:
				toHelp(bot, update)
			}
		} else {
			toHelp(bot, update)
		}
	}
}
