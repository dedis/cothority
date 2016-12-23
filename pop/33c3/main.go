package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
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
var cookieName = "33c3-Cookie"

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

var database = newDatabase()

var staticDir = "static/"

func main() {
	if len(os.Args) != 3 {
		log.Fatal("go run main.go statement.txt schedule.json")
	}
	if b, err := ioutil.ReadFile(os.Args[1]); err != nil {
		log.Fatal(err)
	} else {
		statement = service.NewFinalStatementFromString(string(b))
	}

	database.load(os.Args[2])

	router := mux.NewRouter().StrictSlash(true)
	loadStaticRoutes(router, staticDir)
	router.Methods("GET").Path("/entries").HandlerFunc(Entries)
	router.Methods("GET").Path("/siginfo").HandlerFunc(SigningInfo)
	router.Methods("POST").Path("/login").HandlerFunc(Login)
	router.Methods("POST").Path("/vote").HandlerFunc(Vote)
	loggedRouter := handlers.LoggingHandler(os.Stdout, router)
	log.Fatal(http.ListenAndServeTLS(":8000", "server.crt", "server.key", loggedRouter))
}

func Entries(w http.ResponseWriter, r *http.Request) {
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
	if err := r.ParseForm(); err != nil {
		http.Error(w, "could not parse form", http.StatusInternalServerError)
		return
	}

	var update bool
	if r.Form.Get("update") != "" {
		update = true
	}
	entries, err := database.JSON(tag, update)
	if err != nil {
		http.Error(w, "invalid json representation", http.StatusInternalServerError)
		return
	}
	w.Write(entries)
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
