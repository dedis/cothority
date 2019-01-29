package personhood

import (
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/darc"
	pop "go.dedis.ch/cothority/v3/pop/service"
	"go.dedis.ch/cothority/v3/skipchain"
)

// PROTOSTART
// type :InstanceID:bytes
// type :SkipBlockID:bytes
// type :skipchain.SkipBlockID:bytes
// type :byzcoin.InstanceID:bytes
// package personhood;
//
// import "darc.proto";
// import "pop.proto";
//
// option java_package = "ch.epfl.dedis.lib.proto";
// option java_outer_classname = "Personhood";

// LinkPoP stores a link to a pop-party to accept this configuration. It will
// try to create an account to receive payments from clients.
type LinkPoP struct {
	Party Party
}

// Party represents everything necessary to find a party in the ledger.
type Party struct {
	// ByzCoinID represents the ledger where the pop-party is stored.
	ByzCoinID skipchain.SkipBlockID
	// InstanceID is where to find the party in the ledger.
	InstanceID byzcoin.InstanceID
	// FinalStatement describes the party and the signature of the organizers.
	FinalStatement pop.FinalStatement
	// Darc being responsible for the PartyInstance.
	Darc darc.Darc
	// Signer can call Invoke on the PartyInstance.
	Signer darc.Signer
}

// StringReply can be used by all calls that need a string to be returned
// to the caller.
type StringReply struct {
	Reply string
}

//
// * Questionnaires
//

// Questionnaire represents one poll that will be sent to candidates.
type Questionnaire struct {
	// Title of the poll
	Title string
	// Questions is a slice of texts that will be presented
	Questions []string
	// Replies indicates how many answers the player can chose.
	Replies int
	// Balance left for that questionnaire
	Balance uint64
	// Reward for replying to one questionnaire
	Reward uint64
	// ID is a random identifier of that questionnaire
	ID []byte
}

// Reply holds the results of the questionnaire together with a slice of users
// that participated in it.
type Reply struct {
	// Sum is the sum of all replies for a given index of the questions.
	Sum []int
	// TODO: replace this with a linkable ring signature
	Users []byzcoin.InstanceID
}

// RegisterQuestionnaire creates a questionnaire with a number of questions to
// chose from and how much each replier gets rewarded.
type RegisterQuestionnaire struct {
	// Questionnaire is the questionnaire to be stored.
	Questionnaire Questionnaire
}

// ListQuestionnaires requests all questionnaires from Start, but not more than
// Number.
type ListQuestionnaires struct {
	// Start of the answer.
	Start int
	// Number is the maximum of questionnaires that will be returned.
	Number int
}

// ListQuestionnairesReply is a slice of all questionnaires, starting with the
// one having the highest balance left.
type ListQuestionnairesReply struct {
	// Questionnaires is a slice of questionnaires, with the highest balance first.
	Questionnaires []Questionnaire
}

// AnswerQuestionnaire sends the answer from one client.
type AnswerQuestionnaire struct {
	// QuestID is the ID of the questionnaire to be replied.
	QuestID []byte
	// Replies is a slice of answers, up to Questionnaire.Replies
	Replies []int
	// Account where to put the reward to.
	Account byzcoin.InstanceID
}

// TopupQuestionnaire can be used to add new balance to a questionnaire.
type TopupQuestionnaire struct {
	// QuestID indicates which questionnaire
	QuestID []byte
	// Topup is the amount of coins to put there.
	Topup uint64
}

//
// * Popper
//

// Message represents a message that will be sent to the system.
type Message struct {
	// Subject is one of the fields always visible, even if the client did not
	// chose to read the message.
	Subject string
	// Date, as unix-encoded seconds since 1970.
	Date uint64
	// Text, can be any length of text of the message.
	Text string
	// Author's coin account for eventual rewards/tips to the author.
	Author byzcoin.InstanceID
	// Balance the message has currently left.
	Balance uint64
	// Reward for reading this messgae.
	Reward uint64
	// ID of the messgae - should be random.
	ID []byte
	// PartyIID - the instance ID of the party this message belongs to
	PartyIID byzcoin.InstanceID
}

// SendMessage stores the message in the system.
type SendMessage struct {
	// Message to store.
	Message Message
}

// ListMessages sorts all messages by balance and sends back the messages from
// Start, but not more than Number.
type ListMessages struct {
	// Start of the messages returned
	Start int
	// Number of maximum messages returned
	Number int
	// ReaderID of the reading account, to skip messages created by this reader
	ReaderID byzcoin.InstanceID
}

// ListMessagesReply returns the subjects, IDs, balances and rewards of the top
// messages, as chosen in ListMessages.
type ListMessagesReply struct {
	// Subjects of the messages
	Subjects []string
	// MsgIDs of the messages
	MsgIDs [][]byte
	// Balances
	Balances []uint64
	// Rewards
	Rewards []uint64
	// PartyIIDs
	PartyIIDs []byzcoin.InstanceID
}

// ReadMessage requests the full message and the reward for that message.
type ReadMessage struct {
	// MsgID to request.
	MsgID []byte
	// PartyIID to calculate the party coin account
	PartyIID []byte
	// Reader that will receive the reward
	Reader byzcoin.InstanceID
}

// ReadMessageReply if the message is still active (balance >= reward)
type ReadMessageReply struct {
	// Messsage to read.
	Message Message
	// Rewarded is true if this is the first time the message has been read
	// by this reader.
	Rewarded bool
}

// TopupMessage to fill up the balance of a message
type TopupMessage struct {
	// MsgID of the message to top up
	MsgID []byte
	// Amount to coins to put in the message
	Amount uint64
}
