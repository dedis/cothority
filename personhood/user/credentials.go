package user

// This file gives a definition of all available credentials in a User.

// CredentialEntry represents one entry in the credentials list
type CredentialEntry string

const (
	// Public credentials for this user
	Public CredentialEntry = "1-public"
	// Config of this user
	Config = "1-config"
	// Devices for this user - name:DarcID
	Devices = "1-devices"
	// Recoveries for this user - name:DarcID
	Recoveries = "1-recovery"
	// Calypso entries - anme:InstanceID
	Calypso = "1-calypso"
)

// AttributePublic are all the attributes available in the Public credential
type AttributePublic string

const (
	// Contacts is a concatenated slice of CredentialIDs of known contacts
	Contacts AttributePublic = "contactsBuf"
	// Alias of the user
	Alias = "alias"
	// Email of the user
	Email = "email"
	// CoinID of the user
	CoinID = "coin"
	// SeedPub - Deprecated - seed used to create the user
	SeedPub = "seedPub"
	// Phone of the user
	Phone = "phone"
	// Actions in name:CoinID of the user
	Actions = "actions"
	// Groups the user has stored - name:DarcID
	Groups = "groups"
	// URL for the users website
	URL = "url"
	// Challenge - Deprecated - challenge for Personhood
	Challenge = "challenge"
	// Personhood - Deprecated - Personhood-key
	Personhood = "personhood"
	// Subscribe - Deprecated - subscription to mailing-list
	Subscribe = "subscribe"
	// Snacked - Deprecated - for OpenHouse 2019
	Snacked = "snacked"
	// Version of the Public entries
	Version = "version"
)

// AttributeConfig represents the configuration of the user
type AttributeConfig string

const (
	// View for the login.c4dt.org
	View AttributeConfig = "view"
	// Spawner used by this user
	Spawner = "spawner"
	// StructVersion - increased by 1 for every update
	StructVersion = "structVersion"
	// LtsID used by this user
	LtsID = "ltsID"
	// LtsX of the LtsID
	LtsX = "ltsX"
)
