package main

import (
	"context"
	"database/sql"
	"bytes"
	"fmt"
	"net/http"
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

// global container for creating new devices on re-pair
var sqlContainer *sqlstore.Container

// LogHub broadcasts log lines to SSE subscribers and keeps a small backlog
type LogHub struct {
    mu      sync.Mutex
    subs    map[chan string]struct{}
    backlog []string
    maxBuf  int
}

// pairingPageHandler serves a minimal UI that streams logs via SSE
func pairingPageHandler(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "text/html; charset=utf-8")
    _, _ = w.Write([]byte(`<!doctype html>
<html><head><meta charset="utf-8"/>
<meta name="viewport" content="width=device-width, initial-scale=1"/>
<title>WhatsApp Pairing</title>
<style>
body { font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; margin:0; background:#0b0f14; color:#e6edf3; }
#bar { background:#11171f; padding:10px 14px; position:sticky; top:0; display:flex; gap:10px; align-items:center; }
button{ background:#238636; color:white; border:none; padding:6px 10px; border-radius:6px; cursor:pointer; }
button.secondary{ background:#30363d; }
#log { white-space:pre-wrap; padding:14px; line-height:1.4; }
.qr { color:#9be9a8; }
</style>
</head>
<body>
  <div id="bar">
    <strong>/pairing</strong>
    <button id="clear" class="secondary">Clear</button>
    <span id="status">connecting...</span>
  </div>
  <div id="log"></div>
  <script>
  const log = document.getElementById('log');
  const status = document.getElementById('status');
  document.getElementById('clear').onclick = () => { log.textContent = ''; };
  function append(line){
    const div = document.createElement('div');
    if(line.includes('Scan the QR code')) div.className='qr';
    div.textContent = line;
    log.appendChild(div);
    window.scrollTo(0, document.body.scrollHeight);
  }
  function connect(){
    const es = new EventSource('/events');
    es.onopen = () => { status.textContent='connected'; };
    es.onmessage = (ev) => append(ev.data);
    es.onerror = () => { status.textContent='disconnected, retrying...'; es.close(); setTimeout(connect, 2000); };
  }
  connect();
  </script>
</body></html>`))
}

// sseEventsHandler streams log lines to the browser via SSE including backlog
func sseEventsHandler(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")
    // Allow cross-origin simple access if needed
    w.Header().Set("Access-Control-Allow-Origin", "*")

    flusher, ok := w.(http.Flusher)
    if !ok {
        http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
        return
    }

    ch, backlog, cancel := hub.Subscribe()
    defer cancel()

    // Send backlog first
    for _, line := range backlog {
        fmt.Fprintf(w, "data: %s\n\n", line)
    }
    flusher.Flush()

    // Heartbeat ticker
    ticker := time.NewTicker(15 * time.Second)
    defer ticker.Stop()

    ctx := r.Context()
    for {
        select {
        case <-ctx.Done():
            return
        case line := <-ch:
            fmt.Fprintf(w, "data: %s\n\n", line)
            flusher.Flush()
        case <-ticker.C:
            fmt.Fprintf(w, ": ping\n\n")
            flusher.Flush()
        }
    }
}

var hub = &LogHub{subs: make(map[chan string]struct{}), maxBuf: 500}

func (h *LogHub) Broadcast(line string) {
    h.mu.Lock()
    // append to backlog
    h.backlog = append(h.backlog, line)
    if len(h.backlog) > h.maxBuf {
        h.backlog = h.backlog[len(h.backlog)-h.maxBuf:]
    }
    // send to subscribers (non-blocking)
    for ch := range h.subs {
        select { case ch <- line: default: }
    }
    h.mu.Unlock()
}

func (h *LogHub) Subscribe() (ch chan string, backlog []string, cancel func()) {
    ch = make(chan string, 100)
    h.mu.Lock()
    h.subs[ch] = struct{}{}
    // copy backlog
    if len(h.backlog) > 0 {
        backlog = append(backlog, h.backlog...)
    }
    h.mu.Unlock()
    cancel = func() {
        h.mu.Lock()
        delete(h.subs, ch)
        close(ch)
        h.mu.Unlock()
    }
    return
}

// logf writes to stdout and broadcasts to SSE clients
func logf(format string, args ...interface{}) {
    msg := fmt.Sprintf(format, args...)
    fmt.Print(msg)
    if len(msg) == 0 || msg[len(msg)-1] != '\n' {
        hub.Broadcast(msg)
    } else {
        hub.Broadcast(strings.TrimRight(msg, "\n"))
    }
}

// broadcastQR generates the half-block QR to an in-memory buffer and
// broadcasts it line-by-line to SSE clients so it renders in the web UI.
func broadcastQR(code string) {
    var buf bytes.Buffer
    qrcode.GenerateHalfBlock(code, qrcode.L, &buf)
    for _, line := range strings.Split(strings.TrimRight(buf.String(), "\n"), "\n") {
        if line == "" {
            continue
        }
        hub.Broadcast(line)
    }
}

func main() {
	msg := `autoreplyon`

	ctx := context.Background()
	// Set latest WhatsApp Web version to avoid version mismatch issues
	if latest, err := whatsmeow.GetLatestVersion(ctx, nil); err == nil {
		store.SetWAVersion(*latest)
	} else {
		logf("warn: failed to get latest WA version: %v\n", err)
	}

	// Start HTTP server for pairing/logs
	go func() {
		http.HandleFunc("/pairing", pairingPageHandler)
		http.HandleFunc("/events", sseEventsHandler)
		http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK); _, _ = w.Write([]byte("ok")) })
		addr := ":8080"
		logf("HTTP server listening on %s. Open http://localhost%s/pairing\n", addr, addr)
		if err := http.ListenAndServe(addr, nil); err != nil {
			logf("HTTP server error: %v\n", err)
		}
	}()

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
	sqlContainer = sqlStore

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
		logf("🔑 No session found, pairing required...\n")

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
				logf("\n🔍 Scan the QR code with your phone (WhatsApp → Linked Devices → Link a Device):\n")
				broadcastQR(evt.Code)
			case "success":
				logf("\n✅ Successfully paired with WhatsApp!\n")
			case "timeout":
				logf("\n❌ QR code expired. Please restart the application to generate a new one.\n")
				return
			case "error":
				logf("\n❌ Login error: %v\n", evt.Error)
				if evt.Error != nil {
					logf("Error details: %+v\n", evt.Error)
				}
				return
			default:
				logf("\nℹ️  Login event: %s\n", evt.Event)
				if evt.Error != nil {
					logf("Error details: %+v\n", evt.Error)
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
		case *events.LoggedOut:
			// Device was unlinked/logged out from phone or main device changed
			go handleLoggedOut(ctx, client, initialJID, v)
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
	logf("Incoming from %s in %s: %s\n", from, chat, text)

	// Mark message as read
	err := client.MarkRead([]string{v.Info.ID}, time.Now(), v.Info.Chat, v.Info.Sender)
	if err != nil {
		logf("Error marking message as read: %v\n", err)
	}

	autoReplyForIncoming(ctx, client, v)
}

// handleConnected sends the initial message once after connection stabilizes
func handleConnected(ctx context.Context, client *whatsmeow.Client, to types.JID, initialMsg string) {
	time.Sleep(1 * time.Second)
	if err := sendWithRetry(ctx, client, to, initialMsg, 5, 2*time.Second); err != nil {
		logf("send error: %v\n", err)
	}
}

// handleLoggedOut notifies and logs when the session gets unlinked/logged out
func handleLoggedOut(ctx context.Context, client *whatsmeow.Client, to types.JID, ev *events.LoggedOut) {
	// Build human-friendly reason
	var reason string
	if ev.OnConnect {
		// Triggered by a connect failure
		reason = fmt.Sprintf("Reason %s (%s)", ev.Reason.String(), ev.Reason.NumberString())
	} else {
		// Triggered by a stream:error while connected
		reason = "stream error while connected"
	}

	msg := fmt.Sprintf("This device was unlinked from WhatsApp. %s. Please re-link to continue.", reason)
	logf("%s\n", msg)
	// Try to notify the admin/initial JID; may fail if already disconnected
	_ = sendWithRetry(ctx, client, to, msg, 1, 2*time.Second)

    // Ensure we stop current connection
    client.Disconnect()

    // Delete current device state from store to allow clean re-pairing
    if err := client.Store.Delete(ctx); err != nil {
        		logf("failed to delete device store: %v\n", err)
	}
    // Extra safety: clean up any leftover devices to avoid FK issues
    if err := cleanupAllDevices(ctx); err != nil {
        		logf("failed to cleanup all devices: %v\n", err)
	}
    // Create/fetch a persisted fresh device store and client so inserts reference a valid our_jid
    newStore, err := sqlContainer.GetFirstDevice(ctx)
    if err != nil {
        		logf("failed to get new device store: %v\n", err)
		return
	}
    if newStore == nil {
        newStore = sqlContainer.NewDevice()
    }
    newClient := whatsmeow.NewClient(newStore, nil)

    // Re-attach event handler for the new client
    newClient.AddEventHandler(makeEventHandler(ctx, newClient, to, "autoreplyon"))

    // Start QR pairing flow to allow re-linking immediately with a fresh device
    go startPairingFlow(context.Background(), newClient)
}

// startPairingFlow starts the QR pairing flow
func startPairingFlow(ctx context.Context, client *whatsmeow.Client) {
    // Keep trying to get fresh QR codes until paired successfully
    for {
        // Get the QR code channel before connecting
        qrChan, err := client.GetQRChannel(ctx)
        if err != nil {
            			logf("Failed to get QR channel: %v\n", err)
			time.Sleep(2 * time.Second)
			continue
		}

        // Connect to WhatsApp
        if err := client.Connect(); err != nil {
            			logf("Failed to connect: %v\n", err)
			time.Sleep(2 * time.Second)
			continue
		}

        // Handle QR events until success/timeout/error
        retry := false
    QRChanLoop:
        for evt := range qrChan {
            switch evt.Event {
            case "code":
                logf("\n🔍 Scan the QR code with your phone (WhatsApp → Linked Devices → Link a Device):\n")
                broadcastQR(evt.Code)
            case "success":
                logf("\n✅ Successfully paired with WhatsApp!")
                return
            case "timeout":
                				logf("\n⏳ QR code expired. Generating a fresh QR...\n")
                retry = true
                break QRChanLoop
            case "error":
                logf("\n❌ Login error: %v\n", evt.Error)
                if evt.Error != nil {
                    logf("Error details: %+v\n", evt.Error)
                }
                retry = true
                break QRChanLoop
            default:
                logf("\nℹ️  Login event: %s\n", evt.Event)
                if evt.Error != nil {
                    logf("Error details: %+v\n", evt.Error)
                }
            }
        }

        if retry {
            // Small backoff before fetching a new QR
            time.Sleep(2 * time.Second)
            continue
        }

        // If channel closed without success or explicit retry, pause and try again
        time.Sleep(2 * time.Second)
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
	name, wilayah, dob, msg, ok := parseRegistration(raw)
	var reply string
	if ok {
		// Check if already registered
		db, err := sql.Open("postgres", "postgres://postgres:Gkjp2025@134.209.100.169:5432/vote?sslmode=disable")
		if err != nil {
			log.Printf("Error connecting to database: %v", err)
			reply = "Maaf, terjadi kesalahan sistem. Silakan coba lagi nanti."
		} else {
			defer db.Close()

			// Check if phone number already exists
			var existingName, existingWilayah, existingDOB string
			err = db.QueryRow("SELECT name, wilayah, year_of_birth FROM registration WHERE phone_number = $1",
				v.Info.Sender.User).Scan(&existingName, &existingWilayah, &existingDOB)

			if err == nil {
				// Phone number exists, show existing data
				reply = fmt.Sprintf("Anda sudah terdaftar sebelumnya dengan data:\nNama: %s\nWilayah: %s\nTahun Lahir: %s\n\nData tidak dapat diubah. \n\nHubungi panitia di nomor 081297898399 jika ada kesalahan data.",
					existingName, existingWilayah, existingDOB)
			} else if err == sql.ErrNoRows {
				// Phone number doesn't exist, insert new registration
				_, err = db.Exec(`
                    INSERT INTO registration (name, wilayah, year_of_birth, phone_number, created_at)
                    VALUES ($1, $2, $3, $4, NOW())
                `, strings.ToUpper(name), strings.ToUpper(wilayah), dob, v.Info.Sender.User)

				if err != nil {
					log.Printf("Error saving to database: %v", err)
					reply = "Maaf, terjadi kesalahan saat menyimpan data. Silakan coba lagi."
				} else {
					reply = fmt.Sprintf("Terima kasih!\nPendaftaran pemilihan online berhasil dengan data sebagai berikut:\nNama: %s\nWilayah: %s\nTahun Lahir: %s",
						name, wilayah, dob)
				}
			} else {
				// Other database error
				log.Printf("Error querying database: %v", err)
				reply = "Maaf, terjadi kesalahan sistem. Silakan coba lagi nanti."
			}
		}
	} else {
		reply = fmt.Sprintf("%sUntuk mendaftar pemilihan online, silahkan kirim pesan dengan format:\nDAFTAR-<nama lengkap jemaat>-<wilayah pelayanan>-<tahun lahir># \n\nTahun Lahir 4 Digit, Contoh: 1985\nWilayah pp1/pp2/serpong/bukit/reni\n\nContoh:\nDAFTAR-James Munthe-pp1-1972#\nDAFTAR-Maria Fatmitasari-pp2-1972#\nDAFTAR-Ery Setiawan-bukit-1972#\nDAFTAR-Florencia Irena-reni-1980#\nDAFTAR-Titus Adi Prasetyo-serpong-1985#", msg)
	}

	if err := sendWithRetry(ctx, client, v.Info.Chat, reply, 1, 2*time.Second); err != nil {
		logf("auto-reply error: %v\n", err)
	}
}

// parseRegistration parses text like: DAFTAR-<name>-<wilayah>-<year>#
func parseRegistration(s string) (name, wilayah, dob, message string, ok bool) {
	re := regexp.MustCompile(`(?i)^DAFTAR\s*-\s*([^\-\r\n]+?)\s*-\s*([^\-\r\n]+?)\s*-\s*(\d{4})\s*#\s*$`)
	m := re.FindStringSubmatch(s)
	if len(m) != 4 {
		return "", "", "", "Syaloom Bp/Ibu,\n\n", false
	}
	n := strings.TrimSpace(m[1])
	wRaw := strings.TrimSpace(m[2])
	year := strings.TrimSpace(m[3])
	w := strings.ToLower(wRaw)

	switch w {
	case "pp1", "pp2", "serpong", "bukit", "reni":
	default:
		return "", "", "", "Wilayah pelayanan tidak valid. Silahkan isi pp1/pp2/serpong/bukit/reni\n\n", false
	}

	// Validate year is between 1900 and current year + 1
	currentYear := time.Now().Year()
	parsedYear, err := strconv.Atoi(year)
	if err != nil || parsedYear < 1900 || parsedYear > currentYear+1 {
		return "", "", "", "Tahun lahir tidak valid. Silahkan isi tahun lahir 4 digit\n\n", false
	}

	return n, w, year, "", true
}

// cleanupAllDevices removes all whatsmeow device rows from the SQL store
func cleanupAllDevices(ctx context.Context) error {
    if sqlContainer == nil {
        return nil
    }
    devs, err := sqlContainer.GetAllDevices(ctx)
    if err != nil {
        return err
    }
    for _, d := range devs {
        if err := d.Delete(ctx); err != nil {
            return err
        }
    }
    return nil
}
