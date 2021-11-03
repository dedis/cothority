package personhood

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestEmailConfig_tooManyEmails(t *testing.T) {
	ec := emailConfig{
		Emails:      10,
		EmailsLimit: 10,
		EmailsLast:  0,
	}
	require.True(t, ec.tooManyEmails(0))
	require.False(t, ec.tooManyEmails(1))
	require.True(t, ec.tooManyEmails(1))
	require.False(t, ec.tooManyEmails(3))
	require.False(t, ec.tooManyEmails(3))
	require.True(t, ec.tooManyEmails(3))
}
