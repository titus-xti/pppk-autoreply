package main

import (
    "context"
    "database/sql"
    "fmt"
    "net/http"
    "os"
    "os/signal"
    "syscall"

    _ "github.com/lib/pq"
    "go.mau.fi/whatsmeow"
    "go.mau.fi/whatsmeow/store"
    "go.mau.fi/whatsmeow/store/sqlstore"
    "go.mau.fi/whatsmeow/types"
    waLog "go.mau.fi/whatsmeow/util/log"
)

func main() {
    msg := `autoreplyon`

    ctx := context.Background()
    // Set latest WhatsApp Web version to avoid version mismatch issues
    if latest, err := whatsmeow.GetLatestVersion(ctx, nil); err == nil {
        store.SetWAVersion(*latest)
    } else {
        fmt.Printf("warn: failed to get latest WA version: %v\n", err)
    }

    // Start HTTP server for pairing/logs
    go func() {
        http.HandleFunc("/pairing", pairingPageHandler)
        http.HandleFunc("/events", sseEventsHandler)
        http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK); _, _ = w.Write([]byte("ok")) })
        addr := ":8080"
        fmt.Printf("HTTP server listening on %s. Open http://localhost%s/pairing\n", addr, addr)
        if err := http.ListenAndServe(addr, nil); err != nil {
            fmt.Printf("HTTP server error: %v\n", err)
        }
    }()

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
    sqlStore := sqlstore.NewWithDB(db, "postgres", waLog.Stdout("Database", "DEBUG", true))
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
        fmt.Printf("🔑 No session found, pairing required...\n")

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
                fmt.Printf("\n🔍 Scan the QR code with your phone (WhatsApp → Linked Devices → Link a Device):\n")
                broadcastQR(evt.Code)
            case "success":
                fmt.Printf("\n✅ Successfully paired with WhatsApp!\n")
            case "timeout":
                fmt.Printf("\n❌ QR code expired. Please restart the application to generate a new one.\n")
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

    // Target JID for message and attach event handler
    jid, _ := types.ParseJID("6281297898399@s.whatsapp.net")
    client.AddEventHandler(makeEventHandler(ctx, client, jid, msg))

    // wait so app doesn't exit immediately
    ch := make(chan os.Signal, 1)
    signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
    <-ch
}
