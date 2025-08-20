package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	qrcode "github.com/skip2/go-qrcode"
	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	_ "modernc.org/sqlite"
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

	// Use absolute path for SQLite database
	dbPath := "/app/session.db"
	db, err := sqlstore.New(ctx, "sqlite", "file:"+dbPath+"?_pragma=foreign_keys(ON)&_journal_mode=WAL", nil)
	if err != nil {
		panic(err)
	}
	deviceStore, err := db.GetFirstDevice(ctx)
	if err != nil {
		panic(err)
	}
	if deviceStore == nil {
		deviceStore = db.NewDevice()
	}
	client := whatsmeow.NewClient(deviceStore, nil)

	// Target JID for message
	jid, _ := types.ParseJID("6281297898399@s.whatsapp.net")

	client.AddEventHandler(makeEventHandler(ctx, client, jid, msg))

	if err := client.Connect(); err != nil {
		panic(err)
	}

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
		case *events.QR:
			handleQR(v)
		case *events.Message:
			handleIncomingMessage(ctx, client, v)
		case *events.Connected:
			once.Do(func() {
				go handleConnected(ctx, client, initialJID, initialMsg)
			})
		}
	}
}

// handleQR saves displayed QR codes as PNG files
func handleQR(v *events.QR) {
	os.Mkdir("qr", 0755)
	for i, code := range v.Codes {
		filename := fmt.Sprintf("qr/qr_%d.png", i)
		if err := qrcode.WriteFile(code, qrcode.Medium, 512, filename); err != nil {
			fmt.Println("failed to write QR PNG:", err)
		} else {
			fmt.Println("Saved QR to", filename)
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
	// Prefer extended text if present
	if etm := m.GetExtendedTextMessage(); etm != nil && etm.Text != nil {
		return etm.GetText()
	}
	// Plain conversation
	if c := m.GetConversation(); c != "" {
		return c
	}
	// Sometimes text can be inside ephemeral/container messages
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
	// (?i) makes the DAFTAR keyword case-insensitive
	re := regexp.MustCompile(`(?i)^DAFTAR-([^\-\r\n]+)-([^\-\r\n]+)-([0-9]{8})#$`)
	m := re.FindStringSubmatch(strings.TrimSpace(s))
	if len(m) != 4 {
		return "", "", "", false
	}
	n := strings.TrimSpace(m[1])
	wRaw := strings.TrimSpace(m[2])
	d := strings.TrimSpace(m[3])
	// wilayah: case-insensitive validation; normalize to lowercase canonical
	w := strings.ToLower(wRaw)
	switch w {
	case "pp1", "pp2", "serpong", "bukit", "reni":
		// ok
	default:
		return "", "", "", false
	}
	// tgl_lahir: must be a valid date in DDMMYYYY
	if _, err := time.Parse("02012006", d); err != nil {
		return "", "", "", false
	}
	return n, w, d, true
}
