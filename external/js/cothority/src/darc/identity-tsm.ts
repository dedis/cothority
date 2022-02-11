import { Message, Properties } from "protobufjs/light";
import { EMPTY_BUFFER, registerMessage } from "../protobuf";
import IdentityWrapper, { IIdentity } from "./identity-wrapper";

/**
 * Identity based on a TSM
 */
export default class IdentityTsm extends Message<IdentityTsm> implements IIdentity {
    /**
     * @see README#Message classes
     */
    static register() {
        registerMessage("IdentityTSM", IdentityTsm);
    }

    readonly publickey: Buffer;

    constructor(props?: Properties<IdentityTsm>) {
        super(props);
        this.publickey = Buffer.from(this.publickey || EMPTY_BUFFER);
    }

    /** @inheritdoc */
    verify(msg: Buffer, signature: Buffer): boolean {
        throw new Error("Not implemented");
    }

    /** @inheritdoc */
    toBytes(): Buffer {
        return Buffer.from(this.toString());
    }

    /** @inheritdoc */
    toString(): string {
        return `tsm:${this.publickey.toString('hex')}`;
    }
}
