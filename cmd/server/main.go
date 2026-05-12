package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/aashorefp-droid/stockpulsego/internal/config"
	"github.com/aashorefp-droid/stockpulsego/internal/handlers"
	"github.com/aashorefp-droid/stockpulsego/internal/scheduler"
)

func main() {
	cfg := config.Load()

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(120 * time.Second))

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.CORSOrigins,
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   []string{"*"},
		AllowCredentials: true,
	}))

	srv := handlers.NewServer(cfg)

	// Daily snapshot service (NYSE/NASDAQ swing) and scheduler
	sched := scheduler.New(srv.Telegram, srv.Snapshot, srv.Scanner)
	if err := sched.Start(); err != nil {
		log.Printf("scheduler start failed: %v", err)
	}
	defer sched.Stop()

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		body := `{"status":"ok","scheduler":true,"polling_active":` + boolStr(sched.IsPolling()) + `}`
		w.Write([]byte(body))
	})

	srv.Register(r)

	httpSrv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 130 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		log.Printf("StockPulse Go API listening on :%s", cfg.Port)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	// Graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(ctx)
	log.Println("server stopped")
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
