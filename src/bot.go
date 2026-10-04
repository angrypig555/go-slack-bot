package main

import (
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

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
				log.Printf("Received event: %T\n", event.InnerEvent.Data)

			case socketmode.EventTypeSlashCommand:
				cmd, ok := evt.Data.(slack.SlashCommand)
				if !ok {
					client.Ack(*evt.Request)
					continue
				}
				log.Println("received command: ", cmd.Command)
				if err := client.Ack(*evt.Request, map[string]any{
					"response_type": "ephemeral",
					"text": "hello from go",
				}); err != nil {
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