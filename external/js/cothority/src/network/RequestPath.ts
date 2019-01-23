export var RequestPath = {

    /**
     * @file File containing different available paths for the Cothority.
     */

    /**
     * Services
     */
    STATUS:"Status",
    IDENTITY:"Identity",
    SKIPCHAIN:"Skipchain",
    POP:"PoPServer",
    CISC:"Cisc",
    BYZCOIN:"ByzCoin",
    PERSONHOOD:"Personhood",

    /**
     * ByzCoin Requests
     */
    BYZCOIN_CREATE_GENESIS: "CreateGenesisBlock",
    BYZCOIN_CREATE_GENESIS_RESPONSE: "CreateGenesisBlockResponse",

    /**
     * Status Requests
     */
    STATUS_REQUEST:"status.Request",

    /**
     * Identity Requests
     */
    IDENTITY_PIN_REQUEST:"PinRequest",
    IDENTITY_DATA_UPDATE:"DataUpdate",
    IDENTITY_PROPOSE_UPDATE:"ProposeUpdate",
    IDENTITY_PROPOSE_SEND:"ProposeSend",
    IDENTITY_PROPOSE_VOTE:"ProposeVote",

    /**
     * Skipchain Requests
     */
    SKIPCHAIN_GET_UPDATE_CHAIN:"GetUpdateChain",
    SKIPCHAIN_GET_ALL_SKIPCHAINS:"GetAllSkipchains",

    /**
     * PoP Requests
     */
    POP_STORE_CONFIG:"pop.StoreConfig",
    POP_FINALIZE_REQUEST:"pop.FinalizeRequest",
    POP_FETCH_REQUEST:"pop.FetchRequest",
    POP_PIN_REQUEST:"pop.PinRequest",
    POP_GET_PROPOSALS:"pop.GetProposals",
    POP_CHECK_CONFIG:"pop.CheckConfig",
    POP_VERIFY_LINK:"pop.VerifyLink",
    POP_GET_INSTANCE_ID:"pop.GetInstanceID",
    POP_GET_INSTANCE_ID_REPLY:"pop.GetInstanceIDReply",
    POP_GET_FINAL_STATEMENTS:"pop.GetFinalStatements",
    POP_GET_FINAL_STATEMENTS_REPLY:"pop.GetFinalStatementsReply",
    POP_STORE_KEYS:"pop.StoreKeys",
    POP_STORE_KEYS_REPLY:"pop.StoreKeysReply",
    POP_GET_KEYS:"pop.GetKeys",
    POP_GET_KEYS_REPLY:"pop.GetKeysReply",


    /**
     * CISC Requests
     */
    CISC_CONFIG:"Config",
    CISC_CONFIG_UPDATE:"ConfigUpdate",
    CISC_DEVICE:"Device",
    CISC_PROPOSE_VOTE:"ProposeVote",
    CISC_PROPOSE_SEND:"ProposeSend",
    CISC_PROPOSE_UPDATE:"ProposeUpdate",
    CISC_SCHNORR_SIG:"SchnorrSig",

    /**
     * Personhood Requests
     */
    PERSONHOOD_SENDMESSAGE:"SendMessage",
    PERSONHOOD_LISTMESSAGES:"ListMessages",
    PERSONHOOD_READMESSAGE:"ReadMessage",
    PERSONHOOD_TESTSTORE:"TestStore",
};