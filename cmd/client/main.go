package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"future_next_baseball/pb"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// action defines a callable API action.
type action struct {
	id       uint32
	name     string
	userID   uint64                                         // envelope에 포함할 user_id (buildReq에서 설정)
	buildReq func(a *action, s *bufio.Scanner) (proto.Message, error)
	newResp  func() proto.Message
}

var actions = []action{
	{
		id:   1001,
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
		id:   2001,
		name: "GetCards",
		buildReq: func(a *action, s *bufio.Scanner) (proto.Message, error) {
			uid, err := promptUint64(s, "user_id")
			if err != nil {
				return nil, err
			}
			a.userID = uid
			return &pb.GetCardsRequest{}, nil
		},
		newResp: func() proto.Message { return &pb.GetCardsResponse{} },
	},
	{
		id:   2002,
		name: "UpgradeCardLevel",
		buildReq: func(a *action, s *bufio.Scanner) (proto.Message, error) {
			uid, err := promptUint64(s, "user_id")
			if err != nil {
				return nil, err
			}
			a.userID = uid
			cardIdx, err := promptUint64(s, "card_idx")
			if err != nil {
				return nil, err
			}
			return &pb.UpgradeCardLevelRequest{CardIdx: cardIdx}, nil
		},
		newResp: func() proto.Message { return &pb.UpgradeCardLevelResponse{} },
	},
}

func main() {
	addr := flag.String("addr", "http://localhost:8089/api", "server endpoint")
	clientVersion := flag.String("cv", "1.0.0", "client_version (envelope)")
	flag.Parse()

	scanner := bufio.NewScanner(os.Stdin)
	jsonOpts := protojson.MarshalOptions{Multiline: true, Indent: "  "}

	fmt.Println("=== Future Next Baseball Client ===")
	fmt.Printf("Server        : %s\n", *addr)
	fmt.Printf("client_version: %s (change with [v])\n", *clientVersion)

	for {
		fmt.Println()
		for _, a := range actions {
			fmt.Printf("  [%d] %s\n", a.id, a.name)
		}
		fmt.Printf("  [v] change client_version (current: %s)\n", *clientVersion)
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
			UserId:        act.userID,
			Timestamp:     time.Now().Unix(),
			ClientVersion: *clientVersion,
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

		fmt.Printf("\n--- Response (action=%d, code=%d) ---\n", gameResp.Action, gameResp.Code)

		if gameResp.Code != 200 {
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
		out, _ := jsonOpts.Marshal(respMsg)
		fmt.Println(string(out))
	}
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
