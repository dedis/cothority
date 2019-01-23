import {Proof} from "~/lib/cothority/byzcoin/Proof";
import {InstanceID} from "~/lib/cothority/byzcoin/ClientTransaction";

export class Instance{
    data: Buffer;

    constructor(){}

    get id(): InstanceID{
        return new InstanceID(new Buffer(32));
    }
    static fromProof(p: Proof): Instance{
        return new Instance();
    }
}