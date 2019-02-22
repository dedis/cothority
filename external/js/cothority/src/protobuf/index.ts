import protobuf, { Reader } from "protobufjs/light";
import models from "./models.json";

/**
 * ProtobufJS uses Uint8Array for a browser environment but we want the Buffer
 * to be available. The following will force the library to use buffer
 * (https://www.npmjs.com/package/buffer) which combines the efficiency of
 * Uint8Array but provide most of the Buffer interface. See README.
 */
if (!protobuf.util.isNode) {
    // The module is needed only for a specific environment so
    // we delay the import
    // tslint:disable-next-line
    const buffer = require("buffer");

    // @ts-ignore
    protobuf.Reader.prototype._slice = buffer.Buffer.prototype.slice;
    protobuf.Reader.create = (buf) => new Reader(buffer.Buffer.from(buf));
}

/**
 * Detect a wrong import of the protobufsjs library that could lead
 * to inconsistency at runtime because of different bundles
 */
if (protobuf.build !== "light") {
    throw new Error("expecting to use the light module of protobufs");
}

const root = protobuf.Root.fromJSON(models);

export const EMPTY_BUFFER = Buffer.allocUnsafe(0);

export function registerMessage(name: string, ctor: protobuf.Constructor<{}>): void {
    const m = root.lookupType(name);

    m.ctor = ctor;
}
