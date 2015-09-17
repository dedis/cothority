package sign

import "errors"

var ErrUnknownMessageType error = errors.New("received message of unknown type")

var ErrViewRejected error = errors.New("view Rejected: not all nodes accepted view")

var ErrImposedFailure error = errors.New("failure imposed")

var ErrPastRound error = errors.New("round number already passed")
