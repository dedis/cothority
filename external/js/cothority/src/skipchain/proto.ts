import { Message, Properties } from "protobufjs";
import { SkipBlock } from "./skipblock";
import { registerMessage } from "../protobuf";

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

export class GetUpdateChain extends Message<GetUpdateChain> {
    readonly latestID: Buffer;
}

export class GetUpdateChainReply extends Message<GetUpdateChainReply> {
    readonly update: SkipBlock[];
}

registerMessage('StoreSkipBlock', StoreSkipBlock);
registerMessage('StoreSkipBlockReply', StoreSkipBlockReply);
registerMessage('GetSingleBlock', GetSingleBlock);
registerMessage('GetUpdateChain', GetUpdateChain);
registerMessage('GetUpdateChainReply', GetUpdateChainReply);
