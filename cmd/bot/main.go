package main

import (
	"context"
	"database/sql"
	"io/fs"
	"log"
	"log/slog"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"catforge/internal/app"
	"catforge/internal/assets"
	"catforge/internal/config"
	"catforge/internal/logx"
	"catforge/internal/repo/postgres"
	tg "catforge/internal/transport/telegram"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func init() { rand.Seed(time.Now().UnixNano()) }

func main() {
	cfg, err := config.Load()

	//logger
	level := slog.LevelInfo
	if cfg.Env == "dev" {
		level = slog.LevelDebug
	}
	base := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
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
	cats := postgres.NewCatRepo(pool, rng)
	daily := postgres.NewDailyRepo(pool)

	// event log adapter (telegram)
	sender := tg.NewSender(cfg.TelegramToken, logger.With(logx.String("component", "telegram_sender")))
	ev := tg.NewTelegramEventLog(sender, logger.With(logx.String("component", "telegram_eventlog")))
	arena := postgres.NewArenaRepo(pool)

	a := app.New(users, cats, daily, arena, clock, rng, ev)

	// Telegram webhook registration
	if cfg.PublicBaseURL == "" {
		log.Printf("WARN: PUBLIC_BASE_URL is empty; skipping webhook registration (env=%s)", cfg.Env)
	} else {
		bot := tg.New(a, cfg.TelegramToken, cfg.PublicBaseURL, cfg.WebhookPath)
		if err := bot.RegisterWebhook(ctx); err != nil {
			if cfg.Env == "dev" {
				log.Printf("WARN: setWebhook failed (dev mode): %v", err)
			} else {
				log.Fatalf("setWebhook: %v", err)
			}
		}
	}

	router := tg.NewRouter(a, sender, cfg.PublicBaseURL, cfg.BotUsername, logger.With(logx.String("component", "telegram_router")))

	mux := http.NewServeMux()
	staticFS, err := fs.Sub(assets.FS, "static")
	if err != nil {
		log.Fatalf("assets: %v", err)
	}
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	mux.HandleFunc(cfg.WebhookPath, router.WebhookHandler)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

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
}
