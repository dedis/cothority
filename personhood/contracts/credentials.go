package contracts

// This file gives a definition of all available credentials in a User.

// CredentialEntry represents one entry in the credentials list
type CredentialEntry string

const (
	// CEPublic credentials for this user
	CEPublic CredentialEntry = "1-public"
	// CEConfig of this user
	CEConfig CredentialEntry = "1-config"
	// CEDevices for this user - name:DarcID
	CEDevices CredentialEntry = "1-devices"
	// CERecoveries for this user - name:DarcID
	CERecoveries CredentialEntry = "1-recovery"
	// CECalypso entries - anme:InstanceID
	CECalypso CredentialEntry = "1-calypso"
)

// AttributeString is the basic type for AttributePublic and AttributeConfig
type AttributeString string

// AttributePublic are all the attributes available in the Public credential
type AttributePublic AttributeString

const (
	// APContacts is a concatenated slice of CredentialIDs of known contacts
	APContacts AttributePublic = "contactsBuf"
	// APAlias of the user
	APAlias AttributePublic = "alias"
	// APEmail of the user
	APEmail AttributePublic = "email"
	// APCoinID of the user
	APCoinID AttributePublic = "coin"
	// APSeedPub - Deprecated - seed used to create the user
	APSeedPub AttributePublic = "seedPub"
	// APPhone of the user
	APPhone AttributePublic = "phone"
	// APActions in name:CoinID of the user
	APActions AttributePublic = "actions"
	// APGroups the user has stored - name:DarcID
	APGroups AttributePublic = "groups"
	// APURL for the users website
	APURL AttributePublic = "url"
	// APChallenge - Deprecated - challenge for Personhood
	APChallenge AttributePublic = "challenge"
	// APPersonhood - Deprecated - Personhood-key
	APPersonhood AttributePublic = "personhood"
	// APSubscribe - Deprecated - subscription to mailing-list
	APSubscribe AttributePublic = "subscribe"
	// APSnacked - Deprecated - for OpenHouse 2019
	APSnacked AttributePublic = "snacked"
	// APVersion of the Public entries
	APVersion AttributePublic = "version"
)

// AttributeConfig represents the configuration of the user
type AttributeConfig AttributeString

const (
	// ACView for the login.c4dt.org
	ACView AttributeConfig = "view"
	// ACSpawner used by this user
	ACSpawner AttributeConfig = "spawner"
	// ACStructVersion - increased by 1 for every update
	ACStructVersion AttributeConfig = "structVersion"
	// ACLtsID used by this user
	ACLtsID AttributeConfig = "ltsID"
	// ACLtsX of the LtsID
	ACLtsX AttributeConfig = "ltsX"
)
