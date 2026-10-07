// Command portal-api: backend Go của Portal báo cáo — đọc trực tiếp MongoDB.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"am-shortlink-portal/api/internal/audit"
	"am-shortlink-portal/api/internal/auth"
	"am-shortlink-portal/api/internal/cache"
	"am-shortlink-portal/api/internal/config"
	"am-shortlink-portal/api/internal/httpapi"
	"am-shortlink-portal/api/internal/httpapi/handler"
	"am-shortlink-portal/api/internal/mask"
	"am-shortlink-portal/api/internal/platform/logger"
	"am-shortlink-portal/api/internal/platform/mongodb"
	"am-shortlink-portal/api/internal/platform/redis"
	"am-shortlink-portal/api/internal/platform/telemetry"
	"am-shortlink-portal/api/internal/report"
	"am-shortlink-portal/api/internal/savedreport"
	"am-shortlink-portal/api/internal/store"
)

func main() {
	if err := run(); err != nil {
		slog.Error("portal-api stopped", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log, err := logger.New(logger.Options{
		Level: cfg.LogLevel, Service: cfg.OTel.ServiceName, Environment: cfg.Env,
		Brokers: cfg.Kafka.Brokers, Topic: cfg.Kafka.Topic,
	})
	if err != nil {
		return err
	}
	defer log.Close()
	slog.SetDefault(log.Logger)
	log.Info("starting portal-api", "version", telemetry.Version, "secrets_loaded", config.LoadedSecrets())

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	tel, err := telemetry.Setup(ctx, cfg.OTel, cfg.Env)
	if err != nil {
		return err
	}
	defer func() {
		sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = tel.Shutdown(sctx)
	}()

	db, err := mongodb.Connect(ctx, cfg.Mongo, cfg.OTel.Enabled)
	if err != nil {
		return err
	}
	defer db.Close(context.Background())

	rdb, err := redis.Connect(ctx, cfg.Redis, cfg.OTel.Enabled)
	if err != nil {
		return err
	}
	var reportCache cache.Cache = cache.Noop{}
	if rdb != nil {
		defer rdb.Close()
		reportCache = cache.NewRedis(rdb, cfg.Redis.KeyPrefix+"report:", cfg.Redis.Timeout)
		log.Info("report cache enabled", "ttl", cfg.Redis.ReportTTL, "ttl_past", cfg.Redis.ReportTTLPast)
	} else {
		log.Warn("REDIS_ADDR trống — tắt cache báo cáo, rate limit đăng nhập chỉ trong bộ nhớ pod")
	}

	st := store.New(db)
	tokens := auth.NewTokens(cfg.Auth.JWTSigningKey, cfg.Auth.JWTIssuer, cfg.Auth.AccessTTL)
	authSvc := auth.NewService(st, tokens, auth.Options{
		MaxFailedLogins: cfg.Auth.MaxFailedLogins,
		LockDuration:    cfg.Auth.LockDuration,
		RefreshTTL:      cfg.Auth.RefreshTTL,
	})
	masker := mask.NewMasker(cfg.PIIHashSalt)
	auditLog := audit.New(st)
	h := &handler.Handler{Auth: authSvc, Store: st, Masker: masker, Reports: report.NewService(st, masker).WithCache(reportCache, cfg.Redis.ReportTTL, cfg.Redis.ReportTTLPast).WithShortURLBase(cfg.ShortURLBase),
		Saved: savedreport.New(st, auditLog), Audit: auditLog}

	router, err := httpapi.NewRouter(httpapi.Deps{
		Handler:         h,
		Tokens:          tokens,
		Logger:          log.Logger,
		Ready:           db.Ping,
		Metrics:         tel.MetricsHandler,
		LoginRatePerMin: cfg.Auth.LoginRatePerMin,
		Redis:           rdb,
		RedisPrefix:     cfg.Redis.KeyPrefix,
	})
	if err != nil {
		return err
	}

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      120 * time.Second, // tải file export
		IdleTimeout:       120 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() {
		log.Info("listening", "addr", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
	case err := <-errCh:
		return err
	}
	log.Info("shutting down")
	sctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	return srv.Shutdown(sctx)
}
