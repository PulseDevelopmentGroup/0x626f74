package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/PulseDevelopmentGroup/0x626f74/command"
	"github.com/PulseDevelopmentGroup/0x626f74/config"
	"github.com/PulseDevelopmentGroup/0x626f74/log"
	"github.com/PulseDevelopmentGroup/0x626f74/multiplexer"

	"github.com/bwmarrin/discordgo"
	goenv "github.com/caarlos0/env/v6"
	_ "github.com/joho/godotenv/autoload"
	"github.com/patrickmn/go-cache"
)

type environment struct {
	Token          string `env:"BOT_TOKEN"`
	PerspectiveKey string `env:"PERSPECTIVE_KEY"`
	Debug          bool   `env:"DEBUG" envDefault:"false"`
	DataDir        string `env:"DATA_DIR" envDefault:"data/"`
	ConfigURL      string `env:"CONFIG_URL"`
	Fuzzy          bool   `env:"USE_FUZZY" envDefault:"false"`
}

var (
	env  = environment{}
	cfg  *config.BotConfig
	logs *log.Logs

	prefix = "!"
)

func init() {
	/* Parse enviorment variables */
	if err := goenv.Parse(&env); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	/* Check if URL is being specified */
	path := env.DataDir + "config.json"
	if len(env.ConfigURL) > 0 {
		path = env.ConfigURL
	}

	/* Parse config */
	var err error
	cfg, err = config.Get(path)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	/* Define logging setup */
	logs = log.New(env.Debug, cfg.ErrorChannel)
}

func main() {
	/* Initialize DiscordGo */
	logs.Primary.Info("Starting Bot...")
	dg, err := discordgo.New("Bot " + env.Token)
	if err != nil {
		logs.Primary.WithError(err).Error("Problem starting bot")
	}
	logs.Primary.Info("Bot started")

	/* Initialize Mux */
	mux, err := multiplexer.New(prefix)
	if err != nil {
		logs.Primary.WithError(err).Fatalf("Unable to create multixplexer")
	}

	/* Use the logging middleware with the multiplexer */
	mux.UseMiddleware(logs.MuxMiddleware)

	/* Set Permissions */
	mux.SetPermissions(cfg.Permissions)

	/* Setup Errors */
	mux.SetErrors(&multiplexer.ErrorTexts{
		CommandNotFound: "Command not found.",
		NoPermissions:   "You do not have permissions to execute that command.",
		RateLimited:     "You've used this command too many times, wait a bit and try again.",
	})

	/* === Register all the things === */
	mux.Register(
		command.Wiki{
			Command:      "wikirace",
			HelpText:     "Start a wikirace",
			RateLimitMax: 3,
			RateLimitDB:  cache.New(5*time.Minute, 5*time.Minute),
			Logger:       logs,
		},
		command.Gatekeeper{
			Command:  "role",
			HelpText: "Manage your access to roles, and their related channels",
			Logger:   logs,
		},
		command.Help{
			Command:  "help",
			HelpText: "Displays help  information regarding the bot's commands",
			Logger:   logs,
		},
		command.Inspire{
			Command:      "inspire",
			HelpText:     "Get an inspirational quote from inspirobot.me",
			RateLimitMax: 3,
			RateLimitDB:  cache.New(5*time.Minute, 5*time.Minute),
			Logger:       logs,
		},
		command.JPEG{
			Command:  "jpeg",
			HelpText: "More JPEG for the last image. 'nuff said",
			Logger:   logs,
		},
		command.LMGTFY{
			Command:      "googlehelp",
			HelpText:     "In case someone isn't familiar with Google",
			RateLimitMax: 2,
			RateLimitDB:  cache.New(30*time.Minute, 30*time.Minute),
		},
		command.Toxic{
			Command:      "toxic",
			HelpText:     "Someone really acting up? Get a toxicity rating.",
			Logger:       logs,
			Key:          env.PerspectiveKey,
			RateLimitDB:  cache.New(5*time.Minute, 5*time.Minute),
			RateLimitMax: 5,
		},
	)

	for k := range cfg.SimpleCommands {
		k := k
		mux.RegisterSimple(multiplexer.SimpleCommand{
			Command:  k,
			Content:  cfg.SimpleCommands[k],
			HelpText: "This is a simple command",
		})
	}

	/* Configure multiplexer options */
	mux.SetOptions(&multiplexer.Options{
		IgnoreDMs:        true,
		IgnoreBots:       true,
		IgnoreNonDefault: true,
		IgnoreEmpty:      true,
	})

	/* Initialize the commands */
	mux.Initialize()

	if env.Fuzzy {
		mux.UseFuzzy()
	}

	/* === End Register === */

	/* Handle commands and start DiscordGo */
	dg.AddHandler(mux.Handle)

	err = dg.Open()
	if err != nil {
		logs.Primary.WithError(err).Error(
			"Problem opening websocket connection.",
		)
		return
	}

	idle := 0
	dg.UpdateStatusComplex(discordgo.UpdateStatusData{
		IdleSince: &idle,
		Game: &discordgo.Game{
			Name: "you",
			Type: discordgo.GameTypeWatching,
			Assets: discordgo.Assets{
				LargeImageID: "watching",
				LargeText:    "Watching...",
			},
		},
		Status: "online",
	})

	defer dg.Close()

	/* Wait for interrupt */
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
}
