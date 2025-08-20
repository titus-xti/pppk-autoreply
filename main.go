package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	_ "modernc.org/sqlite"
	qrcode "github.com/mdp/qrterminal/v3"
	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
)

func main() {
	msg := `autoreplyon`

	ctx := context.Background()
	// Set latest WhatsApp Web version to avoid version mismatch issues
	if latest, err := whatsmeow.GetLatestVersion(ctx, nil); err == nil {
		store.SetWAVersion(*latest)
	} else {
		fmt.Println("warn: failed to get latest WA version:", err)
	}

	// Initialize logger
	logger := waLog.Stdout("Database", "DEBUG", true)

	// Use absolute path for SQLite database
	dbPath := "session.db"
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		panic(fmt.Errorf("failed to open database: %w", err))
	}

	// Initialize the SQLite database with the required schema
	sqlStore := sqlstore.NewWithDB(db, "sqlite3", logger)
	deviceStore, err := sqlStore.GetFirstDevice(ctx)
	if err != nil {
		panic(fmt.Errorf("failed to get device: %w", err))
	}
	if deviceStore == nil {
		deviceStore = sqlStore.NewDevice()
	}
	client := whatsmeow.NewClient(deviceStore, nil)

	// Create a background context if not already set
	if ctx == nil {
		ctx = context.Background()
	}

	// If first login (no session yet), request pairing code
	if client.Store.ID == nil {
		fmt.Println("🔑 No session found, pairing required...")

		// Get the QR code channel before connecting
		qrChan, err := client.GetQRChannel(context.Background())
		if err != nil {
			panic(fmt.Errorf("failed to get QR channel: %w", err))
		}

		// Connect to WhatsApp
		if err := client.Connect(); err != nil {
			panic(fmt.Errorf("failed to connect: %w", err))
		}

		for evt := range qrChan {
			if evt.Event == "code" {
				fmt.Println("Scan the QR code with your phone (WhatsApp → Linked Devices → Link a Device):")
				qrcode.GenerateHalfBlock(evt.Code, qrcode.L, os.Stdout)
			} else {
				fmt.Println("Login event:", evt.Event)
				if evt.Event == "success" {
					break
				} else if evt.Event == "timeout" {
					fmt.Println("QR code expired, please restart the application")
					return
				}
			}
		}
	}

	// Target JID for message
	jid, _ := types.ParseJID("6281297898399@s.whatsapp.net")

	// Add event handler after successful connection
	client.AddEventHandler(makeEventHandler(ctx, client, jid, msg))

	// wait so app doesn't exit immediately
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
}

func protoStr(s string) *string { return &s }

// makeEventHandler groups all event handling logic for readability
func makeEventHandler(ctx context.Context, client *whatsmeow.Client, initialJID types.JID, initialMsg string) func(evt interface{}) {
	var once sync.Once
	return func(evt interface{}) {
		switch v := evt.(type) {
		case *events.Message:
			handleIncomingMessage(ctx, client, v)
		case *events.Connected:
			once.Do(func() {
				go handleConnected(ctx, client, initialJID, initialMsg)
			})
		}
	}
}

// handleIncomingMessage logs and auto-replies "okay" to non-self messages
func handleIncomingMessage(ctx context.Context, client *whatsmeow.Client, v *events.Message) {
	from := v.Info.Sender.String()
	chat := v.Info.Chat.String()
	text := messageText(v.Message)
	if text == "" {
		text = "<non-text message>"
	}
	fmt.Printf("Incoming from %s in %s: %s\n", from, chat, text)
	autoReplyForIncoming(ctx, client, v)
}

// handleConnected sends the initial message once after connection stabilizes
func handleConnected(ctx context.Context, client *whatsmeow.Client, to types.JID, initialMsg string) {
	time.Sleep(1 * time.Second)
	if err := sendWithRetry(ctx, client, to, initialMsg, 5, 2*time.Second); err != nil {
		fmt.Println("send error:", err)
	}
}

// sendWithRetry sends a plain text message with simple retries and fixed backoff
func sendWithRetry(ctx context.Context, client *whatsmeow.Client, to types.JID, text string, attempts int, backoff time.Duration) error {
	for i := 0; i < attempts; i++ {
		if _, err := client.SendMessage(ctx, to, &waProto.Message{Conversation: protoStr(text)}); err != nil {
			if i == attempts-1 {
				return err
			}
			time.Sleep(backoff)
			continue
		}
		break
	}
	return nil
}

// messageText extracts a human-readable text from a WhatsApp message proto.
func messageText(m *waProto.Message) string {
	if m == nil {
		return ""
	}
	if etm := m.GetExtendedTextMessage(); etm != nil && etm.Text != nil {
		return etm.GetText()
	}
	if c := m.GetConversation(); c != "" {
		return c
	}
	if em := m.GetEphemeralMessage(); em != nil {
		if inner := messageText(em.GetMessage()); inner != "" {
			return inner
		}
	}
	if vm := m.GetViewOnceMessageV2(); vm != nil {
		if inner := messageText(vm.GetMessage()); inner != "" {
			return inner
		}
	}
	return ""
}

// autoReplyForIncoming decides and sends an auto-reply based on message format
func autoReplyForIncoming(ctx context.Context, client *whatsmeow.Client, v *events.Message) {
	if v.Info.IsFromMe {
		return
	}
	raw := strings.TrimSpace(messageText(v.Message))
	if raw == "" || raw == "<non-text message>" {
		return
	}
	name, wilayah, dob, ok := parseRegistration(raw)
	var reply string
	if ok {
		reply = fmt.Sprintf("Terima kasih. Data diterima:\nNama=%s\nWilayah=%s\nTgl Lahir=%s", name, wilayah, dob)
	} else {
		reply = "Format salah.\nGunakan: DAFTAR-NAMA-WILAYAH-TGL_LAHIR# \nTGL_LAHIR = DDMMYYYY\nWilayah pp1, pp2, serpong, bukit, reni\nContoh: DAFTAR-JOHN-pp1-01021990#"
	}
	if err := sendWithRetry(ctx, client, v.Info.Chat, reply, 1, 2*time.Second); err != nil {
		fmt.Println("auto-reply error:", err)
	}
}

// parseRegistration parses text like: DAFTAR-<name>-<wilayah>-<tgl_lahir>#
func parseRegistration(s string) (name, wilayah, dob string, ok bool) {
	re := regexp.MustCompile(`(?i)^DAFTAR-([^\-\r\n]+)-([^\-\r\n]+)-([0-9]{8})#$`)
	m := re.FindStringSubmatch(strings.TrimSpace(s))
	if len(m) != 4 {
		return "", "", "", false
	}
	n := strings.TrimSpace(m[1])
	wRaw := strings.TrimSpace(m[2])
	d := strings.TrimSpace(m[3])
	w := strings.ToLower(wRaw)
	switch w {
	case "pp1", "pp2", "serpong", "bukit", "reni":
	default:
		return "", "", "", false
	}
	if _, err := time.Parse("02012006", d); err != nil {
		return "", "", "", false
	}
	return n, w, d, true
}
