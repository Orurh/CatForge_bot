package main

import (
	"catforge/internal/observability"
	"context"
	"database/sql"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"io/fs"
	"log"
	"log/slog"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"catforge/internal/ai"
	"catforge/internal/app"
	"catforge/internal/assets"
	"catforge/internal/config"
	"catforge/internal/gameengine"
	"catforge/internal/logx"
	"catforge/internal/repo/postgres"
	tg "catforge/internal/transport/telegram"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func init() { rand.Seed(time.Now().UnixNano()) }

func main() {
	if len(os.Args) == 2 && os.Args[1] == "healthcheck" {
		client := &http.Client{Timeout: 4 * time.Second}
		resp, err := client.Get("http://127.0.0.1:8080/readyz")
		if err != nil {
			os.Exit(1)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			os.Exit(1)
		}
		return
	}
	cfg, err := config.Load()

	//logger
	level := slog.LevelInfo
	if cfg.Env == "dev" {
		level = slog.LevelDebug
	}
	base := slog.New(observability.LogHandler{Handler: slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})})
	slog.SetDefault(base)
	logger := logx.NewSlogAdapter(base).With(logx.String("svc", "catforge-bot"))

	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	//DB pool (pgxpool) для runtime
	pool, err := postgres.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer pool.Close()

	// Миграции (sql.DB под goose)
	sqldb, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("sql open: %v", err)
	}
	defer sqldb.Close()

	if err := postgres.Migrate(sqldb, cfg.MigrationsDir); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	// App wiring
	users := postgres.NewUserRepo(pool)
	clock := app.SystemClock{}
	rng := app.MathRNG{}
	engine, err := gameengine.NewRemoteEngine(cfg.GameEngineAddr)
	if err != nil {
		log.Fatalf("game engine: %v", err)
	}
	defer engine.Close()
	backupDir := os.Getenv("BACKUP_DIR")
	if backupDir == "" {
		backupDir = "./backups"
	}
	go runDependencyMonitor(ctx, pool, engine.Check, backupDir)
	if cfg.AIProvider != "procedural" {
		observability.AIMode.Set(1)
	}
	if cfg.TelegramMode == "polling" {
		observability.Polling.Set(1)
		observability.LastSuccess.WithLabelValues("telegram", "getUpdates").Set(0)
	}
	for _, worker := range []string{"banter", "event_resolve", "event_schedule"} {
		observability.InitializeWorker(worker)
	}
	logger.Info("using C++ game engine", logx.String("address", cfg.GameEngineAddr))
	cats := postgres.NewCatRepo(pool)
	personalities := postgres.NewPersonalityRepo(pool)
	quota := postgres.NewAIQuotaRepo(pool)
	catMessageRefs := postgres.NewCatMessageRefRepo(pool)
	yards := postgres.NewYardRepo(pool)
	yardEvents := postgres.NewYardEventRepo(pool, engine)
	fights := postgres.NewFightRepo(pool, engine)
	items := postgres.NewItemRepo(pool)
	updates := postgres.NewTelegramUpdateRepo(pool)
	eventStore := postgres.NewGameEventRepo(pool)

	// event log adapter (telegram)
	sender := tg.NewSender(cfg.TelegramToken, logger.With(logx.String("component", "telegram_sender")))
	telegramEvents := tg.NewTelegramGameEventSink(sender, catMessageRefs, logger.With(logx.String("component", "telegram_events")))
	events := app.NewGameEventFanout(eventStore, telegramEvents)
	var aiProvider ai.Provider
	if cfg.AIProvider == "openai" {
		aiProvider, err = ai.NewOpenAIProvider(cfg.AIAPIKey, cfg.AIBaseURL, cfg.AIModel, nil)
		if err != nil {
			log.Fatalf("AI provider: %v", err)
		}
	}
	if cfg.AIProvider == "codex" {
		aiProvider, err = ai.NewCodexProvider(cfg.CodexBridgeSocket, cfg.AIModel)
		if err != nil {
			log.Fatalf("Codex provider: %v", err)
		}
	}
	voice := ai.NewGateway(aiProvider, ai.NewFallbackProvider(), postgres.NewAIUsageRepo(pool), cfg.AITimeout)
	logger.Info("AI gateway configured", logx.String("provider", cfg.AIProvider))

	a := app.New(users, cats, personalities, quota, catMessageRefs, yards, yardEvents, fights, items, updates, engine, voice, clock, rng, events)
	a.Payments = app.NewPaymentService(postgres.NewPaymentRepo(pool), clock, cfg.PaymentSupportContact)
	go runPaymentThanks(ctx, a.Payments, sender, logger)
	go runYardEventResolver(ctx, a.YardEvent, logger.With(logx.String("component", "yard_event_resolver")))
	go runCatBanterScheduler(ctx, a.CatBanter, logger.With(logx.String("component", "cat_banter_scheduler")))

	router := tg.NewRouter(a, sender, cfg.PublicBaseURL, cfg.BotUsername, cfg.WebhookSecret, logger.With(logx.String("component", "telegram_router")))
	router.SetGroupCooldownStore(postgres.NewGroupCooldownRepo(pool))
	telegramBot := tg.New(a, cfg.TelegramToken, cfg.PublicBaseURL, cfg.WebhookPath, cfg.WebhookSecret)
	if err := telegramBot.RegisterCommands(ctx); err != nil {
		logger.Warn("Telegram command menu registration failed", logx.Any("err", err))
	} else {
		logger.Info("Telegram command menus registered")
	}
	if cfg.TelegramMode == "polling" {
		if err := telegramBot.DeleteWebhook(ctx, false); err != nil {
			log.Fatalf("deleteWebhook before polling: %v", err)
		}
		logger.Info("telegram long polling enabled")
		go runPolling(ctx, telegramBot, router, logger.With(logx.String("component", "telegram_polling")))
	} else if cfg.PublicBaseURL == "" {
		log.Printf("WARN: PUBLIC_BASE_URL is empty; skipping webhook registration (env=%s)", cfg.Env)
	} else if err := telegramBot.RegisterWebhook(ctx); err != nil {
		if cfg.Env == "dev" {
			log.Printf("WARN: setWebhook failed (dev mode): %v", err)
		} else {
			log.Fatalf("setWebhook: %v", err)
		}
	}

	mux := http.NewServeMux()
	staticFS, err := fs.Sub(assets.FS, "static")
	if err != nil {
		log.Fatalf("assets: %v", err)
	}
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	mux.HandleFunc(cfg.WebhookPath, router.WebhookHandler)
	mux.HandleFunc("/readyz", readinessHandler(pool.Ping, engine.Check))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", promhttp.Handler())
	metricsAddr := os.Getenv("METRICS_ADDR")
	if metricsAddr == "" {
		metricsAddr = "127.0.0.1:9091"
	}
	metricsServer := &http.Server{Addr: metricsAddr, Handler: metricsMux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("metrics server failed", logx.Any("err", err))
			stop()
		}
	}()

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("listening on %s (webhook path %s)", cfg.HTTPAddr, cfg.WebhookPath)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("http server failed", logx.Any("err", err))
			stop()
			return
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	_ = metricsServer.Shutdown(shutdownCtx)
}

func runCatBanterScheduler(ctx context.Context, service *app.CatBanterService, logger logx.Logger) {
	const interval = time.Minute
	run := func() {
		started := time.Now()
		count, err := service.RunDue(ctx, 20)
		observability.Observe("worker", "banter", started, err)
		if err != nil && ctx.Err() == nil {
			logger.Warn("idle cat banter scheduling failed", logx.Any("err", err))
			return
		}
		if count > 0 {
			logger.Info("idle cat banter published", logx.Int("count", count))
		}
	}
	run()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}

func runYardEventResolver(ctx context.Context, service *app.YardEventService, logger logx.Logger) {
	const (
		resolveInterval  = 15 * time.Second
		scheduleInterval = time.Minute
	)
	resolve := func() {
		started := time.Now()
		count, err := service.ResolveDue(ctx, 20)
		observability.Observe("worker", "event_resolve", started, err)
		if err != nil && ctx.Err() == nil {
			logger.Warn("yard event resolution failed", logx.Any("err", err))
			return
		}
		if count > 0 {
			logger.Info("yard events resolved", logx.Int("count", count))
		}
	}
	schedule := func() {
		started := time.Now()
		count, err := service.StartDue(ctx, 20)
		observability.Observe("worker", "event_schedule", started, err)
		if err != nil && ctx.Err() == nil {
			logger.Warn("yard event scheduling failed", logx.Any("err", err))
			return
		}
		if count > 0 {
			logger.Info("yard events started", logx.Int("count", count))
		}
	}
	resolve()
	schedule()
	resolveTicker := time.NewTicker(resolveInterval)
	scheduleTicker := time.NewTicker(scheduleInterval)
	defer resolveTicker.Stop()
	defer scheduleTicker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-resolveTicker.C:
			resolve()
		case <-scheduleTicker.C:
			schedule()
		}
	}
}

func runPolling(ctx context.Context, bot *tg.Bot, router *tg.Router, logger logx.Logger) {
	const retryDelay = 2 * time.Second
	for ctx.Err() == nil {
		if err := bot.Poll(ctx, router.HandleUpdate); err != nil && ctx.Err() == nil {
			logger.Warn("telegram polling interrupted; retrying", logx.Any("err", err), logx.Any("retry_in", retryDelay))
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(retryDelay):
		}
	}
}
