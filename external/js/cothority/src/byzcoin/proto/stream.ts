import { Message, Properties } from "protobufjs/light";
import { registerMessage, EMPTY_BUFFER } from "../../protobuf";
import { SkipBlock } from "../../skipchain/skipblock";

export class StreamingRequest extends Message<StreamingRequest> {
    /**
     * @see README#Message classes
     */
    static register() {
        registerMessage("StreamingRequest", StreamingRequest);
    }

    readonly id: Buffer
}

export class StreamingResponse extends Message<StreamingResponse> {
    /**
     * @see README#Message classes
     */
    static register() {
        registerMessage("StreamingResponse", StreamingResponse, SkipBlock);
    }

    readonly block: SkipBlock
}

export class PaginateRequest extends Message<PaginateRequest> {
    /**
     * @see README#Message classes
     */
    static register() {
        registerMessage("PaginateRequest", PaginateRequest);
    }

    constructor(props?: Properties<PaginateRequest>) {
        super(props);

        this.startid = Buffer.from(this.startid || EMPTY_BUFFER);
        this.pagesize = this.pagesize || 0
        this.numpages = this.numpages || 0
        this.backward = this.backward || false
        this.streamid = Buffer.from(this.streamid || EMPTY_BUFFER)
    }

    readonly startid: Buffer
    readonly pagesize: number
    readonly numpages: number
    readonly backward: Boolean
    readonly streamid: Buffer
}

export class PaginateResponse extends Message<PaginateResponse> {
    /**
     * @see README#Message classes
     */
    static register() {
        registerMessage("PaginateResponse", PaginateResponse, SkipBlock);
    }

    constructor(props?: Properties<PaginateRequest>) {
        super(props);

        this.blocks = this.blocks || []
        this.pagenumber = this.pagenumber || 0
        this.backward = this.backward || false
        this.errorcode = this.errorcode || 0
        this.errortext = this.errortext || []
    }

    readonly blocks: SkipBlock[]
    readonly pagenumber: number
    readonly streamid: Buffer
    readonly backward: Boolean
    readonly errorcode: number
    readonly errortext: string[]
}

StreamingRequest.register()
StreamingResponse.register()

PaginateRequest.register()
PaginateResponse.register()