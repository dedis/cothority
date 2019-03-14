import { Message, Properties } from "protobufjs/light";
import { EMPTY_BUFFER, registerMessage } from "../protobuf";
import { ForwardLink, SkipBlock } from "./skipblock";

export class GetAllSkipChainIDs extends Message<GetAllSkipChainIDs> {
    /**
     * @see README#Message classes
     */
    static register() {
        registerMessage("GetAllSkipChainIDs", GetAllSkipChainIDs);
    }
}

export class GetAllSkipChainIDsReply extends Message<GetAllSkipChainIDsReply> {
    /**
     * @see README#Message classes
     */
    static register() {
        registerMessage("GetAllSkipChainIDsReply", GetAllSkipChainIDsReply);
    }

    readonly skipChainIDs: Buffer[];

    constructor(props?: Properties<GetAllSkipChainIDsReply>) {
        super(props);

        this.skipChainIDs = this.skipChainIDs || [];
    }
}

export class StoreSkipBlock extends Message<StoreSkipBlock> {
    /**
     * @see README#Message classes
     */
    static register() {
        registerMessage("StoreSkipBlock", StoreSkipBlock, SkipBlock);
    }

    readonly targetSkipChainID: Buffer;
    readonly newBlock: SkipBlock;
    readonly signature: Buffer;

    constructor(properties: Properties<StoreSkipBlock>) {
        super(properties);

        this.targetSkipChainID = Buffer.from(this.targetSkipChainID || EMPTY_BUFFER);
        this.signature = Buffer.from(this.signature || EMPTY_BUFFER);
    }
}

export class StoreSkipBlockReply extends Message<StoreSkipBlock> {
    /**
     * @see README#Message classes
     */
    static register() {
        registerMessage("StoreSkipBlockReply", StoreSkipBlockReply, SkipBlock);
    }

    readonly latest: SkipBlock;
    readonly previous: SkipBlock;
}

export class GetSingleBlock extends Message<GetSingleBlock> {
    /**
     * @see README#Message classes
     */
    static register() {
        registerMessage("GetSingleBlock", GetSingleBlock);
    }

    readonly id: Buffer;

    constructor(props?: Properties<GetSingleBlock>) {
        super(props);

        this.id = Buffer.from(this.id || EMPTY_BUFFER);
    }
}

export class GetSingleBlockByIndex extends Message<GetSingleBlockByIndex> {
    /**
     * @see README#Message classes
     */
    static register() {
        registerMessage("GetSingleBlockByIndex", GetSingleBlockByIndex);
    }

    readonly genesis: Buffer;
    readonly index: number;

    constructor(props?: Properties<GetSingleBlockByIndex>) {
        super(props);

        this.genesis = Buffer.from(this.genesis || EMPTY_BUFFER);
    }
}

export class GetSingleBlockByIndexReply extends Message<GetSingleBlockByIndexReply> {
    /**
     * @see README#Message classes
     */
    static register() {
        registerMessage("GetSingleBlockByIndexReply", GetSingleBlockByIndexReply);
    }

    readonly skipblock: SkipBlock;
    readonly links: ForwardLink[];

    constructor(props?: Properties<GetSingleBlockByIndexReply>) {
        super(props);

        this.links = this.links || [];
    }
}

export class GetUpdateChain extends Message<GetUpdateChain> {
    /**
     * @see README#Message classes
     */
    static register() {
        registerMessage("GetUpdateChain", GetUpdateChain);
    }

    readonly latestID: Buffer;

    constructor(props?: Properties<GetUpdateChain>) {
        super(props);

        this.latestID = Buffer.from(this.latestID || EMPTY_BUFFER);
    }
}

export class GetUpdateChainReply extends Message<GetUpdateChainReply> {
    /**
     * @see README#Message classes
     */
    static register() {
        registerMessage("GetUpdateChainReply", GetUpdateChainReply, SkipBlock);
    }

    readonly update: SkipBlock[];

    constructor(props: Properties<GetUpdateChainReply>) {
        super(props);

        this.update = this.update || [];
    }
}

GetAllSkipChainIDs.register();
GetAllSkipChainIDsReply.register();
StoreSkipBlock.register();
StoreSkipBlockReply.register();
GetSingleBlock.register();
GetSingleBlockByIndex.register();
GetSingleBlockByIndexReply.register();
GetUpdateChain.register();
GetUpdateChainReply.register();
