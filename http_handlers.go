package main

import (
    "bytes"
    "fmt"
    "html/template"
    "net/http"
    "strings"
    "sync"
    "time"

    qrcode "github.com/mdp/qrterminal/v3"
)

// LogHub broadcasts log lines to SSE subscribers and keeps a small backlog
type LogHub struct {
    mu      sync.Mutex
    subs    map[chan string]struct{}
    backlog []string
    maxBuf  int
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

var pairingTmpl = template.Must(template.New("pairing").Parse(`<!doctype html>
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

// pairingPageHandler serves a minimal UI that streams logs via SSE
func pairingPageHandler(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "text/html; charset=utf-8")
    if err := pairingTmpl.Execute(w, nil); err != nil {
        http.Error(w, "template render error", http.StatusInternalServerError)
        return
    }
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
