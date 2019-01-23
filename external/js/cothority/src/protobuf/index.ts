import protobuf = require("protobufjs");
import models from './models.json';

const root = protobuf.Root.fromJSON(models);

export function registerMessage(name: string, ctor: protobuf.Constructor<{}>): void {
    const m = root.lookupType(name);

    m.ctor = ctor;
}
