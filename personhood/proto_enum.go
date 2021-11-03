package personhood

// EmailSignupEnum describes the action taken
type EmailSignupEnum int32

const (
	// ESECreated indicates A new user has been created and instructions sent to
	// the email given
	ESECreated EmailSignupEnum = iota
	// ESEExists indicates that the given email already exists and that a
	// recovery is needed.
	ESEExists
	// ESETooManyRequests indicates that the system is blocking new requests for
	// this day.
	ESETooManyRequests
)

// EmailRecoverEnum describes the action taken
type EmailRecoverEnum int32

const (
	// ERERecovered indicates that a recovery has been initiated and the
	// information has been sent to the email address
	ERERecovered EmailRecoverEnum = iota
	// EREUnknown indicates that this email is not known
	EREUnknown
	// ERETooManyRequests indicates that the system is blocking new requests for
	// this day.
	ERETooManyRequests
)
