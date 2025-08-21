package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	_ "github.com/lib/pq"
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

	// PostgreSQL connection string
	pgConnStr := "postgres://postgres:Gkjp2025@134.209.100.169:5432/autoreply?sslmode=disable"

	// Open PostgreSQL connection
	db, err := sql.Open("postgres", pgConnStr)
	if err != nil {
		panic(fmt.Errorf("failed to connect to PostgreSQL: %w", err))
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		panic(fmt.Errorf("failed to ping PostgreSQL: %w", err))
	}

	// Initialize the PostgreSQL database with the required schema
	sqlStore := sqlstore.NewWithDB(db, "postgres", logger)

	// Ensure the database tables are created
	err = sqlStore.Upgrade(ctx)
	if err != nil {
		panic(fmt.Errorf("failed to upgrade database: %w", err))
	}

	// Get or create device
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
			switch evt.Event {
			case "code":
				fmt.Println("\n🔍 Scan the QR code with your phone (WhatsApp → Linked Devices → Link a Device):")
				qrcode.GenerateHalfBlock(evt.Code, qrcode.L, os.Stdout)
			case "success":
				fmt.Println("\n✅ Successfully paired with WhatsApp!")
			case "timeout":
				fmt.Println("\n❌ QR code expired. Please restart the application to generate a new one.")
				return
			case "error":
				fmt.Printf("\n❌ Login error: %v\n", evt.Error)
				if evt.Error != nil {
					fmt.Printf("Error details: %+v\n", evt.Error)
				}
				return
			default:
				fmt.Printf("\nℹ️  Login event: %s\n", evt.Event)
				if evt.Error != nil {
					fmt.Printf("Error details: %+v\n", evt.Error)
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

// handleIncomingMessage logs, marks messages as read, and auto-replies to non-self messages
func handleIncomingMessage(ctx context.Context, client *whatsmeow.Client, v *events.Message) {
	from := v.Info.Sender.String()
	chat := v.Info.Chat.String()
	text := messageText(v.Message)
	if text == "" {
		text = "<non-text message>"
	}
	fmt.Printf("Incoming from %s in %s: %s\n", from, chat, text)

	// Mark message as read
	err := client.MarkRead([]string{v.Info.ID}, time.Now(), v.Info.Chat, v.Info.Sender)
	if err != nil {
		log.Printf("Error marking message as read: %v", err)
	}

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
		reply = fmt.Sprintf("Terima kasih!\nPendaftaran pemilihan online berhasil dengan data sebagai berikut:\nNama=%s\nWilayah=%s\nTahun Lahir=%s", name, wilayah, dob)
	} else {
		reply = "Format pendaftaran salah.\nGunakan: DAFTAR-<nama lengkap jemaat>-<wilayah pelayanan>-<tahun lahir># \n\nTahun Lahir 4 Digit, Contoh: 1985\nWilayah pp1/pp2/serpong/bukit/reni\n\nContoh:\nDAFTAR-James Munthe-pp1-1972#\nDAFTAR-Maria Fatmitasari-pp2-1972#\nDAFTAR-Ery Setiawan-bukit-1972#\nDAFTAR-Florencia Irena-reni-1980#\nDAFTAR-Titus Adi Prasetyo-serpong-1985#"
	}
	if err := sendWithRetry(ctx, client, v.Info.Chat, reply, 1, 2*time.Second); err != nil {
		fmt.Println("auto-reply error:", err)
	}
}

// parseRegistration parses text like: DAFTAR-<name>-<wilayah>-<year>#
func parseRegistration(s string) (name, wilayah, dob string, ok bool) {
	re := regexp.MustCompile(`(?i)^DAFTAR-([^\-\r\n]+)-([^\-\r\n]+)-(\d{4})#$`)
	m := re.FindStringSubmatch(strings.TrimSpace(s))
	if len(m) != 4 {
		return "", "", "", false
	}
	n := strings.TrimSpace(m[1])
	wRaw := strings.TrimSpace(m[2])
	year := strings.TrimSpace(m[3])
	w := strings.ToLower(wRaw)

	switch w {
	case "pp1", "pp2", "serpong", "bukit", "reni":
	default:
		return "", "", "", false
	}

	// Validate year is between 1900 and current year + 1
	currentYear := time.Now().Year()
	parsedYear, err := strconv.Atoi(year)
	if err != nil || parsedYear < 1900 || parsedYear > currentYear+1 {
		return "", "", "", false
	}

	return n, w, year, true
}
