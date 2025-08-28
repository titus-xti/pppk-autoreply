package main

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"go-whatsapp.mastitus.my.id/repository"
	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

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

	// Pre-check: ensure the sender's phone is registered in vote_master
	// If not registered, immediately inform and return. If registered, capture name for greeting.
	var greetName string
	{
		phone := v.Info.Sender.User
		nm, chkErr := repository.VoteMasterPhoneExists(phone)
		if chkErr != nil {
			log.Printf("vote_master check error: %v", chkErr)
		} else if strings.TrimSpace(nm) == "" {
			_ = sendWithRetry(ctx, client, v.Info.Chat, addBackHint(NotRegistered), 1, 2*time.Second)
			return
		} else {
			greetName = nm
		}
	}

	// Normalize user input
	lower := strings.ToLower(raw)

	// Session key per sender number
	key := v.Info.Sender.User

	// Global command: back to menu
	if strings.Contains(lower, "back to main menu") || lower == "menu" || lower == "0" {
		setSessionMode(key, ModeMenu)
		reply := menuText(greetName)
		if err := sendWithRetry(ctx, client, v.Info.Chat, addBackHint(reply), 1, 2*time.Second); err != nil {
			logf("auto-reply error: %v\n", err)
		}
		return
	}

	sess := getSession(key)

	switch sess.Mode {
	case ModeMenu:
		// Numeric-only menu selections
		switch lower {
		case "1":
			setSessionMode(key, ModeInfo)
			if err := sendWithRetry(ctx, client, v.Info.Chat, addBackHint(infoPemilihanHelp("")), 1, 2*time.Second); err != nil {
				logf("auto-reply error: %v\n", err)
			}
			return
		case "2":
			setSessionMode(key, ModeRegistration)
			if err := sendWithRetry(ctx, client, v.Info.Chat, addBackHint(registrationHelp("")), 1, 2*time.Second); err != nil {
				logf("auto-reply error: %v\n", err)
			}
			return
		case "3":
			setSessionMode(key, ModeResendVote)
			if err := sendWithRetry(ctx, client, v.Info.Chat, addBackHint(resendVoteHelp("")), 1, 2*time.Second); err != nil {
				logf("auto-reply error: %v\n", err)
			}
			return
		default:
			// Default: show menu if not recognized
			if err := sendWithRetry(ctx, client, v.Info.Chat, addBackHint(menuText(greetName)), 1, 2*time.Second); err != nil {
				logf("auto-reply error: %v\n", err)
			}
			return
		}

	case ModeRegistration:
		name, wilayah, dob, msg, ok := parseRegistration(raw)
		var reply string
		if ok {
			existingName, existingWilayah, existingDOB, found, err := repository.GetRegistrationByPhone(v.Info.Sender.User)
			if err != nil {
				log.Printf("Error querying database: %v", err)
				reply = ErrorQuerying
			} else if found {
				reply = fmt.Sprintf(AlreadyRegistered,
					existingName, existingWilayah, existingDOB)
			} else {
				if err := repository.InsertRegistration(strings.ToUpper(name), strings.ToUpper(wilayah), dob, v.Info.Sender.User); err != nil {
					log.Printf("Error saving to database: %v", err)
					reply = ErrorSaving
				} else {
					reply = fmt.Sprintf(SuccessRegister,
						name, wilayah, dob)
				}
			}
		} else {
			reply = registrationHelp(msg)
		}

		if err := sendWithRetry(ctx, client, v.Info.Chat, addBackHint(reply), 1, 2*time.Second); err != nil {
			logf("auto-reply error: %v\n", err)
		}
		return
	}
}

func menuText(name string) string {
	base := OpeningGreeting
	if strings.TrimSpace(name) != "" {
		return fmt.Sprintf("Syaloom %s,\n\n%s", name, base)
	}
	return base
}

func registrationHelp(prefix string) string {
	return fmt.Sprintf(RegistrationHelp, prefix)
}

func resendVoteHelp(prefix string) string {
	return fmt.Sprintf(ResendVoteHelp, prefix)
}

func infoPemilihanHelp(prefix string) string {
	return fmt.Sprintf(InfoPemilihanHelp, prefix)
}

func addBackHint(s string) string {
	if strings.Contains(strings.ToLower(s), backHint) {
		return s
	}
	return s + backHint
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
		return "", "", "", ErrorWilayah, false
	}

	// Validate year is between 1900 and current year + 1
	currentYear := time.Now().Year()
	parsedYear, err := strconv.Atoi(year)
	if err != nil || parsedYear < 1900 || parsedYear > currentYear+1 {
		return "", "", "", ErrorDOB, false
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
