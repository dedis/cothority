/*
The external file handles requests from an android (or other source)
by other means than passing through the `cothority/network`-library.
The method chose here is:
__FILL IN__
*/
package identity

import (
    "net/http"
    "encoding/json"
    "io/ioutil"

    "github.com/dedis/cothority/sda"
    "github.com/dedis/cothority/log"
)

var service *Service

func getRequest(r *http.Request) string {
    req, _ := ioutil.ReadAll(r.Body)
    return string(req)
}

func ai(w http.ResponseWriter, r *http.Request) {
    req := getRequest(r)

    // TODO ???
    roster := sda.Roster{}

    if air, err := ExCreateIdentity(req, &roster); err != nil {
        log.Error(err)
    } else {
        log.Print("Successfully added new Idenity to Service")
        _ = air // !!!
    }
}

func cu(w http.ResponseWriter, r *http.Request) {
    msg := getRequest(r)

    if cu, err := ExConfigUpdate(msg); err != nil {
        log.Error(err)
    } else {
        log.Print("Successfully retrieved Config")
        _ = cu // !!!
    }
}

func ps(w http.ResponseWriter, r *http.Request) {
    msg := getRequest(r)

    if err := ExProposeSend(msg); err != nil {
        log.Error(err)
    }
    log.Print("Successfully sent proposal")
}

func pf(w http.ResponseWriter, r *http.Request) {
    msg := getRequest(r)

    if pf, err := ExProposeFetch(msg); err != nil {
        log.Error(err)
    } else {
        log.Print("Successfully fetched proposal")
        _ = pf // !!!
    }
}

func pv(w http.ResponseWriter, r *http.Request) {
    msg := getRequest(r)

    if err := ExProposeVote(msg); err != nil {
        log.Error(err)
    }
    log.Print("Successfully voted on proposal")
}

func ExCreateIdentity(request string, roster *sda.Roster) (*AddIdentityReply, error) {
    ai := AddIdentity{}
    //if err := json.Unmarshal([]byte(request), &ai); err != nil {
        //return nil, err
    //}

    json.Unmarshal([]byte(request), &ai)

    ai.Roster = roster
    msg, err := service.AddIdentity(nil, &ai)
    return msg.(*AddIdentityReply), err
}

func ExConfigUpdate(request string) (*ConfigUpdate, error) {
    cu := ConfigUpdate{}
    if err := json.Unmarshal([]byte(request), &cu); err != nil {
        return nil, err
    }

    msg, err := service.ConfigUpdate(nil, &cu)
    return msg.(*ConfigUpdate), err
}

func ExProposeSend(request string) error {
    ps := ProposeSend{}
    //if err := json.Unmarshal([]byte(request), &ps); err != nil {
        //return err
    //}

    json.Unmarshal([]byte(request), &ps)

    _, err := service.ProposeSend(nil, &ps)
    return err
}

func ExProposeFetch(request string) (*ProposeFetch, error) {
    pf := ProposeFetch{}
    //if err := json.Unmarshal([]byte(request), &pf); err != nil {
        //return nil, err
    //}
    json.Unmarshal([]byte(request), &pf)
    msg, err := service.ProposeFetch(nil, &pf)
    return msg.(*ProposeFetch), err
}

func ExProposeVote(request string) error {
   pv := ProposeVote{}
   if err := json.Unmarshal([]byte(request), &pv); err != nil {
       return err
   }

   _, err := service.ProposeVote(nil, &pv)
   return err
}

// Init HTTP server (multiplex)
func (s *Service) initExternal() {
    service = s
    //finish := make(chan bool)
    //server := http.NewServeMux()
    //server.HandleFunc("/ai", ai)

    //log.Info("HTTP-server listening on 192.168.192.17:2000")
    //go func() {
        //http.ListenAndServe("192.168.192.17:2000", server)
    //}()
    //<-finish
}

