import {InstanceID} from "~/lib/cothority/byzcoin/ClientTransaction";
import {Proof} from "~/lib/cothority/byzcoin/Proof";
import {ByzCoinRPC} from "~/lib/cothority/byzcoin/ByzCoinRPC";
import {Coin, CoinInstance} from "~/lib/cothority/byzcoin/contracts/CoinInstance";

export interface Instance {
    // These fields are available for every instance.
    iid: InstanceID;
    darcID: InstanceID;
    contractID: string;
    data: Buffer;
    bc: ByzCoinRPC;

    // Convert the instance
    toObject(): any;
}

export class BasicInstance implements Instance {
    public iid: InstanceID;
    public darcID: InstanceID;
    public data: Buffer;

    constructor(public bc: ByzCoinRPC, public contractID: string, p: Proof | object = null) {
        if (p) {
            if (p.matchContract) {
                if (p.matchContract(contractID)){
                    this.iid = p.requestedIID;
                    this.darcID = p.darcID;
                    this.data = p.value;
                }
            } else {
                this.data = Buffer.from(p.data);
                this.iid = InstanceID.fromObject(p.iid);
                this.darcID = InstanceID.fromObject(p.darcID);
            }
        }
    }

    toObject(): any {
        return {
            iid: this.iid.toObject(),
            darcID: this.darcID.toObject(),
            data: this.data,
            contractID: this.contractID,
        }
    }
}