package main

import (
	"context"
	"errors"
	"future_next_baseball/internal/log"
	"future_next_baseball/internal/middleware"
	"future_next_baseball/internal/util"
	"net/http"
	"os/signal"
	"runtime"
	"time"

	"golang.org/x/sys/unix"

	"future_next_baseball/config"
	"future_next_baseball/internal/cache"
	"future_next_baseball/internal/database"
	"future_next_baseball/pb"
	"future_next_baseball/router"

	"github.com/labstack/echo/v4"
	"google.golang.org/protobuf/proto"
)

func main() {
	numCpu := runtime.NumCPU()
	runCpu := int(util.Round(float64(numCpu)*0.8, 0.5, 0))
	runtime.GOMAXPROCS(runCpu)

	cfg, err := config.Load("config.yaml")
	if err != nil {
		log.Fatal().Err(err).Msg("failed to config.Load")
	}

	// Database 접속 처리
	for _, dbCfg := range cfg.Databases {
		db, err := database.Init(dbCfg.Name, dbCfg.DSN())
		if err != nil {
			log.Fatal().Err(err).Msgf("failed to init database [%s]: %v", dbCfg.Name, err)
		}
		database.RegisterShard(dbCfg.ShardID, db)
		log.Info().Msgf("database [%s] connected (shard_id=%d)", dbCfg.Name, dbCfg.ShardID)
	}
	defer database.CloseAll()

	// Redis 접속 처리
	for _, rc := range cfg.Redis {
		if err := cache.Init(rc.Name, rc.Host, rc.Port, rc.Password, rc.DB); err != nil {
			log.Fatal().Err(err).Msgf("failed to init redis [%s]", rc.Name)
		}
		log.Info().Msgf("redis [%s] connected (db=%d)", rc.Name, rc.DB)
	}
	defer cache.CloseAll()

	e := echo.New()
	e.Use(middleware.RecoverMiddleware())
	e.Use(middleware.LogMiddleware())

	e.GET("/", func(c echo.Context) error {
		return sendProto(c, http.StatusOK, &pb.WelcomeResponse{
			Message: "Welcome to Future Next Baseball",
		})
	})

	e.GET("/health", func(c echo.Context) error {
		return sendProto(c, http.StatusOK, &pb.HealthResponse{
			Status: "ok",
		})
	})

	router.Setup(e)

	// setting server args
	srv := &http.Server{
		Addr:    ":" + cfg.Server.Port,
		Handler: e,
	}

	go func() {
		log.Info().Msgf("[START_WEBSHOP_SERVER] : (OS:%v, ARCH:%v, CPU:(%v/%v))", runtime.GOOS, runtime.GOARCH, runCpu, numCpu)
		//log.Info().Msgf("[SERVER_ENV] %v(%+v)", config.Config.Server.Mode, config.GetEnv())
		log.Info().Msgf("[SERVICE_PORT] : %v", cfg.Server.Port)

		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Panic().Err(err).Msg("failed to listen and serve on main")
		}
	}()

	runServer(srv)
}

// runServer 서버가 종료 시그널을 받을 때까지 대기한다.
func runServer(srv *http.Server) {
	sigCtx, stop := signal.NotifyContext(context.Background(), unix.SIGINT, unix.SIGTERM)
	defer stop()

	<-sigCtx.Done()

	log.Info().Msg("detect signal will be close the server")

	// 남은 요청을 처리할 수 있도록 3초의 시간을 둔다.
	timeCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	log.Info().Msg("will be shutdown after 3 seconds")

	if err := srv.Shutdown(timeCtx); err != nil {
		log.Fatal().Err(err).Msg("failed to shutdown")
	}

	<-timeCtx.Done()
}

func sendProto(c echo.Context, code int, msg proto.Message) error {
	data, err := proto.Marshal(msg)
	if err != nil {
		return err
	}
	return c.Blob(code, "application/x-protobuf", data)
}
