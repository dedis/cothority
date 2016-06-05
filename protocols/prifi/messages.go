package prifi

import (
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/abstract"
)

/*
 * Messages used by PriFi.
 * Syntax : SOURCE_DEST_CONTENT_CONTENT
 */

//CLI_REL_TELL_PK_AND_EPH_PK
//CLI_REL_UPSTREAM_DATA
//REL_CLI_DOWNSTREAM_DATA
//REL_CLI_TELL_EPH_PKS_AND_TRUSTEES_SIG
//REL_CLI_TELL_TRUSTEES_PK
//REL_TRU_TELL_CLIENTS_PKS_AND_EPH_PKS_AND_BASE
//REL_TRU_TELL_TRANSCRIPT
//TRU_REL_DC_CIPHER
//TRU_REL_SHUFFLE_SIG
//TRU_REL_TELL_NEW_BASE_AND_EPH_PKS
//TRU_REL_TELL_PK

//not used yet :
//REL_CLI_DOWNSTREAM_DATA
//CLI_REL_DOWNSTREAM_NACK

type CLI_REL_TELL_PK_AND_EPH_PK struct {
	Pk    abstract.Point
	EphPk abstract.Point
}

type CLI_REL_UPSTREAM_DATA struct {
	RoundId int32
	Data    []byte
}

type REL_CLI_DOWNSTREAM_DATA struct {
	RoundId    int32
	Data       []byte
	FlagResync bool
}

type REL_CLI_TELL_EPH_PKS_AND_TRUSTEES_SIG struct {
	Base         abstract.Point
	EphPks       []abstract.Point
	TrusteesSigs [][]byte
}

type REL_CLI_TELL_TRUSTEES_PK struct {
	Pks []abstract.Point
}

type REL_TRU_TELL_CLIENTS_PKS_AND_EPH_PKS_AND_BASE struct {
	Pks    []abstract.Point
	EphPks []abstract.Point
	Base   abstract.Point
}

type REL_TRU_TELL_TRANSCRIPT struct {
	G_s    []abstract.Point
	EphPks [][]abstract.Point
	Proofs [][]byte
}

type TRU_REL_DC_CIPHER struct {
	RoundId   int32
	TrusteeId int
	Data      []byte
}

type TRU_REL_SHUFFLE_SIG struct {
	TrusteeId int
	Sig       []byte
}

type TRU_REL_TELL_NEW_BASE_AND_EPH_PKS struct {
	NewBase   abstract.Point
	NewEphPks []abstract.Point
	Proof     []byte
}

type TRU_REL_TELL_PK struct {
	Pk abstract.Point
}

//wrappers

type Struct_CLI_REL_TELL_PK_AND_EPH_PK struct {
	*sda.TreeNode
	CLI_REL_TELL_PK_AND_EPH_PK
}

type Struct_CLI_REL_UPSTREAM_DATA struct {
	*sda.TreeNode
	CLI_REL_UPSTREAM_DATA
}

type Struct_REL_CLI_DOWNSTREAM_DATA struct {
	*sda.TreeNode
	REL_CLI_DOWNSTREAM_DATA
}

type Struct_REL_CLI_TELL_EPH_PKS_AND_TRUSTEES_SIG struct {
	*sda.TreeNode
	REL_CLI_TELL_EPH_PKS_AND_TRUSTEES_SIG
}

type Struct_REL_CLI_TELL_TRUSTEES_PK struct {
	*sda.TreeNode
	REL_CLI_TELL_TRUSTEES_PK
}

type Struct_REL_TRU_TELL_CLIENTS_PKS_AND_EPH_PKS_AND_BASE struct {
	*sda.TreeNode
	REL_TRU_TELL_CLIENTS_PKS_AND_EPH_PKS_AND_BASE
}

type Struct_REL_TRU_TELL_TRANSCRIPT struct {
	*sda.TreeNode
	REL_TRU_TELL_TRANSCRIPT
}

type Struct_TRU_REL_DC_CIPHER struct {
	*sda.TreeNode
	TRU_REL_DC_CIPHER
}

type Struct_TRU_REL_SHUFFLE_SIG struct {
	*sda.TreeNode
	TRU_REL_SHUFFLE_SIG
}

type Struct_TRU_REL_TELL_NEW_BASE_AND_EPH_PKS struct {
	*sda.TreeNode
	TRU_REL_TELL_NEW_BASE_AND_EPH_PKS
}

type Struct_TRU_REL_TELL_PK struct {
	*sda.TreeNode
	TRU_REL_TELL_PK
}
