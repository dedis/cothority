# Ideas for a reduced PoP-session at 33C3
## Apps needed
All in all 2 CLI-apps, 1 bash-script and 1 android-apps are needed:
- APP1 for the organisers
- APP2 for the attendees
- Bash-script for the attendees and pre-printing of the private/public keys
The conode interacts with APP1 through a pop-service.

### Organizers: creating a session
APP1
* Created manually:
	* group.toml - identical for all organizers
	* pop_config.toml - identical for all organizers
* link to a conode by entering a 6-digit number in the APP1 printed on the conode
* collectively sign pop_config.toml with the conode

### Attendees: joining a session 
Bash-script that outputs:
* Ed25519 public/private key in hexadecimal
-> prepare 50 paper-tokens that people can use, make a list of all public-keys for easy copy/paste

### Organisers: during party
APP1
* enter public key that is stored locally

### Organisers: finalisation
APP1:
* one organiser creates the final statement by asking all conodes to collectively sign the list of public keys and the configuration file

### Attendees: finalisation
APP2, that
* takes the final_statement.toml and the private key

### Attendees: usage
APP2 from attendees:
* inputs context and message to be signed
* outputs tag and signature in base64
APP1 from organizers:
* inputs context, message, tag and signature
* verifies the signature and prints OK/Error

## APP1 commands
This lists the commands available for the popmgr-app (PoP-manager):

### popmgr link address
This command will contact the conode at address. Two inputs on the keyboard are required:
verification of public key: the public key is requested from the address and displayed, then a “Y/n”-prompt appears
PIN-entry: the conode will display a PIN on the server-side, that has to be entered here for authentification

### popmgr config pop_desc.toml group.toml
Stores the pop_desc-configuration and the group locally and sends it to the 
conode.

### popmgr public key64
Stores in a list the base-64 encoded public key key64 for sending with the ‘popmgr final’-command.

### popmgr final
finalizes the PoParty:
* sends all public keys to the conode
* waits for other organizers to do the same
* retrieves the collective signature
* prints out the final_statement.toml, containing:
	* pop_desc.toml
	* group.toml
	* public keys
	* collective signature

## APP2 commands

### popclient join final_statement.toml private
* Saves final_statement.toml and private (base64 encoded private Ed25519 key) locally for later usage.
* Keyboard-input: prints the aggregate public key and asks for confirmation
* Verifies the collective signature
* Verifies that the private key corresponds to one of the public keys

### popclient sign message context
* signs the message+context (both strings) and outputs the signature and the tag base64 encoded

### popclient verify message context tag signature
* message and context are strings
* tag and signature are base64-encoded
* verifies the signature against the message and the context
* outputs OK/failure

## Service-API
### Define the API

## Later
Here I copied everything that won’t go into the 33c3 version
Attendees: joining a session 
if they don’t trust us, create their own
add QrCode for scanning with AndroidApp
Organisers: during party
Android-app, which needs to:
connect to the conode (must be done through the service)
APP1 offers a QrCode that is scanned by the Android-app
read attendees public keys and store them in the Android-app
send list of public keys to organizer’s conode
Attendees: finalisation
prints the PoP-token in QrCode
Attendees: usage
Android-app from organisers:
scans the tag and signature from the attendees screen
verifies the signature and checks the tag is not served yet
APP1 commands
popmgr config read
Prints out the complete configuration created with ‘popmgr config set’, so that it can be stored in pop_config.toml and given to the attendees.
/*
Discconects from the conode, terminates the current session
*/
- func UnLinkConode() //Disconnect from conode
/*
Send attendees public keys
*/
- func SendAttendeesKeys(publicKeys []abstract.Point) (error) //Send public keys of all attendees
/*
Displays the QR code to connect to a conode server through the android app
inserts a 6 digit number and the address of a conode
*/
- func DisplayQrCode(int PIN, string address) (error)//Display the QR code of a conode

Use token
/*
Asks to use the token
Receives the hash of final statement, to verify that the user was at the party, outputs message and context to sign
*/
- func UseToken(finalStatement []byte) ([]byte []byte) //Outputs message and context to sign
Android-app
what menus, what steps?


Comments

Nicolas:
Would it be possible to do it only on paper ? Like really have simple public keys (RSA modulo 10 bits :D ), a simple “hash”, a simple “signature scheme” ? with the help of a python script on our side.  
I’m just confused about how the whole thing is gonna look like because for some parts, it says we don’t need the APP, some others we do so it seems that we do need an APP for the whole thing ? And at the same time, will people accept to install an APP we tell them to use?
Distributed trust
It can be put across multiple laptops
You don’t have to trust one person
