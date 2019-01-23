export var DecodeType = {

    /**
     * @file File containing the different message types to decode for the Cothority.
     */

    /**
     * Server response types.
     */
    STATUS_RESPONSE: "Response",
    RANDOM_RESPONSE: "RandomResponse",
    SIGNATURE_RESPONSE: "SignatureResponse",
    CLOCK_RESPONSE: "ClockResponse",
    COUNT_RESPONSE: "CountResponse",

    /**
     * Skip{block, chain} response types.
     */
    GET_BLOCK_REPLY: "GetBlockReply",
    LATEST_BLOCK_RESPONSE: "LatestBlockResponse",
    STORE_SKIP_BLOCK_RESPONSE: "StoreSkipBlockResponse",
    GET_UPDATE_CHAIN_REPLY: "GetUpdateChainReply",
    GET_ALL_SKIPCHAINS_REPLY: "GetAllSkipchainsReply",

    /**
     * PoP response types.
     */
    STORE_CONFIG_REPLY: "StoreConfigReply",
    FINALIZE_RESPONSE: "FinalizeResponse",
    GET_PROPOSALS_REPLY: "GetProposalsReply",
    FETCH_RESPONSE: "FinalizeResponse",
    CHECK_CONFIG_REPLY: "CheckConfigReply",
    VERIFY_LINK_REPLY: 'VerifyLinkReply',

    /**
     * CISC response types.
     */
    DATA_UPDATE_REPLY: "DataUpdateReply",
    CONFIG_UPDATE_REPLY: "ConfigUpdateReply",
    PROPOSE_UPDATE_REPLY: "ProposeUpdateReply",
    PROPOSE_VOTE_REPLY: "ProposeVoteReply",

    /**
     * Personhood response types.
     */
    STRING_REPLY: "StringReply",
    LISTMESSAGES_REPLY: "ListMessagesReply",
    READMESSAGE_REPLY: "ReadMessageReply",

    /**
     * MISC response types.
     */
    // This points to an empty message type as cothority doesn't provide one by default
    EMPTY_REPLY: "status.Request",
};