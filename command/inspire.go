package command

import (
	"io/ioutil"
	"net/http"

	"github.com/PulseDevelopmentGroup/0x626f74/log"
	"github.com/PulseDevelopmentGroup/0x626f74/multiplexer"
	"github.com/bwmarrin/discordgo"
	"github.com/patrickmn/go-cache"
)

// Inspire is a bot command
type Inspire struct {
	Command  string
	HelpText string

	Logger *log.Logs

	RateLimitMax int
	RateLimitDB  *cache.Cache
}

// Init is called by the multiplexer before the bot starts to initialize any
// variables the command needs.
func (c Inspire) Init(m *multiplexer.Mux) {
	// Nothing to init
}

// Handle is called by the multiplexer whenever a user triggers the command.
func (c Inspire) Handle(ctx *multiplexer.Context) {
	resp, err := http.Get("http://inspirobot.me/api?generate=true")
	if err != nil {
		c.Logger.CmdErr(ctx, err, "There was an error contacting the InspiroBot API")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			c.Logger.CmdErr(ctx, err, "Unable to parse InspiroBot response")
			return
		}

		_, err = ctx.Session.ChannelMessageSendEmbed(ctx.Message.ChannelID,
			&discordgo.MessageEmbed{
				Footer: &discordgo.MessageEmbedFooter{
					Text: "Like: 😄 | Delete: ❌",
				},
				Color: 0x6dd3ff,
				Image: &discordgo.MessageEmbedImage{
					URL: string(body),
				},
			},
		)

		if err != nil {
			c.Logger.CmdErr(ctx, err, "There was an issue sending the embed")
			return
		}
		/*
			w, err := disgoreact.NewWatcher(
				msg, ctx.Session,
				2*time.Second,
				ctx,
			)
			if err != nil {
				c.Logger.CmdErr(ctx, err, "There was an issue creating the reaction watcher")
				return
			}

			err = w.Add(
				disgoreact.Option{
					Emoji:      "😄",
					Expiration: 2 * time.Minute,
					OnSucess:   c.like,
					OnError:    c.reactErr,
				},
				disgoreact.Option{
					Emoji:      "❌",
					Expiration: 2 * time.Minute,
					OnSucess:   c.delete,
					OnError:    c.reactErr,
				},
			)
			if err != nil {
				c.Logger.CmdErr(ctx, err,
					"There was an issue adding a one of the reaction options",
				)
			}
		*/
		return
	}
	ctx.ChannelSend(
		"Sorry, I couldn't chat with InspiroBot. Maybe try again later?",
	)
}

/*
func (c Inspire) delete(
	user *discordgo.User,
	ctx *disgoreact.WatchContext,
) {
	ctx.Session.ChannelMessageDelete(ctx.Message.ChannelID, ctx.Message.ID)
}

func (c Inspire) like(
	user *discordgo.User,
	ctx *disgoreact.WatchContext,
) {
	usrCh, err := ctx.Session.UserChannelCreate(user.ID)
	if err != nil {
		c.reactErr(err, ctx)
		return
	}

	ctx.Session.ChannelMessageSend(
		usrCh.ID, "Glad you liked the inspirational quote! Here's the URL: "+
			ctx.Message.Embeds[0].Image.URL,
	)
}

func (c Inspire) reactErr(err error, ctx *disgoreact.WatchContext) {

}
*/
// HandleHelp is called by whatever help command is in place when a user enters
// "!help [command name]". If the help command is not being handled, return
// false.
func (c Inspire) HandleHelp(ctx *multiplexer.Context) bool {
	ctx.ChannelSend("`!inspire` for a free inspirational message generated by inspirobot.me!")
	return true
}

// Settings is called by the multiplexer on startup to process any settings
// associated with that command.
func (c Inspire) Settings() *multiplexer.CommandSettings {
	return &multiplexer.CommandSettings{
		Command:      c.Command,
		HelpText:     c.HelpText,
		RateLimitMax: c.RateLimitMax,
		RateLimitDB:  c.RateLimitDB,
	}
}
