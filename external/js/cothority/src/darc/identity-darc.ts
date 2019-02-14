import { Message } from "protobufjs";
import { registerMessage } from "../protobuf";
import Identity from "./identity";
import IdentityWrapper from "./identity-wrapper";

/**
 * Identity based on a DARC identifier
 */
export default class IdentityDarc extends Message<IdentityDarc> implements Identity {
    readonly id: Buffer;

    /** @inheritdoc */
    verify(msg: Buffer, signature: Buffer): boolean {
        return false;
    }

    /** @inheritdoc */
    toWrapper(): IdentityWrapper {
        return new IdentityWrapper({ darc: this });
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

registerMessage("IdentityDarc", IdentityDarc);
