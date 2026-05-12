package main

import (
	"context"
	"errors"
	"future_was/internal/log"
	"future_was/internal/middleware"
	"future_was/internal/resource"
	"future_was/internal/util"
	"net/http"
	"os/signal"
	"runtime"
	"time"

	"golang.org/x/sys/unix"

	"future_was/config"
	"future_was/internal/cache"
	"future_was/internal/container"
	"future_was/internal/database"
	"future_was/internal/design"
	"future_was/internal/repository"
	"future_was/pb"
	"future_was/router"

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

	// TB_VERSION은 shard_id=0 (게임 DB)에 위치한다. 등록되지 않았으면 즉시 종료.
	gameDB := database.GetGameDB()
	if gameDB == nil {
		log.Fatal().Msg("shard_id=0 (game DB) not registered")
	}

	// 디자인 데이터: TB_VERSION 기반 자동 로드 + Pub/Sub 동기화
	designLoader := design.NewLoader(cfg.CDN.DesignBaseURL, cfg.CDN.HTTPTimeoutSeconds)
	designStore := design.NewStore()
	designSyncer := design.NewSyncer(designStore, designLoader, cache.Get(cache.NameReloadPubSub), repository.NewVersionRepository(gameDB))
	{
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.CDN.HTTPTimeoutSeconds)*time.Second)
		err := designSyncer.LoadActive(ctx)
		cancel()
		if err != nil {
			log.Fatal().Err(err).Msg("failed to load active design versions")
		}
	}

	// 리소스 데이터 로드 + Pub/Sub 동기화
	resourceLoader := resource.NewLoader(gameDB)
	resourceStore := resource.NewStore()
	resourceSyncer := resource.NewSyncer(resourceStore, resourceLoader, cache.Get(cache.NameReloadPubSub))
	{
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := resourceSyncer.LoadAll(ctx)
		cancel()
		if err != nil {
			log.Fatal().Err(err).Msg("failed to load resource data")
		}
	}

	syncerCtx, syncerCancel := context.WithCancel(context.Background())
	designSyncer.Start(syncerCtx)
	resourceSyncer.Start(syncerCtx)

	// 종료 시: ctx 취소 → syncer 고루틴 종료 대기.
	defer func() {
		syncerCancel()
		designSyncer.Wait()
		resourceSyncer.Wait()
	}()

	// Container 구성
	ctn := container.New(designStore, designSyncer, resourceStore, resourceSyncer)

	e := echo.New()
	e.Use(middleware.RecoverMiddleware())
	e.Use(middleware.LogMiddleware())
	e.Use(middleware.MaintenanceMiddleware(resourceStore))

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

	router.Setup(e, ctn)

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
