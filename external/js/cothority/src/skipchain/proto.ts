import { Message, Properties } from "protobufjs/light";
import { registerMessage } from "../protobuf";
import { ForwardLink, SkipBlock } from "./skipblock";

export class GetAllSkipChainIDs extends Message<GetAllSkipChainIDs> { }

export class GetAllSkipChainIDsReply extends Message<GetAllSkipChainIDsReply> {
    readonly skipChainIDs: Buffer[];
}

export class StoreSkipBlock extends Message<StoreSkipBlock> {
    readonly targetSkipChainID: Buffer;
    readonly newBlock: SkipBlock;
    readonly signature: Buffer;

    constructor(properties: Properties<StoreSkipBlock>) {
        super(properties);
    }
}

export class StoreSkipBlockReply extends Message<StoreSkipBlock> {
    readonly latest: SkipBlock;
    readonly previous: SkipBlock;
}

export class GetSingleBlock extends Message<GetSingleBlock> {
    readonly id: Buffer;
}

export class GetSingleBlockByIndex extends Message<GetSingleBlockByIndex> {
    readonly genesis: Buffer;
    readonly index: number;
}

export class GetSingleBlockByIndexReply extends Message<GetSingleBlockByIndexReply> {
    readonly skipblock: SkipBlock;
    readonly links: ForwardLink[];
}

export class GetUpdateChain extends Message<GetUpdateChain> {
    readonly latestID: Buffer;
}

export class GetUpdateChainReply extends Message<GetUpdateChainReply> {
    readonly update: SkipBlock[];
}

registerMessage("GetAllSkipChainIDs", GetAllSkipChainIDs);
registerMessage("GetAllSkipChainIDsReply", GetAllSkipChainIDsReply);
registerMessage("StoreSkipBlock", StoreSkipBlock);
registerMessage("StoreSkipBlockReply", StoreSkipBlockReply);
registerMessage("GetSingleBlock", GetSingleBlock);
registerMessage("GetSingleBlockByIndex", GetSingleBlockByIndex);
registerMessage("GetSingleBlockByIndexReply", GetSingleBlockByIndexReply);
registerMessage("GetUpdateChain", GetUpdateChain);
registerMessage("GetUpdateChainReply", GetUpdateChainReply);
