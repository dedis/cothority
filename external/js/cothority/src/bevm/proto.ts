import { Message, Properties } from "protobufjs/light";
import { registerMessage } from "../protobuf";

export class ViewCallRequest extends Message<ViewCallRequest> {
    static register() {
        registerMessage("bevm.ViewCallRequest", ViewCallRequest);
    }

    readonly byzcoinId: Buffer;
    readonly bevmInstanceId: Buffer;
    readonly accountAddress: Buffer;
    readonly contractAddress: Buffer;
    readonly callData: Buffer;
    readonly minBlockIndex: number;

    constructor(props?: Properties<ViewCallRequest>) {
        super(props);

        /* Protobuf aliases */
        Object.defineProperty(this, "byzcoinid", {
            get(): Buffer {
                return this.byzcoinId;
            },
            set(value: Buffer) {
                this.blockId = value;
            },
        });

        Object.defineProperty(this, "bevminstanceid", {
            get(): Buffer {
                return this.bevmInstanceId;
            },
            set(value: Buffer) {
                this.bevmInstanceId = value;
            },
        });

        Object.defineProperty(this, "accountaddress", {
            get(): Buffer {
                return this.accountAddress;
            },
            set(value: Buffer) {
                this.accountAddress = value;
            },
        });

        Object.defineProperty(this, "contractaddress", {
            get(): Buffer {
                return this.contractAddress;
            },
            set(value: Buffer) {
                this.contractAddress = value;
            },
        });

        Object.defineProperty(this, "calldata", {
            get(): Buffer {
                return this.callData;
            },
            set(value: Buffer) {
                this.callData = value;
            },
        });

        Object.defineProperty(this, "minblockindex", {
            get(): number {
                return this.minBlockIndex;
            },
            set(value: number) {
                this.minBlockIndex = value;
            },
        });
    }
}

export class ViewCallResponse extends Message<ViewCallResponse> {
    static register() {
        registerMessage("bevm.ViewCallResponse", ViewCallResponse);
    }

    readonly result: Buffer;
}

ViewCallRequest.register();
ViewCallResponse.register();
