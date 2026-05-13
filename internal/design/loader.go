package design

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"future_was/internal/design/schema"
	"future_was/internal/log"
)

// Manifest CDN의 manifest.json 형식.
type Manifest struct {
	Version string `json:"version"`
	Files   []struct {
		Name     string `json:"name"`
		Checksum string `json:"checksum"` // "sha256:abc..."
	} `json:"files"`
}

// Loader 지정 버전의 디자인 파일을 CDN에서 다운로드하여 Catalog으로 구성한다.
type Loader struct {
	baseURL    string
	httpClient *http.Client
}

func NewLoader(baseURL string, timeoutSeconds int) *Loader {
	return &Loader{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: time.Duration(timeoutSeconds) * time.Second,
		},
	}
}

// Load CDN에서 manifest 조회 후 각 파일을 다운로드/검증/파싱하여 Catalog 반환.
func (l *Loader) Load(ctx context.Context, version string) (*Catalog, error) {
	manifest, err := l.fetchManifest(ctx, version)
	if err != nil {
		return nil, fmt.Errorf("fetch manifest: %w", err)
	}

	catalog := NewCatalog(version)

	for _, f := range manifest.Files {
		data, err := l.fetchFile(ctx, version, f.Name, f.Checksum)
		if err != nil {
			return nil, fmt.Errorf("fetch %s: %w", f.Name, err)
		}
		if err := unmarshalInto(catalog, f.Name, data); err != nil {
			return nil, fmt.Errorf("unmarshal %s: %w", f.Name, err)
		}
		log.Info().Msgf("design loaded file:%s", f.Name)
	}

	return catalog, nil
}

func (l *Loader) fetchManifest(ctx context.Context, version string) (*Manifest, error) {
	url := fmt.Sprintf("%s/%s/manifest.json", l.baseURL, version)
	data, err := l.httpGet(ctx, url)
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

func (l *Loader) fetchFile(ctx context.Context, version, name, expectedChecksum string) ([]byte, error) {
	url := fmt.Sprintf("%s/%s/%s", l.baseURL, version, name)
	data, err := l.httpGet(ctx, url)
	if err != nil {
		return nil, err
	}
	if expectedChecksum != "" {
		if err := verifyChecksum(data, expectedChecksum); err != nil {
			return nil, err
		}
	}
	return data, nil
}

func (l *Loader) httpGet(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := l.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http %d: %s", resp.StatusCode, url)
	}
	return io.ReadAll(resp.Body)
}

func verifyChecksum(data []byte, expected string) error {
	const prefix = "sha256:"
	if !strings.HasPrefix(expected, prefix) {
		return fmt.Errorf("unsupported checksum format: %s", expected)
	}
	want := strings.TrimPrefix(expected, prefix)
	sum := sha256.Sum256(data)
	got := hex.EncodeToString(sum[:])
	if got != want {
		return fmt.Errorf("checksum mismatch: got %s, want %s", got, want)
	}
	return nil
}

// unmarshalInto 파일명에 따라 적절한 도메인 Table에 JSON 데이터를 채운다.
// 새 도메인 추가 시 case를 추가하면 된다.
func unmarshalInto(c *Catalog, name string, data []byte) error {
	switch name {
	case "BAT_DATA.json":
		var list []*schema.BatDataDesign
		if err := json.Unmarshal(data, &list); err != nil {
			return err
		}
		for _, v := range list {
			c.batData.set(v.Nid, v)

			// UseFlag = true인 카드들 별도 관리
			if v.Useflag {
				c.batDataByUseFlag.set(v.Nid, v)
			}
		}
	case "CURRENCY_LIST.json":
		var list []*schema.CurrencyListDesign
		if err := json.Unmarshal(data, &list); err != nil {
			return err
		}
		for _, v := range list {
			c.currency.set(v.ItemId, v)
		}
	case "ITEM_LIST.json":
		var list []*schema.ItemListDesign
		if err := json.Unmarshal(data, &list); err != nil {
			return err
		}
		for _, v := range list {
			c.item.set(v.ItemId, v)
		}
	default:
		// 알 수 없는 파일은 무시 (향후 추가될 도메인 대비).
	}
	return nil
}
