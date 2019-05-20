import { Message, Properties } from "protobufjs/light";
import { EMPTY_BUFFER, registerMessage } from "../protobuf";
import IdentityWrapper, { IIdentity } from "./identity-wrapper";

/**
 * Identity based on a DARC identifier
 */
export default class IdentityDarc extends Message<IdentityDarc> implements IIdentity {
    /**
     * @see README#Message classes
     */
    static register() {
        registerMessage("IdentityDarc", IdentityDarc);
    }

    readonly id: Buffer;

    constructor(props?: Properties<IdentityDarc>) {
        super(props);

        this.id = Buffer.from(this.id || EMPTY_BUFFER);
    }

    /** @inheritdoc */
    verify(msg: Buffer, signature: Buffer): boolean {
        return false;
    }

    /** @inheritdoc */
    toBytes(): Buffer {
        return this.id;
    }

    /** @inheritdoc */
    toString(): string {
        return `darc:${this.id.toString("hex")}`;
    }
}
