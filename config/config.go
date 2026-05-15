package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server    ServerConfig  `yaml:"server"`
	Databases []DBConfig    `yaml:"databases"`
	Redis     []RedisConfig `yaml:"redis"`
	CDN       CDNConfig     `yaml:"cdn"`
}

type CDNConfig struct {
	DesignBaseURL      string `yaml:"design_base_url"`
	HTTPTimeoutSeconds int    `yaml:"http_timeout_seconds"`
}

// Stage 서버가 떠 있는 환경 단계. config.yaml 의 server.stage 로 결정.
// 환경별 분기 (로그 레벨, admin 노출 범위, 디버그 엔드포인트 등) 에 사용.
type Stage string

const (
	StageLocal   Stage = "local"
	StageDev     Stage = "dev"
	StageQA      Stage = "qa"
	StageStaging Stage = "staging"
	StageLive    Stage = "live"
)

// validStages 허용된 stage 값 집합. 정확히 5 개로 제한.
var validStages = map[Stage]struct{}{
	StageLocal:   {},
	StageDev:     {},
	StageQA:      {},
	StageStaging: {},
	StageLive:    {},
}

// IsLive stage 가 live 인지 확인
func (s Stage) IsLive() bool {
	return s == StageLive
}

type ServerConfig struct {
	Port  string `yaml:"port"`
	Stage Stage  `yaml:"stage"` // local | dev | qa | staging | live
}

type DBConfig struct {
	Name     string `yaml:"name"`
	ShardID  int8   `yaml:"shard_id"`
	Weight   int    `yaml:"weight"` // 신규 유저 자동 할당 가중치. 0이면 자동 할당 풀에서 제외 (예: 시스템 DB).
	Host     string `yaml:"host"`
	Port     string `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	DBName   string `yaml:"dbname"`
}

type RedisConfig struct {
	Name     string `yaml:"name"`
	Host     string `yaml:"host"`
	Port     string `yaml:"port"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

func (c *DBConfig) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		c.User, c.Password, c.Host, c.Port, c.DBName,
	)
}

// Load 지정된 경로의 YAML 설정 파일을 읽고 파싱한다.
// server.stage 값을 검증해 허용된 5 개 중 하나가 아니면 에러 반환.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if _, ok := validStages[cfg.Server.Stage]; !ok {
		return nil, fmt.Errorf("invalid server.stage: %q (must be one of: local, dev, qa, staging, live)", cfg.Server.Stage)
	}

	return &cfg, nil
}
