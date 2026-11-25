package main

import (
	"EverythingSuckz/fsb/config"
	"EverythingSuckz/fsb/internal/bot"
	"EverythingSuckz/fsb/internal/cache"
	"EverythingSuckz/fsb/internal/commands"
	"EverythingSuckz/fsb/internal/routes"
	"EverythingSuckz/fsb/internal/store"
	"EverythingSuckz/fsb/internal/types"
	"EverythingSuckz/fsb/internal/utils"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

var runCmd = &cobra.Command{
	Use:                "run",
	Short:              "Run Ahar File Streamer with the given configuration.",
	DisableSuggestions: false,
	Run:                runApp,
}

var startTime time.Time = time.Now()

func runApp(cmd *cobra.Command, args []string) {
	// initialize logger early so config loading is logged to file
	utils.InitLogger(false)
	config.Load(utils.Logger, cmd)
	// reinitialize with correct dev mode
	utils.InitLogger(config.ValueOf.Dev)
	log := utils.Logger
	mainLogger := log.Named("Main")
	mainLogger.Info("Starting Ahar File Streamer")
	utils.Version = versionString

	if err := store.Init(config.ValueOf.DBPath); err != nil {
		mainLogger.Sugar().Fatalf("Failed to open database %s: %v", config.ValueOf.DBPath, err)
	}
	if len(config.ValueOf.Admins) == 0 {
		mainLogger.Warn("ADMINS is empty - admin commands are disabled until you set ADMINS=<your telegram id>")
	}
	startBackgroundJobs(mainLogger)
	router := getRouter(log)

	mainBot, err := bot.StartClient(log)
	if err != nil {
		log.Sugar().Fatalf("Failed to start main bot: %v", err)
	}
	cache.InitCache(log)
	workers, err := bot.StartWorkers(log)
	if err != nil {
		log.Sugar().Fatalf("Failed to start workers: %v", err)
	}
	workers.AddDefaultClient(mainBot, mainBot.Self)
	commands.WorkersInfo = func() []string {
		lines := make([]string, 0, len(bot.Workers.Bots))
		for _, w := range bot.Workers.Bots {
			lines = append(lines, fmt.Sprintf("#%d @%s", w.ID, w.Self.Username))
		}
		return lines
	}
	bot.StartUserBot(log)
	mainLogger.Info("Server started", zap.Int("port", config.ValueOf.Port))
	mainLogger.Info(config.ValueOf.BotName, zap.String("version", versionString))
	mainLogger.Sugar().Infof("Server is running at %s", config.ValueOf.Host)
	err = router.Run(fmt.Sprintf(":%d", config.ValueOf.Port))
	if err != nil {
		mainLogger.Sugar().Fatalf("Server failed to start: %v", err)
	}
}

func getRouter(log *zap.Logger) *gin.Engine {
	if config.ValueOf.Dev {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.Default()
	router.Use(gin.ErrorLogger())
	router.GET("/", func(ctx *gin.Context) {
		if routes.WantsHTML(ctx) {
			routes.ServeHome(ctx, versionString, startTime)
			return
		}
		ctx.JSON(http.StatusOK, types.RootResponse{
			Name:    config.ValueOf.BotName,
			Workers: len(bot.Workers.Bots),
			Message: "Server is running.",
			Ok:      true,
			Uptime:  utils.TimeFormat(uint64(time.Since(startTime).Seconds())),
			Version: versionString,
		})
	})
	routes.Load(log, router)
	return router
}

// startBackgroundJobs flushes the database on shutdown and prunes old link records.
func startBackgroundJobs(log *zap.Logger) {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		log.Info("Shutting down, saving database")
		if err := store.Default.Flush(); err != nil {
			log.Error("Failed to save database", zap.Error(err))
		}
		os.Exit(0)
	}()
	go func() {
		t := time.NewTicker(time.Hour)
		defer t.Stop()
		for range t.C {
			// keep expired records for a week so users can still see their status
			if n := store.Default.PurgeExpired(7 * 24 * time.Hour); n > 0 {
				log.Sugar().Infof("Purged %d expired link records", n)
			}
		}
	}()
}
