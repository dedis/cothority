package prifi

import (
	prifi_lib "github.com/dedis/cothority/lib/prifi"
	"github.com/dedis/cothority/lib/sda"
)

//wrappers

type Struct_ALL_ALL_PARAMETERS struct {
	*sda.TreeNode
	prifi_lib.ALL_ALL_PARAMETERS
}

type Struct_CLI_REL_TELL_PK_AND_EPH_PK struct {
	*sda.TreeNode
	prifi_lib.CLI_REL_TELL_PK_AND_EPH_PK
}

type Struct_CLI_REL_UPSTREAM_DATA struct {
	*sda.TreeNode
	prifi_lib.CLI_REL_UPSTREAM_DATA
}

type Struct_REL_CLI_DOWNSTREAM_DATA struct {
	*sda.TreeNode
	prifi_lib.REL_CLI_DOWNSTREAM_DATA
}

type Struct_REL_CLI_TELL_EPH_PKS_AND_TRUSTEES_SIG struct {
	*sda.TreeNode
	prifi_lib.REL_CLI_TELL_EPH_PKS_AND_TRUSTEES_SIG
}

type Struct_REL_CLI_TELL_TRUSTEES_PK struct {
	*sda.TreeNode
	prifi_lib.REL_CLI_TELL_TRUSTEES_PK
}

type Struct_REL_TRU_TELL_CLIENTS_PKS_AND_EPH_PKS_AND_BASE struct {
	*sda.TreeNode
	prifi_lib.REL_TRU_TELL_CLIENTS_PKS_AND_EPH_PKS_AND_BASE
}

type Struct_REL_TRU_TELL_TRANSCRIPT struct {
	*sda.TreeNode
	prifi_lib.REL_TRU_TELL_TRANSCRIPT
}

type Struct_TRU_REL_DC_CIPHER struct {
	*sda.TreeNode
	prifi_lib.TRU_REL_DC_CIPHER
}

type Struct_TRU_REL_SHUFFLE_SIG struct {
	*sda.TreeNode
	prifi_lib.TRU_REL_SHUFFLE_SIG
}

type Struct_TRU_REL_TELL_NEW_BASE_AND_EPH_PKS struct {
	*sda.TreeNode
	prifi_lib.TRU_REL_TELL_NEW_BASE_AND_EPH_PKS
}

type Struct_TRU_REL_TELL_PK struct {
	*sda.TreeNode
	prifi_lib.TRU_REL_TELL_PK
}
