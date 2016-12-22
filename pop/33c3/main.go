package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority/pop/service"
	"github.com/dedis/crypto/anon"
	"github.com/dedis/crypto/ed25519"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/gorilla/securecookie"
)

// linkable ring signature informations
var statement *service.FinalStatement
var context = []byte("33c3-demo-context")

// crypto suite used
var suite = ed25519.NewAES128SHA256Ed25519(false)

// custom made insecure session management
// 64 byte for auth with hmac and 32 bytes encr. for AES 256
var cookieHandler = securecookie.New(securecookie.GenerateRandomKey(64),
	securecookie.GenerateRandomKey(32))
var cookieName = "Session"

// session store / cookie related
// Cookies contains the tag provided when the user made the linkable ring
// signature
type sessionStore_ struct {
	sessions map[string]bool
	nonces   map[string]bool
	sync.Mutex
}

var sessionStore = sessionStore_{
	sessions: map[string]bool{},
	nonces:   map[string]bool{}}

var expireLimit = time.Hour * 3

// custom database yay
type database_ struct {
	db map[int]Entry_
	sync.Mutex
}

// Entry_ represents any conferences at 33c3
type Entry_ struct {
	Name        string
	Description string
	Location    string
	// map of tag => vote status
	Votes map[string]bool
}
type EntryJSON struct {
	Index       int
	Name        string
	Description string
	Location    string
	Up          int
	Down        int
	Voted       bool
}

var database = newDatabase()

var staticDir = "static/"

func main() {
	if len(os.Args) != 2 {
		log.Fatal("statement file as argument required.")
	}
	if b, err := ioutil.ReadFile(os.Args[1]); err != nil {
		log.Fatal(err)
	} else {
		statement = service.NewFinalStatementFromString(string(b))
	}

	database.load()

	router := mux.NewRouter().StrictSlash(true)
	loadStaticRoutes(router, staticDir)
	router.Methods("GET").Path("/entries").HandlerFunc(Entries)
	router.Methods("GET").Path("/siginfo").HandlerFunc(SigningInfo)
	router.Methods("POST").Path("/login").HandlerFunc(Login)
	router.Methods("POST").Path("/vote").HandlerFunc(Vote)
	loggedRouter := handlers.LoggingHandler(os.Stdout, router)
	log.Fatal(http.ListenAndServeTLS(":8080", "server.crt", "server.key", loggedRouter))
}

func Entries(w http.ResponseWriter, r *http.Request) {
	fmt.Print("yo")
	// check if user is registered or not
	var tag string
	var cookie *http.Cookie
	var err error
	if cookie, err = r.Cookie(cookieName); err == nil {
		value := make(map[string]string)
		if err = cookieHandler.Decode(cookieName, cookie.Value, &value); err == nil {
			tag = value["tag"]
		}
	}
	entriesJSON, err := database.JSON(tag)
	if err != nil {
		http.Error(w, "invalid json representation", http.StatusInternalServerError)
		return
	}
	w.Write(entriesJSON)
}

func SigningInfo(w http.ResponseWriter, r *http.Request) {
	var container struct {
		Attendees [][]byte
		Nonce     string
		Context   string
	}
	for _, p := range statement.Attendees {
		b, err := p.MarshalBinary()
		if err != nil {
			panic(err)
		}
		container.Attendees = append(container.Attendees, b)
	}
	container.Nonce = string(Secure32())
	sessionStore.NonceStore(container.Nonce)
	container.Context = string(context)
	toml.NewEncoder(w).Encode(container)
	var b bytes.Buffer
	toml.NewEncoder(&b).Encode(container)
}

// Fetch the signature from the client and verifies it. If the signature is
// correct, it replies with an authenticated cookie, otherwise it justs replies
// a status unauthorized code.
func Login(w http.ResponseWriter, r *http.Request) {
	var loginInfo struct {
		Nonce     []byte
		Signature []byte
	}
	if _, err := toml.DecodeReader(r.Body, &loginInfo); err != nil {
		http.Error(w, "could not read login info", http.StatusInternalServerError)
		return
	}

	/* if sessionStore.NonceDelete(loginInfo.Nonce) != nil {*/
	//http.Error(w, "nonce was never issued", http.StatusUnauthorized)
	//return
	//}
	fmt.Println("Nonce: ", hex.EncodeToString(loginInfo.Nonce))
	fmt.Println("Context: ", string(context))
	fmt.Println("Signature (", len(loginInfo.Signature), ") : ", hex.EncodeToString(loginInfo.Signature))
	ctag, err := anon.Verify(suite, loginInfo.Nonce, anon.Set(statement.Attendees), []byte(context), loginInfo.Signature)
	if err != nil {
		http.Error(w, "invalid signature verification", http.StatusUnauthorized)
		return
	}

	if sessionStore.Exists(string(ctag)) {
		http.Error(w, "already registered user", http.StatusTooManyRequests)
		return
	}

	// Signature is fine so let's give the user a cookie ;)
	sessionStore.Store(string(ctag))
	value := map[string]string{
		"tag": string(ctag),
	}

	if encoded, err := cookieHandler.Encode(cookieName, value); err == nil {
		cookie := &http.Cookie{
			Name:    cookieName,
			Value:   encoded,
			Path:    "/",
			Expires: time.Now().Add(expireLimit),
		}
		http.SetCookie(w, cookie)
	}
	w.WriteHeader(200)
	w.Write(ctag)
}

func Vote(w http.ResponseWriter, r *http.Request) {
	// parse cookie
	var cookie *http.Cookie
	var err error
	if cookie, err = r.Cookie(cookieName); err != nil {
		http.Error(w, "no cookie found", http.StatusInternalServerError)
		return
	}
	value := make(map[string]string)
	if err := cookieHandler.Decode(cookieName, cookie.Value, &value); err != nil {
		http.Error(w, "invalid cookie given", http.StatusInternalServerError)
		return
	}

	tag := value["tag"]
	// parse form
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	indexStr := r.Form.Get("index")
	voteStr := r.Form.Get("vote")
	var index int
	if index, err = strconv.Atoi(indexStr); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var vote bool
	if vote, err = strconv.ParseBool(voteStr); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// save the vote or error
	if err := database.VoteOrError(index, tag, vote); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(200)
}

func (st *sessionStore_) Store(tag string) {
	st.Lock()
	defer st.Unlock()
	st.sessions[tag] = true
}

func (st *sessionStore_) Exists(tag string) bool {
	st.Lock()
	defer st.Unlock()
	return st.sessions[tag]
}

func (st *sessionStore_) NonceStore(nonce string) {
	st.Lock()
	defer st.Unlock()
	st.nonces[nonce] = true
}

// return error if nonce was not present
func (st *sessionStore_) NonceDelete(nonce string) error {
	st.Lock()
	defer st.Unlock()
	if _, present := st.nonces[nonce]; !present {
		return errors.New("nonce non present")
	}
	delete(st.nonces, nonce)
	return nil
}
func newDatabase() *database_ {
	return &database_{db: map[int]Entry_{}}
}

// Returns the JSON representation with information including whether this tag
// has voted or not
func (d *database_) JSON(tag string) ([]byte, error) {
	d.Lock()
	defer d.Unlock()

	var entriesJSON []EntryJSON
	// list of entries
	for idx, entry := range d.db {
		_, voted := entry.Votes[tag]
		var up, down = 0, 0
		// count the votes
		for _, v := range entry.Votes {
			if v {
				up++
			} else {
				down++
			}
		}
		entriesJSON = append(entriesJSON, EntryJSON{
			Index:       idx,
			Name:        entry.Name,
			Description: entry.Description,
			Location:    entry.Location,
			Up:          up,
			Down:        down,
			Voted:       voted,
		})
	}
	return json.Marshal(entriesJSON)
}

func (d *database_) VoteOrError(entry int, tag string, vote bool) error {
	d.Lock()
	defer d.Unlock()
	e, ok := d.db[entry]
	if !ok {
		return errors.New("invalid entry id")
	}
	if v, ok := e.Votes[tag]; ok && v == vote {
		return errors.New("users already voted")
	}
	e.Votes[tag] = vote
	return nil
}

func (d *database_) load() {
	d.Lock()
	defer d.Unlock()
	for i, e := range []struct {
		Name        string
		Description string
		Location    string
	}{
		{"TLS 1.3", "The good, the bad and the ugly", "Saal 2"},
		{"The DROWN Attack", "Breaking TLS using SSLv2 (en)", "Saal 2"},
	} {
		d.db[i] = Entry_{
			Name:        e.Name,
			Description: e.Description,
			Location:    e.Location,
			Votes:       make(map[string]bool),
		}
	}
}

func Secure32() []byte {
	var n [32]byte
	rand.Read(n[:])
	return n[:]
}

func loadStaticRoutes(router *mux.Router, dir string) {
	// inspired from https://stackoverflow.com/questions/15834278/serving-static-content-with-a-root-url-with-the-gorilla-toolkit
	staticPaths := map[string]string{
		"css":              dir + "css/",
		"bower_components": dir + "bower_components/",
		"images":           dir + "images/",
		"js":               dir + "js/",
	}
	for pathName, pathValue := range staticPaths {
		pathPrefix := "/" + pathName + "/"
		router.PathPrefix(pathPrefix).Handler(http.StripPrefix(pathPrefix,
			http.FileServer(http.Dir(pathValue))))
	}
	// serve index.html at static/
	router.Handle("/", http.FileServer(http.Dir(dir)))
}
