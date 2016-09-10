package identity

import (
	"encoding/base64"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/dedis/cothority/crypto"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/sda"
)

type jsonID struct {
	address string
	service *Service
}

func NewJsonID(s *Service, address string) {
	jid := &jsonID{
		address: address,
		service: s,
	}

	server := http.NewServeMux()
	server.HandleFunc("/cu", jid.cu)
	server.HandleFunc("/ps", jid.ps)
	server.HandleFunc("/pu", jid.pu)
	server.HandleFunc("/pv", jid.pv)

	go func() {
		http.ListenAndServe(address, server)
	}()
}

const (
	corruptRequest   = 500
	invalidJson      = 501
	cothorityFailure = 502
	noProposed       = 503
	invalidBase64    = 504
)

type jsonConfig struct {
	Threshold int
	Device    map[string]string
	Data      map[string]string
}

type jsonConfigUpdate struct {
	ID string
}

type jsonCreateIdentity struct {
	Config *jsonConfig
	Roster *sda.Roster
}

type jsonProposeSend struct {
	ID      string
	Propose *jsonConfig
}

type jsonProposeUpdate struct {
	ID string
}

type jsonProposeVote struct {
	ID        string
	Signer    string
	Signature string
}

type jsonQR struct {
	ID   string
	Host string
	Port string
}

func createDeviceMap(pubs map[string]string) map[string]*Device {
	device := make(map[string]*Device)
	for k, v := range pubs {
		reader := strings.NewReader(v)
		point, _ := crypto.ReadPub64(network.Suite, reader)
		device[k] = &Device{Point: point}
	}
	return device
}

func createJsonConfig(c *Config) *jsonConfig {
	jc := &jsonConfig{Threshold: c.Threshold,
		Device: make(map[string]string), Data: c.Data}
	for k, v := range c.Device {
		p, _ := crypto.Pub64(network.Suite, v.Point)
		jc.Device[k] = p
	}
	return jc
}

func (jid *jsonID) cu(w http.ResponseWriter, r *http.Request) {
	req, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(corruptRequest)
		return
	}

	jcu := jsonConfigUpdate{}
	if err = json.Unmarshal(req, &jcu); err != nil {
		w.WriteHeader(invalidJson)
		return
	}

	id64, err := base64.StdEncoding.DecodeString(jcu.ID)
	if err != nil {
		w.WriteHeader(invalidBase64)
		return
	}

	cu := ConfigUpdate{ID: id64}
	if msg, err := jid.service.ConfigUpdate(nil, &cu); err == nil {
		cur := msg.(*ConfigUpdateReply)
		rep, _ := json.Marshal(createJsonConfig(cur.Config))
		w.Write(rep)
	} else {
		w.WriteHeader(cothorityFailure)
	}
}

func (jid *jsonID) ps(w http.ResponseWriter, r *http.Request) {
	req, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(corruptRequest)
		return
	}

	jps := jsonProposeSend{}
	if err = json.Unmarshal(req, &jps); err != nil {
		w.WriteHeader(invalidJson)
		return
	}

	id64, err := base64.StdEncoding.DecodeString(jps.ID)
	if err != nil {
		w.WriteHeader(invalidBase64)
		return
	}

	device := createDeviceMap(jps.Propose.Device)
	conf := Config{
		Threshold: jps.Propose.Threshold,
		Device:    device,
		Data:      jps.Propose.Data}
	ps := ProposeSend{ID: id64, Propose: &conf}

	if _, err = jid.service.ProposeSend(nil, &ps); err == nil {
		rep, _ := json.Marshal(createJsonConfig(&conf))
		w.Write(rep)
	} else {
		w.WriteHeader(cothorityFailure)
	}
}

func (jid *jsonID) pu(w http.ResponseWriter, r *http.Request) {
	req, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(corruptRequest)
		return
	}

	jpu := jsonProposeUpdate{}
	if err = json.Unmarshal(req, &jpu); err != nil {
		w.WriteHeader(invalidJson)
		return
	}

	id64, _ := base64.StdEncoding.DecodeString(jpu.ID)
	pu := ProposeUpdate{ID: id64}

	if msg, err := jid.service.ProposeUpdate(nil, &pu); err == nil {
		pur := msg.(*ProposeUpdateReply)
		if pur.Propose == nil {
			w.WriteHeader(noProposed)
			return
		}

		rep, _ := json.Marshal(createJsonConfig(pur.Propose))
		w.Write(rep)
	} else {
		w.WriteHeader(cothorityFailure)
	}
}

func (jid *jsonID) pv(w http.ResponseWriter, r *http.Request) {
	req, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(corruptRequest)
		return
	}

	jpv := jsonProposeVote{}
	json.Unmarshal(req, &jpv)

	id64, err := base64.StdEncoding.DecodeString(jpv.ID)
	if err != nil {
		w.WriteHeader(invalidBase64)
		return
	}

	sig, err := base64.StdEncoding.DecodeString(jpv.Signature)
	if err != nil {
		w.WriteHeader(invalidBase64)
		return
	}

	pv := ProposeVote{ID: id64, Signer: jpv.Signer,
		Signature: sig}

	if msg, err := jid.service.ProposeVote(nil, &pv); err == nil {
		_ = msg.(*ProposeVoteReply)
		w.Write([]byte("success"))
	} else {
		w.WriteHeader(cothorityFailure)
	}
}
