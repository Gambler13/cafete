package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"text/template"
	"time"
)

func main() {

	t := template.New("index.html")
	t, err := t.ParseFiles("index.html") // Parse template file.
	if err != nil {
		panic(err)
	}

	ti := template.New("info.html")
	ti, err = ti.ParseFiles("info.html") // Parse template file.
	if err != nil {
		panic(err)
	}

	username := os.Getenv("USERNAME")
	password := os.Getenv("PASSWORD")

	s := Server{
		Index:    t,
		Info:     ti,
		Results:  make([]Event, 0),
		Password: password,
		Username: username,
	}

	s.startFetchJson(context.Background())

	r := mux.NewRouter()
	r.HandleFunc("/index.html", s.handleRequest)
	r.HandleFunc("/info.html", s.handleInfo)
	r.HandleFunc("/", s.handleRequest)
	r.HandleFunc("/info", s.handleInfoRequest)
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./static/"))))
	r.PathPrefix("/").Handler(http.FileServer(http.Dir("./")))
	log.Fatal(http.ListenAndServe(":8989", r))

}

type Server struct {
	Username string
	Password string
	InfoText string
	Index    *template.Template
	Info     *template.Template
	Results  []Event
}

func (s *Server) startFetchJson(ctx context.Context) {
	go func() {
		fmt.Println("fetch initial event json")
		s.Results = processJson(fetchJson())

		after := time.After(1 * time.Hour)
		for {
			select {
			case <-after:
				fmt.Println("fetch event json")
				s.Results = processJson(fetchJson())
				after = time.After(1 * time.Hour)
			case <-ctx.Done():
				return
			default:

			}
		}
	}()
}

func fetchJson() []string {
	spreadshetId := os.Getenv("GOOGLE_SPREADSHEET_ID")
	apiKey := os.Getenv("GOOGLE_API_KEY")
	url := fmt.Sprintf("https://sheets.googleapis.com/v4/spreadsheets/%s/values/Data?key=%s", spreadshetId, apiKey)
	res, err := http.Get(url)
	if err != nil {
		fmt.Printf("could not get json data: %v\n", err.Error())
	}

	out := make(map[string]interface{})

	decoder := json.NewDecoder(res.Body)
	err = decoder.Decode(&out)

	if err != nil {
		fmt.Printf("could not decode json data: %v\n", err.Error())
		return nil
	}

	v, ok := out["values"]
	if !ok {
		fmt.Errorf("not a []string")
		return nil
	}

	tmpList, okList := v.([]interface{})
	if !okList {
		fmt.Errorf("not a []string")
		return nil
	}

	list := make([]string, len(tmpList))

	for i := range tmpList {
		v, ok := tmpList[i].([]interface{})
		if ok && len(v) > 0 {
			sv, ok := v[0].(string)
			if ok {
				if sv == "(leer)" {
					sv = ""
				}
				list[i] = sv
			}
		}
	}

	return list
}

type Event struct {
	Date        string
	Title       string
	Acts        string
	Style       string
	Description string
	Link1       Link
	Link2       Link
	Link3       Link
	Link4       Link
	Link5       Link
}

type Link struct {
	Text string
	Url  string
}

func processJson(lines []string) []Event {
	events := make([]Event, 0)
	for i := 1; i < len(lines)-10; i = i + 10 {
		if lines[i] == "" {
			continue
		}
		e := Event{
			Date:        lines[i],
			Title:       lines[i+1],
			Acts:        lines[i+2],
			Style:       lines[i+3],
			Description: lines[i+4],
			Link1: Link{
				Text: sanitizeLink(lines[i+5]),
				Url:  lines[i+5],
			},
			Link2: Link{
				Text: sanitizeLink(lines[i+6]),
				Url:  lines[i+6],
			},
			Link3: Link{
				Text: sanitizeLink(lines[i+7]),
				Url:  lines[i+7],
			},
			Link4: Link{
				Text: sanitizeLink(lines[i+8]),
				Url:  lines[i+8],
			},
			Link5: Link{
				Text: sanitizeLink(lines[i+9]),
				Url:  lines[i+9],
			},
		}
		events = append(events, e)
	}

	return events
}

func sanitizeLink(link string) string {
	link = strings.TrimPrefix(link, "http://")
	link = strings.TrimPrefix(link, "https://")
	link = strings.TrimPrefix(link, "www.")
	link = strings.TrimSuffix(link, "\n")
	links := strings.Split(link, "?")
	return strings.TrimSuffix(links[0], "/")
}

func (s *Server) handleRequest(w http.ResponseWriter, r *http.Request) {
	events := s.Results

	data := struct {
		Events   []Event
		InfoText string
	}{
		Events: events,
		//InfoText: "Achtung: Beim Einlass gelten die 3G-Regeln!\n=> Registrierung vorab auf https://covtr.app/",
		InfoText: s.InfoText,
	}

	err := s.Index.Execute(w, data) // merge.
	if err != nil {
		fmt.Println("error occured: " + err.Error())
	}
}

func (s *Server) handleInfo(w http.ResponseWriter, r *http.Request) {

	if err := s.checkBasicAuth(w, r); err != nil {
		return
	}

	data := struct {
		InfoText string
	}{
		InfoText: s.InfoText,
	}

	err := s.Info.Execute(w, data) // merge.
	if err != nil {
		fmt.Println("error occured: " + err.Error())
	}
}

func (s *Server) handleInfoRequest(w http.ResponseWriter, r *http.Request) {

	if err := s.checkBasicAuth(w, r); err != nil {
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	text := string(body)
	text = strings.TrimPrefix(text, "text=")
	s.InfoText = strings.TrimSpace(text)

	http.Redirect(w, r, "/index.html", http.StatusPermanentRedirect)
}

func (s *Server) checkBasicAuth(w http.ResponseWriter, r *http.Request) error {
	w.Header().Set("WWW-Authenticate", `Basic realm="riot.fm"`)
	if auth := r.Header.Get("Authorization"); auth == "" {
		w.WriteHeader(http.StatusUnauthorized)
		return fmt.Errorf("Authorization header not found")
	} else {
		creds64 := strings.TrimPrefix(auth, "Basic ")
		creds, _ := base64.StdEncoding.DecodeString(creds64)
		credsArray := strings.SplitN(string(creds), ":", 2)
		if len(credsArray) != 2 || credsArray[0] != s.Username || credsArray[1] != s.Password {
			w.WriteHeader(http.StatusUnauthorized)
			return fmt.Errorf("Wrong credentials")
		}
	}

	return nil
}
