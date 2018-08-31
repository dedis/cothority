package service

import (
	ol "github.com/dedis/cothority/omniledger/service"
	"github.com/dedis/cothority/skipchain"
)

// PROTOSTART
// type :InstanceID:bytes
// type :SkipBlockID:bytes
// type :skipchain.SkipBlockID:bytes
// type :ol.InstanceID:bytes
// package personhood;
//
// option java_package = "ch.epfl.dedis.proto";
// option java_outer_classname = "Personhood";

// LinkPop stores a link to a pop-party to accept this configuration. It will
// try to create an account to receive payments from clients.
type LinkPop struct {
	OmniLedgerID skipchain.SkipBlockID
	PopInstance  ol.InstanceID
}

// StringReply can be used by all calls that need a string to be returned
// to the caller.
type StringReply struct {
	Reply string
}

type GetAccount struct {
	PopInstance ol.InstanceID
}

type GetAccountReply struct {
	Account ol.InstanceID
}

//
// * Questionnaires
//

type Questionnaire struct {
	Title     string
	Questions []string
	// Replies indicates how many answers the player can chose.
	Replies int
	Balance uint64
	Reward  uint64
	ID      []byte
}

// RegisterQuestionnaire creates a questionnaire with a number of questions to
// chose from and how much each replier gets rewarded.
type RegisterQuestionnaire struct {
	Questionnaire Questionnaire
}

type ListQuestionnaires struct {
	Start  int
	Number int
}

type ListQuestionnairesReply struct {
	Questionnaires []Questionnaire
}

type AnswerQuestionnaire struct {
	ID      []byte
	Replies []int
	Account ol.InstanceID
}

type TopupQuestionnaire struct {
	ID    []byte
	Topup uint64
}

//
// * Popper
//

type Message struct {
	Subject string
	Date    uint64
	Text    string
	Author  ol.InstanceID
	Balance uint64
	Reward  uint64
	ID      []byte
}

type SendMessage struct {
	Message Message
}

type ListMessages struct {
	Start  int
	Number int
}

type ListMessagesReply struct {
	Subjects []string
}

type ReadMessage struct {
	Subject string
	Reader  ol.InstanceID
}

type ReadMessageReply struct {
	Message Message
}

type TopupMessage struct {
	ID     []byte
	Amount uint64
}
