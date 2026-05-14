package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"future_was/internal/handler"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"future_was/pb"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// ANSI 색상 코드. macOS/Linux 기본 터미널에서 정상 출력.
const (
	colorGreen = "\033[32m"
	colorCyan  = "\033[36m"
	colorReset = "\033[0m"
)

// action defines a callable API action.
type action struct {
	id       uint32
	name     string
	userID   uint64 // envelope에 포함할 user_id (buildReq에서 설정)
	buildReq func(a *action, s *bufio.Scanner) (proto.Message, error)
	newResp  func() proto.Message
}

var actions = []action{
	{
		id:   handler.ActionLogin,
		name: "Login",
		buildReq: func(a *action, s *bufio.Scanner) (proto.Message, error) {
			uid, err := promptUint64(s, "channel_uid")
			if err != nil {
				return nil, err
			}
			did := promptString(s, "device_id")
			return &pb.LoginRequest{ChannelUid: uid, DeviceId: did}, nil
		},
		newResp: func() proto.Message { return &pb.LoginResponse{} },
	},
	{
		id:   handler.ActionGetCards,
		name: "GetCards",
		buildReq: func(a *action, s *bufio.Scanner) (proto.Message, error) {
			return &pb.GetCardsRequest{}, nil
		},
		newResp: func() proto.Message { return &pb.GetCardsResponse{} },
	},
	{
		id:   handler.ActionUpgradeCardLevel,
		name: "UpgradeCardLevel",
		buildReq: func(a *action, s *bufio.Scanner) (proto.Message, error) {
			cardIdx, err := promptUint64(s, "card_idx")
			if err != nil {
				return nil, err
			}
			return &pb.UpgradeCardLevelRequest{CardIdx: cardIdx}, nil
		},
		newResp: func() proto.Message { return &pb.UpgradeCardLevelResponse{} },
	},
	{
		id:   handler.ActionGetItems,
		name: "GetItems",
		buildReq: func(a *action, s *bufio.Scanner) (proto.Message, error) {
			return &pb.GetItemsRequest{}, nil
		},
		newResp: func() proto.Message { return &pb.GetItemsResponse{} },
	},
	{
		id:   handler.ActionGetAssets,
		name: "GetAssets",
		buildReq: func(a *action, s *bufio.Scanner) (proto.Message, error) {
			return &pb.GetAssetsRequest{}, nil
		},
		newResp: func() proto.Message { return &pb.GetAssetsResponse{} },
	},
	{
		id:   handler.ActionGetShopList,
		name: "GetShopList",
		buildReq: func(a *action, s *bufio.Scanner) (proto.Message, error) {
			return &pb.GetShopListRequest{}, nil
		},
		newResp: func() proto.Message { return &pb.GetShopListResponse{} },
	},
}

func main() {
	addr := flag.String("addr", "http://localhost:8089/api", "server endpoint")
	clientVersion := flag.String("cv", "1.0.0", "client_version (envelope)")
	flag.Parse()

	scanner := bufio.NewScanner(os.Stdin)
	jsonOpts := protojson.MarshalOptions{Multiline: true, Indent: "  "}

	// Login 응답으로 받은 세션 토큰. 이후 모든 요청 envelope에 자동 첨부된다.
	var sessionToken string
	var userID uint64

	fmt.Println("=== Future Next Baseball Client ===")
	fmt.Printf("Server        : %s\n", *addr)
	fmt.Printf("client_version: %s (change with [v])\n", *clientVersion)

	for {
		fmt.Println()
		for _, a := range actions {
			fmt.Printf("  [%d] %s\n", a.id, a.name)
		}
		fmt.Printf("  [v] change client_version (current: %s)\n", *clientVersion)
		fmt.Printf("  user_id : %v\n", userID)
		fmt.Printf("  session_token : %s\n", shortToken(sessionToken))
		fmt.Println("  [0] Exit")
		fmt.Println()

		fmt.Print("Action> ")
		scanner.Scan()
		input := strings.TrimSpace(scanner.Text())
		if input == "v" || input == "V" {
			*clientVersion = promptString(scanner, "new client_version")
			fmt.Printf("client_version set to: %s\n", *clientVersion)
			continue
		}
		id64, err := strconv.ParseUint(input, 10, 32)
		if err != nil {
			fmt.Println("invalid input")
			continue
		}
		id := uint32(id64)
		if id == 0 {
			fmt.Println("Bye!")
			return
		}

		act := findAction(id)
		if act == nil {
			fmt.Printf("unknown action: %d\n", id)
			continue
		}

		reqMsg, err := act.buildReq(act, scanner)
		if err != nil {
			fmt.Printf("input error: %v\n", err)
			continue
		}

		innerBytes, err := proto.Marshal(reqMsg)
		if err != nil {
			fmt.Printf("marshal error: %v\n", err)
			continue
		}

		envelope := &pb.GameRequest{
			Action:        act.id,
			UserId:        userID,
			Timestamp:     time.Now().Unix(),
			ClientVersion: *clientVersion,
			SessionToken:  sessionToken,
			Body:          innerBytes,
		}
		envBytes, err := proto.Marshal(envelope)
		if err != nil {
			fmt.Printf("envelope marshal error: %v\n", err)
			continue
		}

		resp, err := http.Post(*addr, "application/x-protobuf", bytes.NewReader(envBytes))
		if err != nil {
			fmt.Printf("request failed: %v\n", err)
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		var gameResp pb.GameResponse
		if err := proto.Unmarshal(body, &gameResp); err != nil {
			fmt.Printf("response unmarshal error: %v\n", err)
			fmt.Printf("  HTTP status : %s\n", resp.Status)
			fmt.Printf("  Content-Type: %s\n", resp.Header.Get("Content-Type"))
			if len(body) < 512 {
				fmt.Printf("  raw body    : %s\n", string(body))
			} else {
				fmt.Printf("  raw body (first 512): %s\n", string(body[:512]))
			}
			continue
		}

		if gameResp.Code != 200 {
			fmt.Printf("\n--- Response (action=%d, code=%d) ---\n", gameResp.Action, gameResp.Code)
			var errResp pb.ErrorResponse
			if err := proto.Unmarshal(gameResp.Body, &errResp); err == nil {
				fmt.Printf("Error: %s\n", errResp.Message)
			} else {
				fmt.Printf("raw body: %x\n", gameResp.Body)
			}
			continue
		}

		respMsg := act.newResp()
		if err := proto.Unmarshal(gameResp.Body, respMsg); err != nil {
			fmt.Printf("body unmarshal error: %v\n", err)
			continue
		}

		// Login 성공 시 세션 토큰을 보관하여 이후 요청 envelope에 자동 첨부한다.
		if lr, ok := respMsg.(*pb.LoginResponse); ok {
			userID = lr.UserId
			sessionToken = lr.SessionToken
		}

		out, _ := jsonOpts.Marshal(respMsg)
		fmt.Printf("\n%s--- Response (action=%d, code=%d) ---\n%s%s\n",
			colorGreen, gameResp.Action, gameResp.Code, string(out), colorReset)

		// envelope sync: 이번 요청에서 서버가 변경한 엔티티(자동 첨부).
		if gameResp.Sync != nil {
			syncJSON, _ := jsonOpts.Marshal(gameResp.Sync)
			fmt.Printf("\n%s--- Sync ---\n%s%s\n", colorCyan, string(syncJSON), colorReset)
		}
	}
}

// shortToken 토큰을 짧게 표시한다 (디버깅용).
func shortToken(t string) string {
	if t == "" {
		return "(none)"
	}
	if len(t) <= 12 {
		return t
	}
	return t[:8] + "..."
}

func findAction(id uint32) *action {
	for i := range actions {
		if actions[i].id == id {
			return &actions[i]
		}
	}
	return nil
}

// --- prompt helpers ---

func promptString(s *bufio.Scanner, label string) string {
	fmt.Printf("%s> ", label)
	s.Scan()
	return strings.TrimSpace(s.Text())
}

func promptUint64(s *bufio.Scanner, label string) (uint64, error) {
	str := promptString(s, label)
	return strconv.ParseUint(str, 10, 64)
}

func promptUint32(s *bufio.Scanner, label string) (uint32, error) {
	v, err := promptUint64(s, label)
	return uint32(v), err
}
