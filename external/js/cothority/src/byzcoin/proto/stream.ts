import { Message, Properties } from "protobufjs/light";

import { EMPTY_BUFFER, registerMessage } from "../../protobuf";
import { SkipBlock } from "../../skipchain/skipblock";

export class StreamingRequest extends Message<StreamingRequest> {
    /**
     * @see README#Message classes
     */
    static register() {
        registerMessage("StreamingRequest", StreamingRequest);
    }

    readonly id: Buffer;
}

export class StreamingResponse extends Message<StreamingResponse> {
    /**
     * @see README#Message classes
     */
    static register() {
        registerMessage("StreamingResponse", StreamingResponse, SkipBlock);
    }

    readonly block: SkipBlock;
}

export class PaginateRequest extends Message<PaginateRequest> {
    /**
     * @see README#Message classes
     */
    static register() {
        registerMessage("PaginateRequest", PaginateRequest);
    }

    readonly startid: Buffer;
    readonly pagesize: number;
    readonly numpages: number;
    readonly backward: boolean;

    constructor(props?: Properties<PaginateRequest>) {
        super(props);

        this.startid = Buffer.from(this.startid || EMPTY_BUFFER);
        this.pagesize = this.pagesize || 0;
        this.numpages = this.numpages || 0;
        this.backward = this.backward || false;
    }
}

export class PaginateResponse extends Message<PaginateResponse> {
    /**
     * @see README#Message classes
     */
    static register() {
        registerMessage("PaginateResponse", PaginateResponse, SkipBlock);
    }

    readonly blocks: SkipBlock[];
    readonly pagenumber: number;
    readonly backward: boolean;
    readonly errorcode: number;
    readonly errortext: string[];

    constructor(props?: Properties<PaginateRequest>) {
        super(props);

        this.blocks = this.blocks || [];
        this.pagenumber = this.pagenumber || 0;
        this.backward = this.backward || false;
        this.errorcode = this.errorcode || 0;
        this.errortext = this.errortext || [];
    }
}

StreamingRequest.register();
StreamingResponse.register();

PaginateRequest.register();
PaginateResponse.register();
