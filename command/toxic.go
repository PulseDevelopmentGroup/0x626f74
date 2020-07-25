package command

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"

	"github.com/PulseDevelopmentGroup/0x626f74/log"
	"github.com/PulseDevelopmentGroup/0x626f74/multiplexer"
	"github.com/PulseDevelopmentGroup/0x626f74/util"
	"github.com/bwmarrin/discordgo"
)

// Toxic is a bot command
type (
	Toxic struct {
		Command  string
		HelpText string

		Key string

		Logger *log.Logs
	}

	response struct {
		AttributeScores map[string]*attribute `json:"attributeScores"`
	}

	attribute struct {
		SummaryScore struct {
			Value float32 `json:"value"`
		} `json:"summaryScore"`
	}
)

const (
	fmtRequest = `
	{
		"comment": {"text": "%s"}, 
		"languages": ["en"], 
		"requestedAttributes": {
			"TOXICITY":{}, 
			"FLIRTATION": {}, 
			"IDENTITY_ATTACK":{}, 
			"THREAT":{}
		}
	}`

	fmtURL = "https://commentanalyzer.googleapis.com/v1alpha1/comments:analyze?key=%s"
)

// Init is called by the multiplexer before the bot starts to initialize any
// variables the command needs.
func (c Toxic) Init(m *multiplexer.Mux) {
	// Nothing to init
}

// Handle is called by the multiplexer whenever a user triggers the command.
func (c Toxic) Handle(ctx *multiplexer.Context) {
	/* Get messages from the arguments */
	messages, err := c.getMessages(ctx)
	if err != nil {
		c.Logger.CmdErr(ctx, err, "Unable to get messages")
	}

	/* If no messages were found, let the user know and exit */
	if len(messages) == 0 {
		ctx.ChannelSend("No messages to check against")
		return
	}

	/* Built out arrays of messages and their attributes */
	var (
		ratings []map[string]*attribute
		content string // Temporary placeholder for single messages
	)

	for _, msg := range messages {
		/* Convert mentions to text */
		var err error
		content, err = msg.ContentWithMoreMentionsReplaced(ctx.Session)
		if err != nil {
			content = msg.Content
		}

		/* Get ratings */
		rating, err := c.getRatings(content, ctx)
		if err != nil {
			c.Logger.CmdErr(
				ctx, err, "Unable to get ratings for the supplied message",
			)
		}
		ratings = append(ratings, rating)
	}

	embed := &discordgo.MessageEmbed{
		Color: 0x13ff03,
		Author: &discordgo.MessageEmbedAuthor{
			IconURL: messages[0].Author.AvatarURL(""),
			Name:    messages[0].Author.Username,
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: "Chat analysis provided by Google's Perspective API",
		},
	}

	/* If there's just a single rating (only checking one message) */
	var fields []*discordgo.MessageEmbedField
	if len(ratings) == 1 {
		for k, v := range ratings[0] {
			fields = append(fields, &discordgo.MessageEmbedField{
				Name:  c.fixKey(k),
				Value: fmt.Sprintf("%.0f%%", v.SummaryScore.Value*100),
			})
		}

		embed.Title = "Message Toxicity Report"
		embed.Description = "\"" + content + "\""
		embed.Fields = fields
		/* If there's more than one rating (checking multiple messages) */
	} else {
		embed.Title = "User Toxicity Report"
		embed.Description = "Report based on the recent messages sent by the user"

		totals := make(map[string]float32)

		for _, rating := range ratings {
			for k, v := range rating {
				totals[k] += v.SummaryScore.Value
			}
		}

		for k, v := range totals {
			avg := v / float32(len(ratings)) * 100

			fields = append(fields, &discordgo.MessageEmbedField{
				Name:  c.fixKey(k),
				Value: fmt.Sprintf("%.0f%%", avg),
			})
		}

		embed.Fields = fields
	}

	ctx.Session.ChannelMessageSendEmbed(ctx.Message.ChannelID, embed)
}

func (c Toxic) getRatings(
	message string, ctx *multiplexer.Context,
) (map[string]*attribute, error) {
	req, err := http.NewRequest(
		"POST",
		fmt.Sprintf(fmtURL, c.Key),
		bytes.NewBuffer([]byte(fmt.Sprintf(fmtRequest, message))),
	)
	req.Header.Set("Content-Type", "application/json")
	if err != nil {
		return nil, err
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	res := response{}
	err = json.Unmarshal(body, &res)
	if err != nil {
		return nil, err
	}
	return res.AttributeScores, nil
}

func (c Toxic) getMessages(ctx *multiplexer.Context) ([]*discordgo.Message, error) {
	ctx.Session.ChannelTyping(ctx.Message.ChannelID)

	var messages []*discordgo.Message

	/* No arguments? Grab the previous message */
	if len(ctx.Arguments) == 0 {
		latestMessages, err := ctx.Session.ChannelMessages(
			ctx.Message.ChannelID, 1, ctx.Message.ID, "", "",
		)
		if err != nil {
			return messages, err
		}

		message := latestMessages[len(latestMessages)-1]
		if len(message.Content) <= 1 {
			return messages, fmt.Errorf("message too short to process")
		}

		return append(messages, message), nil
	}

	/* Is the argument supplied a generic ID? Grab that message */
	if util.IsID(ctx.Arguments[0]) {
		message, err := ctx.Session.ChannelMessage(
			ctx.Message.ChannelID, ctx.Arguments[0],
		)
		if err != nil {
			return messages, err
		}

		if len(message.Content) <= 1 {
			return messages, fmt.Errorf("message too short to process")
		}

		return append(messages, message), nil
	}

	/* Is the argument supplied a username? Grab their last few messages */
	if util.IsUser(ctx.Arguments[0]) {
		/* Convert the funky string to a user strictly for the error checking */
		user, err := ctx.Session.User(util.GetID(ctx.Arguments[0]))
		if err != nil {
			return messages, err
		}

		/* Grab the last 20 messages (unless specified otherwise) */
		limit := 20
		if len(ctx.Arguments) > 1 {
			i, err := strconv.Atoi(ctx.Arguments[1])
			if err != nil {
				ctx.ChannelSend("You didn't specify a valid number, defaulting to 20")
			}
			limit = i
		}

		/* DiscordGo only supports getting 100 messages */
		if limit > 100 {
			limit = 100
		}

		/* Get the messages and iterate through them, taking only the messages
		sent by the user */
		bulkMessages, err := ctx.Session.ChannelMessages(
			ctx.Message.ChannelID, limit, ctx.Message.ID, "", "",
		)
		if err != nil {
			return messages, err
		}

		for _, msg := range bulkMessages {
			if msg.Author.ID == user.ID {
				messages = append(messages, msg)
			}
		}

		if len(messages) == 0 {
			return messages, fmt.Errorf("not enough messages from the supplied user")
		}
		return messages, nil
	}

	return messages, fmt.Errorf(
		"the argument `%s` doesn't seem to be a message ID or username",
		ctx.Arguments[0],
	)
}

func (c Toxic) fixKey(key string) string {
	return strings.Title(strings.ToLower(strings.Replace(key, "_", " ", -1)))
}

// HandleHelp is called by whatever help command is in place when a user enters
// "!help [command name]".
func (c Toxic) HandleHelp(ctx *multiplexer.Context) bool {
	ctx.ChannelSendf(
		"`!%s` to check the previous message's toxicity levels\n"+
			"`!%s [username] [# messages]` to check how toxic the user in question has been\n"+
			"`!%s [message ID]` to check how toxic a specific message was\n",
		c.Command,
	)
	return true
}

// Settings is called by the multiplexer on startup to process any settings
// associated with that command.
func (c Toxic) Settings() *multiplexer.CommandSettings {
	return &multiplexer.CommandSettings{
		Command:  c.Command,
		HelpText: c.HelpText,
	}
}