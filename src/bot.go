package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

func handleMention(api *slack.Client, mention *slackevents.AppMentionEvent) {
	threadTS := mention.ThreadTimeStamp
	if threadTS == "" {
		threadTS = mention.TimeStamp
	}

	_, _, err := api.PostMessage(
		mention.Channel,
		slack.MsgOptionText("go", false),
		slack.MsgOptionTS(threadTS),
	)
	if err != nil {
		log.Println("failed to reply to mention: ", err)
	}
}

func getReadme(name_full string) (err error, response string){
	name_parts := strings.Split(name_full, "/")
	if len(name_parts) < 2 {
		return errors.New("invalid arguments, see help for usage"), ""
	}
	name := name_parts[0]
	repo := name_parts[1]
	url := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/refs/heads/main/README.md", name, repo)
	log.Println("attempting to downlaod readme of ", name, "/", repo)
	log.Printf("trying %s", url)
	resp, err := http.Get(url)
	if err != nil {
		return err, ""
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		readmeBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return err, ""
		}
		readmeString := string(readmeBytes)
		log.Println("downloaded readme")
		return nil, readmeString
	} else if resp.StatusCode == http.StatusNotFound {
		log.Println("repository not found")
		return errors.New("repository not found"), ""
	} else {
		error_msg := fmt.Sprintf("unknown error, code: %d", resp.Request.Response.StatusCode)
		log.Println(error_msg)
		return errors.New(error_msg), ""
	}
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("no .env found, defaulting to env variables")
	}

	appToken := os.Getenv("SLACK_APP_TOKEN")
	botToken := os.Getenv("SLACK_BOT_TOKEN")
	if appToken == "" || botToken == "" {
		log.Fatal("set SLACK_APP_TOKEN and SLACK_BOT_TOKEN first")
	}

	api := slack.New(botToken, slack.OptionAppLevelToken(appToken))
	client := socketmode.New(api)

	go func ()  {
		for evt := range client.Events {
			switch evt.Type {
			case socketmode.EventTypeConnected:
				log.Println("connected to slack")
			case socketmode.EventTypeEventsAPI:
				if err := client.Ack(*evt.Request); err != nil {
					log.Println("failed to acknowledge event: ", err)
				}

				event, ok := evt.Data.(slackevents.EventsAPIEvent)
				if !ok {
					continue
				}

				mention, ok := event.InnerEvent.Data.(*slackevents.AppMentionEvent)
				if !ok || mention.BotID != "" || mention.User == "" {
					continue
				}
				log.Printf("Received event: %T\n", event.InnerEvent.Data)
				go handleMention(api, mention)
			case socketmode.EventTypeSlashCommand:
				cmd, ok := evt.Data.(slack.SlashCommand)
				if !ok {
					client.Ack(*evt.Request)
					continue
				}

				if cmd.Command != "/gobot" {
					client.Ack(*evt.Request)
					continue
				}
				log.Println("received command: ", cmd.Command)
				var response string
				var rtype string
				var rblocks []slack.Block
				command := strings.ToLower(strings.TrimSpace(cmd.Text))
				command_parts := strings.Fields(command)
				switch command_parts[0] {
				case "hello":
					response = "hello! see help for all commands"
					rtype = "ephemeral"
				case "help":
					response = "gobot help\ngobot is a bot made for the go-ship ysws\nit's main purpose is to download and display readme's of github projects\navailable commands:\n - hello - displays a test message\n - help - displays this message\n - get [user/repo_name] - gets the readme for the repository\n - leaderboard - shows the top 10 users of gobot"
					rtype = "ephemeral"
				case "get":
					if len(command_parts) < 2 {
						response = "not enough arguments, usage:\nget [username/repo]"
						rtype = "ephemeral"
					}
					err, readme := getReadme(command_parts[1])
					if err != nil {
						response = fmt.Sprintf("failed to get readme: %s, contact @brny if this is not user error", err)
						rtype = "ephemeral"
					} else {
						footer := fmt.Sprintf("_Gobot - called by <@%s>_", cmd.UserID)
						blocks := []slack.Block{
							slack.NewHeaderBlock(
								slack.NewTextBlockObject(slack.PlainTextType, ":white_check_mark: Readme found", true, false),
							),
							slack.NewDividerBlock(),
							slack.NewSectionBlock(
								slack.NewTextBlockObject(
									slack.MarkdownType,
									"```" + readme + "```",
									false, false,
								),
								nil, nil,
							),
							slack.NewDividerBlock(),
							slack.NewSectionBlock(
								slack.NewTextBlockObject(
									slack.MarkdownType,
									footer,
									false, false,
								),
								nil, nil,
							),

						}
						response = readme
						rtype = "in_channel"
						rblocks = blocks
					}
					
				}
				payload := map[string]any{
					"response_type": rtype,
					"text": response,
				}
				if len(rblocks) > 0 {
					payload["blocks"] = rblocks
				}
				if err := client.Ack(*evt.Request, payload); err != nil {
					log.Println("failed to respond to command: ", err)
				}
			}
		}
	}()

	log.Println("connecting to slack...")
	if err := client.Run(); err != nil {
		log.Fatal(err)
	}
}