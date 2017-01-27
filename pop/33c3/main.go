package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"encoding/json"

	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority/pop/service"
	"github.com/dedis/crypto/anon"
	"github.com/dedis/crypto/ed25519"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/gorilla/securecookie"
	"gopkg.in/dedis/onet.v1/log"
)

const (
	dbName    = "votes.db"
	storeName = "store.db"
)

// linkable ring signature informations
var statement *service.FinalStatement
var context = []byte("33c3-demo-context")

// crypto suite used
var suite = ed25519.NewAES128SHA256Ed25519(false)

// custom made insecure session management
// 64 byte for auth with hmac and 32 bytes encr. for AES 256
var cookieName = "33c3-Cookie"

var sessionStore = newSessionStore()

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

	//database.load(os.Args[2])
	database.VotesLoad(os.Args[2], dbName)
	sessionStore.Load(storeName)

	router := mux.NewRouter().StrictSlash(true)
	loadStaticRoutes(router, staticDir)
	router.Methods("GET").Path("/entries").HandlerFunc(entries)
	router.Methods("GET").Path("/siginfo").HandlerFunc(signingInfo)
	router.Methods("POST").Path("/login").HandlerFunc(login)
	router.Methods("POST").Path("/vote").HandlerFunc(vote)
	loggedRouter := handlers.LoggingHandler(os.Stdout, router)
	log.Fatal(http.ListenAndServeTLS(":8000", "server.crt", "server.key", loggedRouter))
	//log.Fatal(http.ListenAndServe(":8000", loggedRouter))
}

func entries(w http.ResponseWriter, r *http.Request) {
	// check if user is registered or not
	var tag []byte
	var cookie *http.Cookie
	var err error
	if cookie, err = r.Cookie(cookieName); err == nil {
		value := make(map[string]string)
		if err = sessionStore.SecureCookie.Decode(cookieName, cookie.Value, &value); err == nil {
			tag = []byte(value["tag"])
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

func signingInfo(w http.ResponseWriter, r *http.Request) {
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
	nonce := secure32()
	container.Nonce = string(nonce)
	sessionStore.NonceStore(nonce)
	container.Context = string(context)
	toml.NewEncoder(w).Encode(container)
	var b bytes.Buffer
	toml.NewEncoder(&b).Encode(container)
}

// Fetch the signature from the client and verifies it. If the signature is
// correct, it replies with an authenticated cookie, otherwise it justs replies
// a status unauthorized code.
func login(w http.ResponseWriter, r *http.Request) {
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

	if !sessionStore.Exists(ctag) {
		// Signature is fine so let's give the user a cookie ;)
		sessionStore.Store(ctag)
	}

	value := map[string]string{
		"tag": string(ctag),
	}

	if encoded, err := sessionStore.SecureCookie.Encode(cookieName, value); err == nil {
		cookie := &http.Cookie{
			Name:    cookieName,
			Value:   encoded,
			Expires: time.Now().Add(expireLimit),
		}
		http.SetCookie(w, cookie)
	}
	sessionStore.Save(storeName)
	w.WriteHeader(200)
	w.Write(ctag)
}

func vote(w http.ResponseWriter, r *http.Request) {
	// parse cookie
	var cookie *http.Cookie
	var err error
	if cookie, err = r.Cookie(cookieName); err != nil {
		http.Error(w, "no cookie found", http.StatusInternalServerError)
		return
	}
	value := make(map[string]string)
	if err := sessionStore.SecureCookie.Decode(cookieName, cookie.Value, &value); err != nil {
		http.Error(w, "invalid cookie given", http.StatusInternalServerError)
		return
	}

	tag := []byte(value["tag"])
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

	if err := database.VotesSave(dbName); err != nil {
		log.Error(err)
	}
	w.WriteHeader(200)
}

// session store / cookie related
// Cookies contains the tag provided when the user made the linkable ring
// signature
type sessionStoreStruct struct {
	Sessions     [][]byte
	Nonces       [][]byte
	HashKey      []byte
	BlockKey     []byte
	SecureCookie *securecookie.SecureCookie
	sync.Mutex
}

func newSessionStore() *sessionStoreStruct {
	st := &sessionStoreStruct{
		Sessions: [][]byte{},
		Nonces:   [][]byte{},
		HashKey:  securecookie.GenerateRandomKey(64),
		BlockKey: securecookie.GenerateRandomKey(32),
	}
	st.SecureCookie = securecookie.New(st.HashKey, st.BlockKey)
	return st
}

func (st *sessionStoreStruct) Store(tag []byte) {
	st.Lock()
	defer st.Unlock()
	log.Printf("Storing %x", tag)
	st.Sessions = append(st.Sessions, tag)
}

func (st *sessionStoreStruct) Exists(tag []byte) bool {
	st.Lock()
	defer st.Unlock()
	for _, t := range st.Sessions {
		if bytes.Equal(t, tag) {
			return true
		}
	}
	return false
}

func (st *sessionStoreStruct) NonceStore(nonce []byte) {
	st.Lock()
	defer st.Unlock()
	st.Nonces = append(st.Nonces, nonce)
}

// return error if nonce was not present
func (st *sessionStoreStruct) NonceDelete(nonce []byte) error {
	st.Lock()
	defer st.Unlock()
	for i, n := range st.Nonces {
		if bytes.Equal(n, nonce) {
			st.Nonces = append(st.Nonces[:i], st.Nonces[i+1:]...)
			return nil
		}
	}
	return errors.New("nonce non present")
}

// Save stores the sessionStore into the file 'name'.
func (st *sessionStoreStruct) Save(name string) error {
	file, err := os.OpenFile(name, os.O_RDWR+os.O_CREATE, 0660)
	if err != nil {
		return err
	}
	if err = json.NewEncoder(file).Encode(st); err != nil {
		return err
	}
	return file.Close()
}

// Load tries to load the content of the file 'name' into the sessionstore.
func (st *sessionStoreStruct) Load(name string) error {
	_, err := os.Stat(name)
	if err != nil {
		return err
	}
	file, err := os.Open(name)
	if err != nil {
		return err
	}
	defer file.Close()
	err = json.NewDecoder(file).Decode(st)
	if err != nil {
		return err
	}
	st.SecureCookie = securecookie.New(st.HashKey, st.BlockKey)
	return nil
}

func secure32() []byte {
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
